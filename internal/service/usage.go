package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/logan/cloudcode/internal/ent"
)

// UsageTracker records active usage time per user.
type UsageTracker struct {
	db       *ent.Client
	interval time.Duration
	logger   *slog.Logger
}

// NewUsageTracker creates a new UsageTracker.
func NewUsageTracker(db *ent.Client, interval time.Duration, logger *slog.Logger) *UsageTracker {
	return &UsageTracker{
		db:       db,
		interval: interval,
		logger:   logger,
	}
}

// RecordActive records that a user's instance was active during this check interval.
// Called by the ActivityService when activity is detected.
func (u *UsageTracker) RecordActive(ctx context.Context, inst *ent.Instance) {
	owner, err := inst.QueryOwner().Only(ctx)
	if err != nil {
		u.logger.Error("failed to query owner for usage", "instance_id", inst.ID, "error", err)
		return
	}

	hours := u.interval.Hours()
	_, err = u.db.User.UpdateOneID(owner.ID).
		AddUsageHours(hours).
		Save(ctx)
	if err != nil {
		u.logger.Error("failed to record usage", "user_id", owner.ID, "error", err)
	}
}
