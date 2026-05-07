package health

import (
	"log/slog"
	"os"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/bitomia/realm/agent/db"
	"github.com/bitomia/realm/common/config"
)

type HealthPublisher struct {
	db              *db.AgentDB
	hostname        string
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
			hostname:        hostname,
			publishChan:     make(chan bool, 1),
			stopChan:        make(chan bool, 1),
			publishInterval: DEFAULT_PUBLISH_INTERVAL,
		}
	})
	return instance
}

func (hp *HealthPublisher) createLease() error {
	leaseID, err := hp.db.CreateLease(DEFAULT_TTL)
	if err != nil {
		slog.Error("Failed to create lease", "error", err.Error())
		return err
	}
	hp.leaseID = leaseID
	return nil
}

func (hp *HealthPublisher) Start() error {
	slog.Info("Starting health publisher for node", "hostname", hp.hostname)

	if err := hp.createLease(); err != nil {
		slog.Error("Failed to create lease", "error", err.Error())
		return err
	}

	keepAliveChan, err := hp.db.KeepAlive(hp.leaseID)
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
	slog.Info("Stopping health publisher for node", "hostname", hp.hostname)

	if err := hp.PublishStatus(STATUS_STOPPING, nil); err != nil {
		slog.Warn("Failed to publish stopping status", "hostname", hp.hostname, "error", err)
	}

	close(hp.stopChan)
	hp.wg.Wait()

	if err := hp.db.DeleteHealthStatus(hp.hostname); err != nil {
		slog.Warn("Failed to delete health status", "hostname", hp.hostname, "error", err)
	}
	slog.Info("Health publisher stopped")
}

func (hp *HealthPublisher) PublishStatus(status string, metadata map[string]any) error {
	db := db.GetDB()

	err := db.PublishHealthStatus(hp.hostname, hp.leaseID, status, metadata)
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
				slog.Warn("Health publisher lease keep-alive channel closed, recreating lease")
				for {
					select {
					case <-hp.stopChan:
						return
					default:
					}

					if err := hp.createLease(); err != nil {
						slog.Error("Failed to recreate lease, retrying...", "error", err.Error())
						select {
						case <-hp.stopChan:
							return
						case <-time.After(5 * time.Second):
						}
						continue
					}

					newChan, err := hp.db.KeepAlive(hp.leaseID)
					if err != nil {
						slog.Error("Failed to restart keep-alive, retrying...", "error", err.Error())
						select {
						case <-hp.stopChan:
							return
						case <-time.After(5 * time.Second):
						}
						continue
					}

					slog.Info("Lease and keep-alive restored successfully")
					keepAliveChan = newChan
					break
				}
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

func (hp *HealthPublisher) collectMetadata() map[string]any {
	return map[string]any{
		"version": config.GetVersion(),
	}
}
