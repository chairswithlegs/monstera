package apimodel

import "github.com/chairswithlegs/monstera/internal/service"

// AdminMetrics is the response for GET /admin/metrics.
type AdminMetrics struct {
	LocalAccounts    int64 `json:"local_accounts"`
	RemoteAccounts   int64 `json:"remote_accounts"`
	LocalStatuses    int64 `json:"local_statuses"`
	RemoteStatuses   int64 `json:"remote_statuses"`
	KnownInstances   int64 `json:"known_instances"`
	OpenReports      int64 `json:"open_reports"`
	DeliveryDLQDepth int64 `json:"delivery_dlq_depth"`
	FanoutDLQDepth   int64 `json:"fanout_dlq_depth"`
}

// AdminMetricsFromService converts a service.AdminMetrics to an API response.
func AdminMetricsFromService(m *service.AdminMetrics) AdminMetrics {
	return AdminMetrics{
		LocalAccounts:    m.LocalAccounts,
		RemoteAccounts:   m.RemoteAccounts,
		LocalStatuses:    m.LocalStatuses,
		RemoteStatuses:   m.RemoteStatuses,
		KnownInstances:   m.KnownInstances,
		OpenReports:      m.OpenReports,
		DeliveryDLQDepth: m.DeliveryDLQDepth,
		FanoutDLQDepth:   m.FanoutDLQDepth,
	}
}
