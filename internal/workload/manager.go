package workload

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/jakub-dzon/k4e-device-worker/internal/volumes"

	"git.sr.ht/~spc/go-log"
	api2 "github.com/jakub-dzon/k4e-device-worker/internal/workload/api"
	"github.com/jakub-dzon/k4e-operator/models"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

const (
	defaultWorkloadsMonitoringInterval = 15
)

type WorkloadManager struct {
	manifestsDir   string
	volumesDir     string
	workloads      WorkloadWrapper
	managementLock sync.Locker
	ticker         *time.Ticker
	deregistered   bool
	eventsQueue    []*models.EventInfo
	deviceId       string
	authDir        string
}

type podAndPath struct {
	pod          v1.Pod
	manifestPath string
}

func NewWorkloadManager(dataDir string, deviceId string) (*WorkloadManager, error) {
	wrapper, err := newWorkloadInstance(dataDir)
	if err != nil {
		return nil, err
	}

	return NewWorkloadManagerWithParams(dataDir, wrapper, deviceId)
}

func NewWorkloadManagerWithMonitorInterval(dataDir string, monitorInterval int64, deviceId string) (*WorkloadManager, error) {
	wrapper, err := newWorkloadInstance(dataDir)
	if err != nil {
		return nil, err
	}

	return NewWorkloadManagerWithParamsAndInterval(dataDir, wrapper, monitorInterval, deviceId)
}

func NewWorkloadManagerWithParams(dataDir string, ww WorkloadWrapper, deviceId string) (*WorkloadManager, error) {
	return NewWorkloadManagerWithParamsAndInterval(dataDir, ww, defaultWorkloadsMonitoringInterval, deviceId)
}

func NewWorkloadManagerWithParamsAndInterval(dataDir string, ww WorkloadWrapper, monitorInterval int64, deviceId string) (*WorkloadManager, error) {
	manifestsDir := path.Join(dataDir, "manifests")
	if err := os.MkdirAll(manifestsDir, 0755); err != nil {
		return nil, fmt.Errorf("cannot create directory: %w", err)
	}
	volumesDir := path.Join(dataDir, "volumes")
	if err := os.MkdirAll(volumesDir, 0755); err != nil {
		return nil, fmt.Errorf("cannot create directory: %w", err)
	}
	authDir := path.Join(dataDir, "auth")
	if err := os.MkdirAll(authDir, 0755); err != nil {
		return nil, fmt.Errorf("cannot create directory: %w", err)
	}
	manager := WorkloadManager{
		manifestsDir:   manifestsDir,
		volumesDir:     volumesDir,
		authDir:        authDir,
		workloads:      ww,
		managementLock: &sync.Mutex{},
		deregistered:   false,
		deviceId:       deviceId,
	}
	if err := manager.workloads.Init(); err != nil {
		return nil, err
	}

	manager.initTicker(monitorInterval)
	return &manager, nil
}

// PopEvents return copy of all the events stored in eventQueue
func (w *WorkloadManager) PopEvents() []*models.EventInfo {
	w.managementLock.Lock()
	defer w.managementLock.Unlock()

	// Copy the events:
	events := []*models.EventInfo{}
	for _, event := range w.eventsQueue {
		e := *event
		events = append(events, &e)
	}
	// Empty the events:
	w.eventsQueue = []*models.EventInfo{}
	return events
}

func (w *WorkloadManager) ListWorkloads() ([]api2.WorkloadInfo, error) {
	return w.workloads.List()
}

func (w *WorkloadManager) GetExportedHostPath(workloadName string) string {
	return volumes.HostPathVolumePath(w.volumesDir, workloadName)
}

func (w *WorkloadManager) Update(configuration models.DeviceConfigurationMessage) error {
	w.managementLock.Lock()
	defer w.managementLock.Unlock()
	var errors error
	if w.deregistered {
		log.Info("Deregistration was finished, no need to update anymore")
		return errors
	}

	configuredWorkloadNameSet := make(map[string]struct{})
	for _, workload := range configuration.Workloads {
		log.Tracef("Deploying workload: %s", workload.Name)
		configuredWorkloadNameSet[workload.Name] = struct{}{}

		pod, err := w.toPod(workload)
		if err != nil {
			errors = multierror.Append(errors, fmt.Errorf(
				"cannot convert workload '%s' to pod: %s", workload.Name, err))
			continue
		}
		manifestPath := w.getManifestPath(pod.Name)
		podYaml, err := w.toPodYaml(pod)
		if err != nil {
			errors = multierror.Append(errors, fmt.Errorf("cannot create pod's Yaml: %s", err))
			continue
		}
		authFilePath := w.getAuthFilePath(pod.Name)
		var authFile string
		if workload.ImageRegistries != nil {
			authFile = workload.ImageRegistries.AuthFile
		}
		if !w.podConfigurationModified(manifestPath, podYaml, authFilePath, authFile) {
			log.Tracef("Pod '%s' definition is unchanged (%s)", workload.Name, manifestPath)
			continue
		}
		err = w.storeFile(manifestPath, podYaml)
		if err != nil {
			errors = multierror.Append(errors, fmt.Errorf(
				"cannot store manifest for workload '%s': %s", workload.Name, err))
			continue
		}

		authFilePath, err = w.manageAuthFile(authFilePath, authFile)
		if err != nil {
			errors = multierror.Append(errors, fmt.Errorf(
				"cannot store auth configuration for workload '%s': %s", workload.Name, err))
			continue
		}

		err = w.workloads.Remove(workload.Name)
		if err != nil {
			log.Errorf("Error removing workload: %v", err)
			errors = multierror.Append(errors, fmt.Errorf("error removing workload %s: %s", workload.Name, err))
			continue
		}
		err = w.workloads.Run(pod, manifestPath, authFilePath)
		if err != nil {
			log.Errorf("Cannot run workload: %v", err)
			errors = multierror.Append(errors, fmt.Errorf(
				"cannot run workload '%s': %s", workload.Name, err))
			continue
		}
	}

	deployedWorkloadByName, err := w.indexWorkloads()
	if err != nil {
		log.Errorf("Cannot get deployed workloads: %v", err)
		errors = multierror.Append(errors, fmt.Errorf("cannot get deployed workloads: %s", err))
		return errors
	}
	// Remove any workloads that don't correspond to the configured ones
	for name := range deployedWorkloadByName {
		if _, ok := configuredWorkloadNameSet[name]; !ok {
			log.Infof("Workload not found: %s. Removing", name)
			manifestPath := w.getManifestPath(name)
			if err := deleteFile(manifestPath); err != nil {
				errors = multierror.Append(errors, fmt.Errorf("cannot remove existing manifest workload: %s", err))
			}
			authPath := w.getAuthFilePath(name)
			if err := deleteFile(authPath); err != nil {
				errors = multierror.Append(errors, fmt.Errorf("cannot remove existing workload auth file: %s", err))
			}
			if err := w.workloads.Remove(name); err != nil {
				errors = multierror.Append(errors, fmt.Errorf("cannot remove stale workload name='%s': %s", name, err))
			}
			log.Infof("Workload %s removed", name)
		}
	}
	// Reset the interval of the current monitoring routine
	if configuration.WorkloadsMonitoringInterval > 0 {
		w.ticker.Reset(time.Duration(configuration.WorkloadsMonitoringInterval))
	}
	return errors
}

// manageAuthFile is responsible for bringing auth configuration file under authFilePath to expected state;
// if the content of the file - authFile is supposed to be blank, the file is removed, otherwise authFile is written
// to the authFilePath file.
func (w *WorkloadManager) manageAuthFile(authFilePath, authFile string) (string, error) {
	if authFile == "" {
		if err := deleteFile(authFilePath); err != nil {
			return "", fmt.Errorf("cannot remove auth file %s: %s", authFilePath, err)
		}
		return "", nil
	}
	if err := w.storeFile(authFilePath, []byte(authFile)); err != nil {
		return "", fmt.Errorf("cannot store auth file %s: %s", authFilePath, err)
	}
	return authFilePath, nil
}

func (w *WorkloadManager) initTicker(periodSeconds int64) {
	ticker := time.NewTicker(time.Second * time.Duration(periodSeconds))
	w.ticker = ticker
	go func() {
		for range ticker.C {
			err := w.ensureWorkloadsFromManifestsAreRunning()
			if err != nil {
				log.Error(err)
			}
		}
	}()
}

func (w *WorkloadManager) storeFile(filePath string, content []byte) error {
	return ioutil.WriteFile(filePath, content, 0640)
}

func (w *WorkloadManager) getAuthFilePath(workloadName string) string {
	return path.Join(w.authDir, w.getAuthFileName(workloadName))
}

func (w *WorkloadManager) getAuthFileName(workloadName string) string {
	return strings.ReplaceAll(workloadName, " ", "-") + "-auth.yaml"
}

func (w *WorkloadManager) getManifestPath(workloadName string) string {
	fileName := strings.ReplaceAll(workloadName, " ", "-") + ".yaml"
	return path.Join(w.manifestsDir, fileName)
}

func (w *WorkloadManager) toPodYaml(pod *v1.Pod) ([]byte, error) {
	podYaml, err := yaml.Marshal(pod)
	if err != nil {
		return nil, err
	}
	return podYaml, nil
}

func (w *WorkloadManager) ensureWorkloadsFromManifestsAreRunning() error {
	w.managementLock.Lock()
	defer w.managementLock.Unlock()
	nameToWorkload, err := w.indexWorkloads()
	if err != nil {
		return err
	}

	manifestInfo, err := ioutil.ReadDir(w.manifestsDir)
	if err != nil {
		return err
	}
	manifestNameToPodAndPath := make(map[string]podAndPath)
	expectedAuthFiles := make(map[string]struct{})
	for _, fi := range manifestInfo {
		filePath := path.Join(w.manifestsDir, fi.Name())
		manifest, err := ioutil.ReadFile(filePath)
		if err != nil {
			log.Error(err)
			continue
		}
		pod := v1.Pod{}
		err = yaml.Unmarshal(manifest, &pod)
		if err != nil {
			log.Error(err)
			continue
		}
		manifestNameToPodAndPath[pod.Name] = podAndPath{pod, filePath}
		expectedAuthFiles[w.getAuthFileName(pod.Name)] = struct{}{}
	}

	// Remove any workloads that don't correspond to stored manifests
	for name := range nameToWorkload {
		if _, ok := manifestNameToPodAndPath[name]; !ok {
			log.Infof("Workload not found: %s. Removing", name)
			if err := w.workloads.Remove(name); err != nil {
				log.Error(err)
			}
		}
	}

	// Remove any auth configuration files that are not used anymore
	authManifestInfo, err := ioutil.ReadDir(w.authDir)
	for _, fi := range authManifestInfo {
		if _, present := expectedAuthFiles[fi.Name()]; !present {
			if err := deleteFile(path.Join(w.authDir, fi.Name())); err != nil {
				log.Error(err)
			}
		}
	}

	for name, podWithPath := range manifestNameToPodAndPath {
		if workload, ok := nameToWorkload[name]; ok {
			if workload.Status != "Running" {
				// Workload is not running - start
				err = w.workloads.Start(&podWithPath.pod)
				if err != nil {
					w.eventsQueue = append(w.eventsQueue, &models.EventInfo{
						Message: err.Error(),
						Reason:  "Failed",
						Type:    models.EventInfoTypeWarn,
					})
					log.Errorf("failed to start workload %s: %v", name, err)
				}
			}
			continue
		}
		// Workload is not present - run
		err = w.workloads.Run(&podWithPath.pod, podWithPath.manifestPath, w.getAuthFilePathIfExists(name))
		if err != nil {
			log.Errorf("failed to run workload %s (manifest: %s): %v", name, podWithPath.manifestPath, err)
			continue
		}
	}
	if err = w.workloads.PersistConfiguration(); err != nil {
		log.Errorf("failed to persist workload configuration: %v", err)
	}
	return nil
}

func (w *WorkloadManager) indexWorkloads() (map[string]api2.WorkloadInfo, error) {
	workloads, err := w.workloads.List()
	if err != nil {
		return nil, err
	}
	nameToWorkload := make(map[string]api2.WorkloadInfo)
	for _, workload := range workloads {
		nameToWorkload[workload.Name] = workload
	}
	return nameToWorkload, nil
}

func (w *WorkloadManager) RegisterObserver(observer Observer) {
	w.workloads.RegisterObserver(observer)
}

func (w *WorkloadManager) Deregister() error {
	w.managementLock.Lock()
	defer w.managementLock.Unlock()

	var errors error
	err := w.removeAllWorkloads()
	if err != nil {
		errors = multierror.Append(errors, fmt.Errorf("failed to remove workloads: %v", err))
		log.Errorf("failed to remove workloads: %v", err)
	}

	err = w.deleteManifestsDir()
	if err != nil {
		errors = multierror.Append(errors, fmt.Errorf("failed to delete manifests directory: %v", err))
		log.Errorf("failed to delete manifests directory: %v", err)
	}

	err = w.deleteAuthDir()
	if err != nil {
		errors = multierror.Append(errors, fmt.Errorf("failed to delete auth directory: %v", err))
		log.Errorf("failed to delete auth directory: %v", err)
	}

	err = w.deleteTable()
	if err != nil {
		errors = multierror.Append(errors, fmt.Errorf("failed to delete table: %v", err))
		log.Errorf("failed to delete table: %v", err)
	}

	err = w.deleteVolumeDir()
	if err != nil {
		errors = multierror.Append(errors, fmt.Errorf("failed to delete volumes directory: %v", err))
		log.Errorf("failed to delete volumes directory: %v", err)
	}

	err = w.removeTicker()
	if err != nil {
		errors = multierror.Append(errors, fmt.Errorf("failed to remove ticker: %v", err))
		log.Errorf("failed to remove ticker: %v", err)
	}

	err = w.removeMappingFile()
	if err != nil {
		errors = multierror.Append(errors, fmt.Errorf("failed to remove mapping file: %v", err))
		log.Errorf("failed to remove mapping file: %v", err)
	}

	w.deregistered = true
	return errors
}

func (w *WorkloadManager) removeTicker() error {
	log.Info("Stopping ticker that ensure workloads from manifests are running")
	if w.ticker != nil {
		w.ticker.Stop()
	}
	return nil
}

func (w *WorkloadManager) removeAllWorkloads() error {
	log.Info("Removing all workload")
	workloads, err := w.workloads.List()
	if err != nil {
		return err
	}
	for _, workload := range workloads {
		log.Infof("Removing workload %s", workload.Name)
		err := w.workloads.Remove(workload.Name)
		if err != nil {
			log.Errorf("Error removing workload %[1]s: %v", workload.Name, err)
			return err
		}
	}
	return nil
}

func (w *WorkloadManager) deleteManifestsDir() error {
	log.Info("Deleting manifests directory")
	return deleteDir(w.manifestsDir)
}

func (w *WorkloadManager) deleteVolumeDir() error {
	log.Info("Deleting volumes directory")
	return deleteDir(w.volumesDir)
}

func (w *WorkloadManager) deleteAuthDir() error {
	log.Info("Deleting auth directory")
	return deleteDir(w.authDir)
}

func deleteDir(path string) error {
	err := os.RemoveAll(path)
	if err != nil {
		log.Error(err)
		return err
	}

	return nil
}

func deleteFile(file string) error {
	if err := os.Remove(file); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func (w *WorkloadManager) deleteTable() error {
	log.Info("Deleting nftable")
	err := w.workloads.RemoveTable()
	if err != nil {
		log.Error(err)
		return err
	}

	return nil
}

func (w *WorkloadManager) removeMappingFile() error {
	log.Info("Deleting mapping file")
	err := w.workloads.RemoveMappingFile()
	if err != nil {
		log.Error(err)
		return err
	}

	return nil
}

func (w *WorkloadManager) toPod(workload *models.Workload) (*v1.Pod, error) {
	podSpec := v1.PodSpec{}
	err := yaml.Unmarshal([]byte(workload.Specification), &podSpec)
	if err != nil {
		return nil, err
	}
	pod := v1.Pod{
		Spec: podSpec,
	}
	pod.Kind = "Pod"
	pod.Name = workload.Name
	exportVolume := volumes.HostPathVolume(w.volumesDir, workload.Name)
	pod.Spec.Volumes = append(pod.Spec.Volumes, exportVolume)
	var containers []v1.Container
	for _, container := range pod.Spec.Containers {
		mount := v1.VolumeMount{
			Name:      exportVolume.Name,
			MountPath: "/export",
		}
		container.VolumeMounts = append(container.VolumeMounts, mount)
		container.Env = append(container.Env, v1.EnvVar{Name: "DEVICE_ID", Value: w.deviceId})
		containers = append(containers, container)
	}
	pod.Spec.Containers = containers
	return &pod, nil
}

func (w *WorkloadManager) podConfigurationModified(manifestPath string, podYaml []byte, authPath string, auth string) bool {
	return w.podModified(manifestPath, podYaml) || w.podAuthModified(authPath, auth)
}

func (w *WorkloadManager) podModified(manifestPath string, podYaml []byte) bool {
	file, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		return true
	}
	return bytes.Compare(file, podYaml) != 0
}

func (w *WorkloadManager) getAuthFilePathIfExists(workloadName string) string {
	authFilePath := w.getAuthFilePath(workloadName)
	if _, err := os.Stat(authFilePath); err != nil {
		return ""
	}
	return authFilePath
}

func (w *WorkloadManager) podAuthModified(authPath string, auth string) bool {
	if _, err := os.Stat(authPath); err != nil {
		if auth == "" {
			return false
		}
		return true
	}
	file, err := ioutil.ReadFile(authPath)
	if err != nil {
		return true
	}
	return bytes.Compare(file, []byte(auth)) != 0
}
