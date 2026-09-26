package main

import (
	"context"
	"time"

	"himate.local/services/internal/common"
)

func central8GatewayMigration() common.Migration {
	return common.Migration{
		Version: 16,
		Name:    "central-8-partner-portal-activity-telemetry",
		Statements: []string{
			`CREATE TABLE IF NOT EXISTS identity.partner_portal_activity_buckets(
				partner_id TEXT NOT NULL,
				user_id TEXT NOT NULL,
				bucket_start TIMESTAMPTZ NOT NULL,
				request_count BIGINT NOT NULL DEFAULT 1,
				first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				PRIMARY KEY(partner_id,user_id,bucket_start)
			)`,
			`CREATE INDEX IF NOT EXISTS partner_portal_activity_partner_time_idx
				ON identity.partner_portal_activity_buckets(partner_id,bucket_start DESC)`,
		},
	}
}

func (a *app) recordPartnerPortalActivity(u partnerUser) {
	if u.PartnerID == "" || u.ID == "" {
		return
	}
	now := time.Now().UTC()
	bucket := now.Truncate(5 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
	defer cancel()
	_, _ = a.db.ExecContext(ctx, `INSERT INTO identity.partner_portal_activity_buckets(
			partner_id,user_id,bucket_start,request_count,first_seen_at,last_seen_at)
		VALUES($1,$2,$3,1,$4,$4)
		ON CONFLICT(partner_id,user_id,bucket_start) DO UPDATE SET
			request_count=identity.partner_portal_activity_buckets.request_count+1,
			first_seen_at=LEAST(identity.partner_portal_activity_buckets.first_seen_at,EXCLUDED.first_seen_at),
			last_seen_at=GREATEST(identity.partner_portal_activity_buckets.last_seen_at,EXCLUDED.last_seen_at)`,
		u.PartnerID, u.ID, bucket, now)
}
