package cli

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStats_Percentiles(t *testing.T) {
	t.Parallel()

	s := &Stats{}

	// Feed 100 latencies: 1ms, 2ms, ..., 100ms.
	for i := 1; i <= 100; i++ {
		s.Record(time.Duration(i)*time.Millisecond, 200)
	}

	assert.Equal(t, int64(50), s.Percentile(50).Milliseconds())
	assert.Equal(t, int64(95), s.Percentile(95).Milliseconds())
	assert.Equal(t, int64(99), s.Percentile(99).Milliseconds())
}

func TestStats_Percentile_Empty(t *testing.T) {
	t.Parallel()
	s := &Stats{}
	assert.Equal(t, time.Duration(0), s.Percentile(99))
}

func TestStats_StatusCounts(t *testing.T) {
	t.Parallel()

	s := &Stats{}
	s.Record(10*time.Millisecond, 202)
	s.Record(10*time.Millisecond, 202)
	s.Record(10*time.Millisecond, 400)
	s.Record(10*time.Millisecond, 422)
	s.Record(10*time.Millisecond, 500)

	assert.Equal(t, int64(5), s.total.Load())
	assert.Equal(t, int64(2), s.success.Load())
	assert.Equal(t, int64(2), s.client4xx.Load())
	assert.Equal(t, int64(1), s.server5xx.Load())
}

func TestPrintInboxResult_Table(t *testing.T) {
	t.Parallel()

	r := InboxResult{
		Requests:  5000,
		Successes: 4988,
		C4xx:      9,
		C5xx:      3,
		RPS:       312.4,
		Duration:  16.0,
		P50Ms:     18,
		P95Ms:     52,
		P99Ms:     130,
	}

	var buf bytes.Buffer
	PrintInboxResult(&buf, r, false)
	out := buf.String()

	require.Contains(t, out, "Inbox Flood Results")
	assert.Contains(t, out, "5000")
	assert.Contains(t, out, "4988")
	assert.Contains(t, out, "312.4")
	assert.Contains(t, out, "18ms")
}

func TestPrintFanoutResult_Table(t *testing.T) {
	t.Parallel()

	r := FanoutResult{
		FollowersSeeded:    10000,
		Instances:          50,
		DeliveriesExpected: 10000,
		DeliveriesReceived: 9997,
		FirstDeliveryS:     1.2,
		LastDeliveryS:      14.8,
		DeliveryRate:       675,
		NATSLagBefore:      0,
		NATSLagAfter:       0,
	}

	var buf bytes.Buffer
	PrintFanoutResult(&buf, r, false)
	out := buf.String()

	require.Contains(t, out, "Fanout Results")
	assert.Contains(t, out, "10000")
	assert.Contains(t, out, "9997")
}

func TestPrintInboxResult_JSON(t *testing.T) {
	t.Parallel()

	r := InboxResult{Requests: 100, Successes: 99}
	var buf bytes.Buffer
	PrintInboxResult(&buf, r, true)
	assert.Contains(t, buf.String(), `"requests"`)
	assert.Contains(t, buf.String(), `"successes"`)
}
