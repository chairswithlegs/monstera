package observability

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMetrics_registersAllCollectors(t *testing.T) {
	t.Parallel()

	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)
	require.NotNil(t, m)
	assert.NotNil(t, m.HTTPRequestsTotal)
	assert.NotNil(t, m.HTTPRequestDurationSeconds)
	assert.NotNil(t, m.FederationDeliveriesTotal)
	assert.NotNil(t, m.ActiveSSEConnections)
	assert.NotNil(t, m.NATSPublishTotal)
	assert.NotNil(t, m.DBQueryDurationSeconds)
	assert.NotNil(t, m.MediaUploadBytesTotal)
	assert.NotNil(t, m.AccountsTotal)
}
