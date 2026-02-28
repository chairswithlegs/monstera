package observability

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIncNATSPublish_noopWhenUnset(t *testing.T) {
	prev := defaultMetrics
	t.Cleanup(func() { defaultMetrics = prev })
	defaultMetrics = nil
	IncNATSPublish("federation.deliver.create", "ok")
	IncNATSPublish("federation.deliver.create", "error")
}

func TestIncNATSPublish_incrementsWhenSet(t *testing.T) {
	prev := defaultMetrics
	t.Cleanup(func() { defaultMetrics = prev })
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)
	SetMetrics(m)
	IncNATSPublish("federation.deliver.create", "ok")
	IncNATSPublish("federation.deliver.create", "ok")
	IncNATSPublish("federation.deliver.create", "error")
	metrics, err := reg.Gather()
	require.NoError(t, err)
	var okCount, errCount float64
	for _, mf := range metrics {
		if mf.GetName() != "monstera_fed_nats_publish_total" {
			continue
		}
		for _, m := range mf.GetMetric() {
			for _, l := range m.GetLabel() {
				if l.GetName() == "result" {
					if l.GetValue() == "ok" {
						okCount += m.GetCounter().GetValue()
					} else if l.GetValue() == "error" {
						errCount += m.GetCounter().GetValue()
					}
				}
			}
		}
		break
	}
	assert.InDelta(t, 2.0, okCount, 0.01)
	assert.InDelta(t, 1.0, errCount, 0.01)
}
