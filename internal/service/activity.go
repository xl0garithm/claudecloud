package service

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/logan/cloudcode/internal/ent"
	entinstance "github.com/logan/cloudcode/internal/ent/instance"
	"github.com/logan/cloudcode/internal/provider"
)

var activityTracer = otel.Tracer("cloudcode/service/activity")
var meter = otel.Meter("cloudcode/service/activity")

// ActivityService polls running instances and auto-pauses idle ones.
type ActivityService struct {
	db            *ent.Client
	provider      provider.Provisioner
	logger        *slog.Logger
	interval      time.Duration
	idleThreshold time.Duration
	stopCh        chan struct{}
	onActive      func(ctx context.Context, inst *ent.Instance) // usage callback

	// Track consecutive health check failures per instance
	healthFailures sync.Map // map[int]int (instance ID → consecutive failures)
}

// SetOnActive sets a callback invoked when an instance is detected as active.
func (a *ActivityService) SetOnActive(fn func(ctx context.Context, inst *ent.Instance)) {
	a.onActive = fn
}

// NewActivityService creates a new ActivityService.
func NewActivityService(
	db *ent.Client,
	prov provider.Provisioner,
	logger *slog.Logger,
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
	a.logger.Info("activity service started", "interval", a.interval, "idle_threshold", a.idleThreshold)
}

// Stop signals the activity loop to stop.
func (a *ActivityService) Stop() {
	close(a.stopCh)
	a.logger.Info("activity service stopped")
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
	ctx, span := activityTracer.Start(ctx, "activity.check_all")
	defer span.End()

	// Find all running instances
	instances, err := a.db.Instance.Query().
		Where(entinstance.StatusEQ("running")).
		All(ctx)
	if err != nil {
		a.logger.Error("failed to query running instances", "error", err)
		return
	}

	span.SetAttributes(attribute.Int("instance_count", len(instances)))

	now := time.Now()
	activeCount := 0
	for _, inst := range instances {
		a.checkInstance(ctx, inst, now)
	}

	// Update OTEL metrics
	if totalGauge, err := meter.Int64UpDownCounter("cloudcode.instances.total"); err == nil {
		totalGauge.Add(ctx, 0, metric.WithAttributes(attribute.String("status", "running")))
	}
	if activeGauge, err := meter.Int64Gauge("cloudcode.instances.active"); err == nil {
		activeGauge.Record(ctx, int64(activeCount))
	}
}

func (a *ActivityService) checkInstance(ctx context.Context, inst *ent.Instance, now time.Time) {
	info, err := a.provider.Activity(ctx, inst.ProviderID)
	if err != nil {
		a.logger.Error("activity check failed", "instance_id", inst.ID, "provider_id", inst.ProviderID, "error", err)
		return
	}

	// Track health check failures
	if !info.IsHealthy {
		var failures int
		if v, ok := a.healthFailures.Load(inst.ID); ok {
			failures = v.(int)
		}
		failures++
		a.healthFailures.Store(inst.ID, failures)

		if failures >= 3 {
			a.logger.Warn("instance unhealthy for 3 consecutive checks",
				"instance_id", inst.ID, "provider_id", inst.ProviderID)
		}
	} else {
		a.healthFailures.Delete(inst.ID)
	}

	if info.IsActive {
		// Update last_activity_at
		_, err := inst.Update().SetLastActivityAt(now).Save(ctx)
		if err != nil {
			a.logger.Error("failed to update activity timestamp", "instance_id", inst.ID, "error", err)
		}
		// Notify usage tracker
		if a.onActive != nil {
			a.onActive(ctx, inst)
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
		a.logger.Info("auto-pausing idle instance", "instance_id", inst.ID, "idle_duration", idleDuration.Round(time.Minute))
		if err := a.provider.Pause(ctx, inst.ProviderID); err != nil {
			a.logger.Error("failed to pause instance", "instance_id", inst.ID, "error", err)
			return
		}
		_, _ = inst.Update().SetStatus("stopped").Save(ctx)
		a.healthFailures.Delete(inst.ID)
	}
}

// CheckInstance is exported for testing. Checks a single instance's activity.
func (a *ActivityService) CheckInstance(ctx context.Context, inst *ent.Instance, now time.Time) {
	a.checkInstance(ctx, inst, now)
}
