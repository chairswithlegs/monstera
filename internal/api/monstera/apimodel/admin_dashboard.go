package apimodel

// AdminDashboard is the response for GET /admin/dashboard.
type AdminDashboard struct {
	LocalUsersCount    int64 `json:"local_users_count"`
	LocalStatusesCount int64 `json:"local_statuses_count"`
	OpenReportsCount   int   `json:"open_reports_count"`
}
