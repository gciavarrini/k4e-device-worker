package configuration

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"sync/atomic"
	"time"

	"git.sr.ht/~spc/go-log"

	"github.com/hashicorp/go-multierror"
	"github.com/jakub-dzon/k4e-operator/models"
)

var (
	defaultHeartbeatConfiguration = models.HeartbeatConfiguration{
		HardwareProfile: &models.HardwareProfileConfiguration{},
		PeriodSeconds:   60,
	}
	defaultDeviceConfiguration = models.DeviceConfiguration{
		Heartbeat: &defaultHeartbeatConfiguration,
	}
	defaultDeviceConfigurationMessage = models.DeviceConfigurationMessage{
		Configuration: &defaultDeviceConfiguration,
	}
)

//go:generate mockgen -package=configuration -destination=configuration_mock.go . Observer
type Observer interface {
	Update(configuration models.DeviceConfigurationMessage) error
}

type Manager struct {
	deviceConfiguration *models.DeviceConfigurationMessage

	observers        []Observer
	deviceConfigFile string
	initialConfig    atomic.Value
}

func NewConfigurationManager(dataDir string) *Manager {
	deviceConfigFile := path.Join(dataDir, "device-config.json")
	log.Infof("Device config file: %s", deviceConfigFile)
	file, err := ioutil.ReadFile(deviceConfigFile)
	var deviceConfiguration models.DeviceConfigurationMessage
	initialConfig := atomic.Value{}
	initialConfig.Store(false)
	if err != nil {
		log.Error(err)
		deviceConfiguration = defaultDeviceConfigurationMessage
		initialConfig.Store(true)
	} else {
		err = json.Unmarshal(file, &deviceConfiguration)
		if err != nil {
			log.Error(err)
			deviceConfiguration = defaultDeviceConfigurationMessage
		}
	}
	mgr := Manager{
		observers:           make([]Observer, 0),
		deviceConfigFile:    deviceConfigFile,
		deviceConfiguration: &deviceConfiguration,
		initialConfig:       initialConfig,
	}
	return &mgr
}

func (m *Manager) RegisterObserver(observer Observer) {
	m.observers = append(m.observers, observer)
}

func (m *Manager) GetDeviceConfiguration() models.DeviceConfiguration {
	return *m.deviceConfiguration.Configuration
}

func (m *Manager) GetWorkloads() models.WorkloadList {
	return m.deviceConfiguration.Workloads
}

func (m *Manager) Update(message models.DeviceConfigurationMessage) error {

	configurationEqual := reflect.DeepEqual(message.Configuration, m.deviceConfiguration.Configuration)
	workloadsEqual := reflect.DeepEqual(message.Workloads, m.deviceConfiguration.Workloads)
	log.Tracef("Workloads equal: [%v]; configurationEqual: [%v]", workloadsEqual, configurationEqual)

	shouldUpdate := !(configurationEqual && workloadsEqual)

	if m.IsInitialConfig() {
		log.Trace("Force update because it's init phase")
		shouldUpdate = true
	}

	if !shouldUpdate {
		log.Trace("Configuration didn't change")
		return nil
	}

	log.Tracef("Updating configuration: %v", message)
	var errors error
	for _, observer := range m.observers {
		err := observer.Update(message)
		if err != nil {
			errors = multierror.Append(errors, fmt.Errorf("running config observer failed: %s", err))
		}
	}

	file, err := json.MarshalIndent(message, "", " ")
	if err != nil {
		errors = multierror.Append(errors, fmt.Errorf("cannot unmarshal message JSON: %s", err))
		return errors
	}

	log.Tracef("Writing config to %s: %v", m.deviceConfigFile, file)
	err = ioutil.WriteFile(m.deviceConfigFile, file, 0640)
	if err != nil {
		errors = multierror.Append(fmt.Errorf("cannot write device config file '%s': %s", m.deviceConfigFile, err))
		return errors
	}

	m.deviceConfiguration = &message
	m.initialConfig.Store(false)

	return errors
}

func (m *Manager) GetDataTransferInterval() time.Duration {
	return time.Second * 15
}

func (m *Manager) GetConfigurationVersion() string {
	version := m.deviceConfiguration.Version
	log.Tracef("Configuration version: %v", version)
	return version
}

func (m *Manager) IsInitialConfig() bool {
	return m.initialConfig.Load().(bool)
}

func (m *Manager) Deregister() error {
	log.Infof("Removing device config file: %s", m.deviceConfigFile)
	err := os.Remove(m.deviceConfigFile)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}
