package service

import (
	"context"
	"log"
	"time"
)

// CronService runs periodic background tasks.
type CronService struct {
	netbird  *NetbirdService
	logger   *log.Logger
	interval time.Duration
	stopCh   chan struct{}
}

// NewCronService creates a new CronService that periodically cleans up Netbird resources.
func NewCronService(netbird *NetbirdService, logger *log.Logger, interval time.Duration) *CronService {
	return &CronService{
		netbird:  netbird,
		logger:   logger,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start begins the periodic cleanup loop in a goroutine.
func (c *CronService) Start() {
	go c.run()
	c.logger.Printf("cron: started (interval=%s)", c.interval)
}

// Stop signals the cron loop to stop.
func (c *CronService) Stop() {
	close(c.stopCh)
	c.logger.Println("cron: stopped")
}

func (c *CronService) run() {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			if err := c.netbird.CleanupExpiredKeys(ctx); err != nil {
				c.logger.Printf("cron: cleanup expired keys: %v", err)
			}
			cancel()
		}
	}
}
