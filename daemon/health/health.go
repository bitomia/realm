package health

import (
	"log/slog"
	"os"
	"sync"
	"time"

	"go.etcd.io/etcd/client/v3"

	"github.com/bitomia/realm/daemon/db"
	"github.com/bitomia/realm/config"
)

type HealthPublisher struct {
	db              *db.DaemonDB
	nodeID          string
	leaseID         clientv3.LeaseID
	publishChan     chan bool
	stopChan        chan bool
	wg              sync.WaitGroup
	publishInterval time.Duration
}

const (
	STATUS_HEALTHY   = "healthy"
	STATUS_UNHEALTHY = "unhealthy"
	STATUS_STARTING  = "starting"
	STATUS_STOPPING  = "stopping"

	DEFAULT_TTL              = 30 // seconds
	DEFAULT_PUBLISH_INTERVAL = 10 * time.Second
)

var (
	instance *HealthPublisher
	once     sync.Once
)

func GetHealthPublisher() *HealthPublisher {
	once.Do(func() {
		hostname, err := os.Hostname()
		if err != nil {
			slog.Error("Error getting hostname", "error", err)
			return
		}
		instance = &HealthPublisher{
			db:              db.GetDB(),
			nodeID:          hostname,
			publishChan:     make(chan bool, 1),
			stopChan:        make(chan bool, 1),
			publishInterval: DEFAULT_PUBLISH_INTERVAL,
		}
	})
	return instance
}

func (hp *HealthPublisher) Start() error {
	slog.Info("Starting health publisher for node", "nodeID", hp.nodeID)

	leaseID, err := hp.db.CreateLease(DEFAULT_TTL)
	if err != nil {
		slog.Error("Failed to create lease", "error", err.Error())
		return err
	}
	hp.leaseID = leaseID

	keepAliveChan, err := hp.db.KeepAlive(leaseID)
	if err != nil {
		slog.Error("Failed to start lease keep-alive", "error", err.Error())
		return err
	}

	hp.wg.Add(2)

	go hp.keepAliveHandler(keepAliveChan)
	go hp.publishHealthLoop()

	err = hp.PublishStatus(STATUS_STARTING, nil)
	if err != nil {
		slog.Error("Failed to publish initial health status", "error", err.Error())
		return err
	}

	slog.Info("Health publisher started successfully")
	return nil
}

func (hp *HealthPublisher) Stop() {
	slog.Info("Stopping health publisher for node", "nodeID", hp.nodeID)

	hp.PublishStatus(STATUS_STOPPING, nil)

	close(hp.stopChan)
	hp.wg.Wait()

	hp.db.DeleteHealthStatus(hp.nodeID)
	slog.Info("Health publisher stopped")
}

func (hp *HealthPublisher) PublishStatus(status string, metadata map[string]interface{}) error {
	db := db.GetDB()

	err := db.PublishHealthStatus(hp.nodeID, hp.leaseID, status, metadata)
	if err != nil {
		slog.Error("Failed to publish health status", "error", err.Error())
		return err
	}

	return nil
}

func (hp *HealthPublisher) PublishHealthy() error {
	return hp.PublishStatus(STATUS_HEALTHY, hp.collectMetadata())
}

func (hp *HealthPublisher) PublishUnhealthy() error {
	return hp.PublishStatus(STATUS_UNHEALTHY, hp.collectMetadata())
}

func (hp *HealthPublisher) TriggerPublish() {
	select {
	case hp.publishChan <- true:
	default:
	}
}

func (hp *HealthPublisher) keepAliveHandler(keepAliveChan <-chan *clientv3.LeaseKeepAliveResponse) {
	defer hp.wg.Done()

	for {
		select {
		case <-hp.stopChan:
			return
		case resp := <-keepAliveChan:
			if resp == nil {
				slog.Warn("Health publisher lease keep-alive channel closed")
				return
			}
		}
	}
}

func (hp *HealthPublisher) publishHealthLoop() {
	defer hp.wg.Done()

	ticker := time.NewTicker(hp.publishInterval)
	defer ticker.Stop()

	for {
		select {
		case <-hp.stopChan:
			return
		case <-ticker.C:
			err := hp.PublishHealthy()
			if err != nil {
				slog.Error("Failed to publish periodic health status", "error", err.Error())
			}
		case <-hp.publishChan:
			err := hp.PublishHealthy()
			if err != nil {
				slog.Error("Failed to publish triggered health status", "error", err.Error())
			}
		}
	}
}

func (hp *HealthPublisher) collectMetadata() map[string]interface{} {
	return map[string]interface{}{
		"version": config.GetVersion(),
	}
}
