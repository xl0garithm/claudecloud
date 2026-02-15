package service

import (
	"context"
	"log"
	"time"

	"github.com/logan/cloudcode/internal/ent"
)

// UsageTracker records active usage time per user.
type UsageTracker struct {
	db       *ent.Client
	interval time.Duration
	logger   *log.Logger
}

// NewUsageTracker creates a new UsageTracker.
func NewUsageTracker(db *ent.Client, interval time.Duration, logger *log.Logger) *UsageTracker {
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
		u.logger.Printf("usage: query owner for instance %d: %v", inst.ID, err)
		return
	}

	hours := u.interval.Hours()
	_, err = u.db.User.UpdateOneID(owner.ID).
		AddUsageHours(hours).
		Save(ctx)
	if err != nil {
		u.logger.Printf("usage: record for user %d: %v", owner.ID, err)
	}
}
