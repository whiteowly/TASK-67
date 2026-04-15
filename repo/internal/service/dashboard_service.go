package service

import (
	"context"

	"github.com/campusrec/campusrec/internal/repo"
)

// DashboardService provides dashboard KPIs and job status information.
type DashboardService struct {
	repos *repo.Repositories
}

func NewDashboardService(repos *repo.Repositories) *DashboardService {
	return &DashboardService{repos: repos}
}

// KPIs holds key performance indicators for the admin dashboard.
type KPIs struct {
	TotalUsers          int     `json:"total_users"`
	ActiveSessions      int     `json:"active_sessions"`
	TotalOrders         int     `json:"total_orders"`
	PendingTickets      int     `json:"pending_tickets"`
	OpenModerationCases int     `json:"open_moderation_cases"`
	FillRate            float64 `json:"fill_rate"`
	MemberGrowthMonth   int     `json:"member_growth_month"`
	EngagementRate      float64 `json:"engagement_rate"`
	CoachSessionCount   int     `json:"coach_session_count"`
	// ChurnRate: fraction of users who were active 31–60 days ago but had no
	// registration or order in the last 30 days. Range [0, 1].
	// Definition: churned = active_prev_period AND NOT active_current_period.
	// active = has at least one registration or order in the period.
	ChurnRate float64 `json:"churn_rate"`
}

// GetKPIs returns current dashboard KPI metrics.
func (s *DashboardService) GetKPIs(ctx context.Context) (*KPIs, error) {
	kpis := &KPIs{}

	s.repos.User.Pool().QueryRow(ctx, `SELECT count(*) FROM users WHERE deleted_at IS NULL AND is_active = true`).Scan(&kpis.TotalUsers)
	s.repos.Catalog.Pool().QueryRow(ctx, `SELECT count(*) FROM program_sessions WHERE status = 'published' AND deleted_at IS NULL`).Scan(&kpis.ActiveSessions)
	s.repos.Order.Pool().QueryRow(ctx, `SELECT count(*) FROM orders`).Scan(&kpis.TotalOrders)
	s.repos.Ticket.Pool().QueryRow(ctx, `SELECT count(*) FROM tickets WHERE status NOT IN ('closed','resolved')`).Scan(&kpis.PendingTickets)
	s.repos.Moderation.Pool().QueryRow(ctx, `SELECT count(*) FROM moderation_cases WHERE status NOT IN ('actioned','dismissed')`).Scan(&kpis.OpenModerationCases)

	// Fill rate: avg(reserved/total) across active sessions
	s.repos.Catalog.Pool().QueryRow(ctx,
		`SELECT COALESCE(AVG(CASE WHEN total_seats > 0 THEN reserved_seats::float / total_seats ELSE 0 END), 0)
		 FROM session_seat_inventory`).Scan(&kpis.FillRate)

	// Member growth: new users in last 30 days
	s.repos.User.Pool().QueryRow(ctx,
		`SELECT count(*) FROM users WHERE created_at >= now() - interval '30 days' AND deleted_at IS NULL`).Scan(&kpis.MemberGrowthMonth)

	// Engagement: ratio of users with at least one registration or order in last 30 days
	var activeUsers int
	s.repos.User.Pool().QueryRow(ctx,
		`SELECT count(DISTINCT user_id) FROM (
			SELECT user_id FROM session_registrations WHERE created_at >= now() - interval '30 days'
			UNION SELECT user_id FROM orders WHERE created_at >= now() - interval '30 days'
		) t`).Scan(&activeUsers)
	if kpis.TotalUsers > 0 {
		kpis.EngagementRate = float64(activeUsers) / float64(kpis.TotalUsers)
	}

	// Coach productivity: count of sessions with an instructor
	s.repos.Catalog.Pool().QueryRow(ctx,
		`SELECT count(*) FROM program_sessions WHERE instructor_name IS NOT NULL AND deleted_at IS NULL`).Scan(&kpis.CoachSessionCount)

	// Churn rate: fraction of users active 31-60 days ago who had no activity in the last 30 days.
	// "Active" = at least one registration or order in the period.
	var prevActive, churned int
	s.repos.User.Pool().QueryRow(ctx, `
		WITH prev_active AS (
			SELECT DISTINCT user_id FROM (
				SELECT user_id FROM session_registrations
				WHERE created_at >= now() - interval '60 days' AND created_at < now() - interval '30 days'
				UNION
				SELECT user_id FROM orders
				WHERE created_at >= now() - interval '60 days' AND created_at < now() - interval '30 days'
			) t
		),
		curr_active AS (
			SELECT DISTINCT user_id FROM (
				SELECT user_id FROM session_registrations WHERE created_at >= now() - interval '30 days'
				UNION
				SELECT user_id FROM orders WHERE created_at >= now() - interval '30 days'
			) t
		)
		SELECT
			(SELECT count(*) FROM prev_active),
			(SELECT count(*) FROM prev_active WHERE user_id NOT IN (SELECT user_id FROM curr_active))
	`).Scan(&prevActive, &churned)
	if prevActive > 0 {
		kpis.ChurnRate = float64(churned) / float64(prevActive)
	}

	return kpis, nil
}

// JobStatusSummary holds a summary of job queue status.
type JobStatusSummary struct {
	Queued  int `json:"queued"`
	Running int `json:"running"`
	Failed  int `json:"failed"`
}

// GetJobStatus returns a summary of the job queue.
func (s *DashboardService) GetJobStatus(ctx context.Context) (*JobStatusSummary, error) {
	summary := &JobStatusSummary{}
	s.repos.Job.Pool().QueryRow(ctx, `SELECT count(*) FROM job_queue WHERE status = 'pending'`).Scan(&summary.Queued)
	s.repos.Job.Pool().QueryRow(ctx, `SELECT count(*) FROM job_queue WHERE status = 'running'`).Scan(&summary.Running)
	s.repos.Job.Pool().QueryRow(ctx, `SELECT count(*) FROM job_queue WHERE status = 'failed'`).Scan(&summary.Failed)
	return summary, nil
}
