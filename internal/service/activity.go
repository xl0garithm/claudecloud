package service

import (
	"context"
	"log"
	"time"

	"github.com/logan/cloudcode/internal/ent"
	entinstance "github.com/logan/cloudcode/internal/ent/instance"
	"github.com/logan/cloudcode/internal/provider"
)

// ActivityService polls running instances and auto-pauses idle ones.
type ActivityService struct {
	db            *ent.Client
	provider      provider.Provisioner
	logger        *log.Logger
	interval      time.Duration
	idleThreshold time.Duration
	stopCh        chan struct{}
}

// NewActivityService creates a new ActivityService.
func NewActivityService(
	db *ent.Client,
	prov provider.Provisioner,
	logger *log.Logger,
	interval time.Duration,
	idleThreshold time.Duration,
) *ActivityService {
	return &ActivityService{
		db:            db,
		provider:      prov,
		logger:        logger,
		interval:      interval,
		idleThreshold: idleThreshold,
		stopCh:        make(chan struct{}),
	}
}

// Start begins the activity polling loop in a goroutine.
func (a *ActivityService) Start() {
	go a.run()
	a.logger.Printf("activity: started (interval=%s, idle_threshold=%s)", a.interval, a.idleThreshold)
}

// Stop signals the activity loop to stop.
func (a *ActivityService) Stop() {
	close(a.stopCh)
	a.logger.Println("activity: stopped")
}

func (a *ActivityService) run() {
	ticker := time.NewTicker(a.interval)
	defer ticker.Stop()

	for {
		select {
		case <-a.stopCh:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			a.checkAll(ctx)
			cancel()
		}
	}
}

func (a *ActivityService) checkAll(ctx context.Context) {
	// Find all running instances
	instances, err := a.db.Instance.Query().
		Where(entinstance.StatusEQ("running")).
		All(ctx)
	if err != nil {
		a.logger.Printf("activity: query running instances: %v", err)
		return
	}

	now := time.Now()
	for _, inst := range instances {
		a.checkInstance(ctx, inst, now)
	}
}

func (a *ActivityService) checkInstance(ctx context.Context, inst *ent.Instance, now time.Time) {
	info, err := a.provider.Activity(ctx, inst.ProviderID)
	if err != nil {
		a.logger.Printf("activity: check %s: %v", inst.ProviderID, err)
		return
	}

	if info.IsActive {
		// Update last_activity_at
		_, err := inst.Update().SetLastActivityAt(now).Save(ctx)
		if err != nil {
			a.logger.Printf("activity: update timestamp %d: %v", inst.ID, err)
		}
		return
	}

	// Check if idle long enough to auto-pause
	lastActivity := inst.LastActivityAt
	if lastActivity == nil {
		// No recorded activity — use created_at as baseline
		lastActivity = &inst.CreatedAt
	}

	idleDuration := now.Sub(*lastActivity)
	if idleDuration >= a.idleThreshold {
		a.logger.Printf("activity: auto-pausing instance %d (idle %s)", inst.ID, idleDuration.Round(time.Minute))
		if err := a.provider.Pause(ctx, inst.ProviderID); err != nil {
			a.logger.Printf("activity: pause %d: %v", inst.ID, err)
			return
		}
		_, _ = inst.Update().SetStatus("stopped").Save(ctx)
	}
}

// CheckInstance is exported for testing. Checks a single instance's activity.
func (a *ActivityService) CheckInstance(ctx context.Context, inst *ent.Instance, now time.Time) {
	a.checkInstance(ctx, inst, now)
}
