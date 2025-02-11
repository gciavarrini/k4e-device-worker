package heartbeat

import (
	"context"
	"encoding/json"
	"time"

	"git.sr.ht/~spc/go-log"
	"github.com/go-openapi/strfmt"
	"github.com/google/uuid"
	cfg "github.com/jakub-dzon/k4e-device-worker/internal/configuration"
	"github.com/jakub-dzon/k4e-device-worker/internal/datatransfer"
	hw "github.com/jakub-dzon/k4e-device-worker/internal/hardware"
	workld "github.com/jakub-dzon/k4e-device-worker/internal/workload"
	"github.com/jakub-dzon/k4e-operator/models"
	pb "github.com/redhatinsights/yggdrasil/protocol"
)

type HeartbeatData struct {
	configManager   *cfg.Manager
	workloadManager *workld.WorkloadManager
	dataMonitor     *datatransfer.Monitor
	hardware        *hw.Hardware
}

func NewHeartbeatData(configManager *cfg.Manager,
	workloadManager *workld.WorkloadManager, hardware *hw.Hardware, dataMonitor *datatransfer.Monitor) *HeartbeatData {

	return &HeartbeatData{
		configManager:   configManager,
		workloadManager: workloadManager,
		hardware:        hardware,
		dataMonitor:     dataMonitor,
	}
}

func (s *HeartbeatData) RetrieveInfo() models.Heartbeat {
	var workloadStatuses []*models.WorkloadStatus
	workloads, err := s.workloadManager.ListWorkloads()
	for _, info := range workloads {
		workloadStatus := models.WorkloadStatus{
			Name:   info.Name,
			Status: info.Status,
		}
		if lastSyncTime := s.dataMonitor.GetLastSuccessfulSyncTime(info.Name); lastSyncTime != nil {
			workloadStatus.LastDataUpload = strfmt.DateTime(*lastSyncTime)
		}
		workloadStatuses = append(workloadStatuses, &workloadStatus)
	}
	if err != nil {
		log.Errorf("Cannot get workload information: %v", err)
	}

	config := s.configManager.GetDeviceConfiguration()
	var hardwareInfo *models.HardwareInfo
	if config.Heartbeat.HardwareProfile.Include {
		hardwareInfo, err = s.hardware.GetHardwareInformation()
		if err != nil {
			log.Errorf("Can't get hardware information: %v", err)
		}
	}

	heartbeatInfo := models.Heartbeat{
		Status:    models.HeartbeatStatusUp,
		Time:      strfmt.DateTime(time.Now()),
		Version:   s.configManager.GetConfigurationVersion(),
		Workloads: workloadStatuses,
		Hardware:  hardwareInfo,
	}
	return heartbeatInfo
}

type Heartbeat struct {
	ticker           *time.Ticker
	dispatcherClient pb.DispatcherClient
	data             *HeartbeatData
}

func NewHeartbeatService(dispatcherClient pb.DispatcherClient, configManager *cfg.Manager,
	workloadManager *workld.WorkloadManager, hardware *hw.Hardware, dataMonitor *datatransfer.Monitor) *Heartbeat {
	return &Heartbeat{
		ticker:           nil,
		dispatcherClient: dispatcherClient,
		data: &HeartbeatData{
			configManager:   configManager,
			workloadManager: workloadManager,
			hardware:        hardware,
			dataMonitor:     dataMonitor},
	}
}

func (s *Heartbeat) Start() {
	s.initTicker(s.getInterval(s.data.configManager.GetDeviceConfiguration()))
}

func (s *Heartbeat) HasStarted() bool {
	return s.ticker != nil
}

func (s *Heartbeat) Update(config models.DeviceConfigurationMessage) error {
	periodSeconds := s.getInterval(*config.Configuration)
	log.Infof("Reconfiguring ticker with interval: %v", periodSeconds)
	if s.ticker != nil {
		s.ticker.Stop()
	}
	s.initTicker(periodSeconds)
	return nil
}

func (s *Heartbeat) getInterval(config models.DeviceConfiguration) int64 {
	var interval int64 = 60

	if config.Heartbeat != nil {
		interval = config.Heartbeat.PeriodSeconds
	}
	if interval <= 0 {
		interval = 60
	}
	return interval
}

func (s *Heartbeat) pushInformation() error {

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Create a data message to send back to the dispatcher.
	heartbeatInfo := s.data.RetrieveInfo()

	content, err := json.Marshal(heartbeatInfo)
	if err != nil {
		return err
	}

	data := &pb.Data{
		MessageId: uuid.New().String(),
		Content:   content,
		Directive: "heartbeat",
	}

	_, err = s.dispatcherClient.Send(ctx, data)
	return err
}

func (s *Heartbeat) initTicker(periodSeconds int64) {
	ticker := time.NewTicker(time.Second * time.Duration(periodSeconds))
	s.ticker = ticker
	go func() {
		for range ticker.C {
			err := s.pushInformation()
			if err != nil {
				log.Errorf("Heartbeat interval cannot send the data, err: %s", err)
			}
		}
	}()
	log.Info("The heartbeat was started")
}

func (s *Heartbeat) Deregister() error {
	log.Info("Stopping heartbeat ticker")
	if s.ticker != nil {
		s.ticker.Stop()
	}
	return nil
}
