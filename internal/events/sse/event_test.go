package sse

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamKeyToSubject(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		streamKey string
		want      string
	}{
		{"public", StreamPublic, SubjectPrefixPublic},
		{"public local", StreamPublicLocal, SubjectPrefixPublicLocal},
		{"user", StreamUserPrefix + "acc123", SubjectPrefixUser + "acc123"},
		{"hashtag", StreamHashtagPrefix + "golang", SubjectPrefixHashtag + "golang"},
		{"empty user", StreamUserPrefix, SubjectPrefixUser},
		{"unknown", "unknown", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StreamKeyToSubject(tt.streamKey)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSubjectToStreamKey(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		subject string
		want    string
	}{
		{"public", SubjectPrefixPublic, StreamPublic},
		{"public local", SubjectPrefixPublicLocal, StreamPublicLocal},
		{"user", SubjectPrefixUser + "acc123", StreamUserPrefix + "acc123"},
		{"hashtag", SubjectPrefixHashtag + "golang", StreamHashtagPrefix + "golang"},
		{"unknown", "events.other", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SubjectToStreamKey(tt.subject)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestStreamKeyMetricLabel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		streamKey string
		want      string
	}{
		{"public", StreamPublic, "public"},
		{"public local", StreamPublicLocal, "public:local"},
		{"user", StreamUserPrefix + "acc123", "user"},
		{"hashtag", StreamHashtagPrefix + "golang", "hashtag"},
		{"unknown", "other", "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StreamKeyMetricLabel(tt.streamKey)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDecodeSSEEvent(t *testing.T) {
	t.Parallel()
	ev := SSEEvent{Stream: "user", Event: "update", Data: `{"id":"123"}`}
	data, err := json.Marshal(ev)
	require.NoError(t, err)

	decoded, err := DecodeSSEEvent(data)
	require.NoError(t, err)
	assert.Equal(t, ev.Stream, decoded.Stream)
	assert.Equal(t, ev.Event, decoded.Event)
	assert.Equal(t, ev.Data, decoded.Data)
}

func TestDecodeSSEEvent_InvalidJSON(t *testing.T) {
	t.Parallel()
	_, err := DecodeSSEEvent([]byte("not json"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode SSEEvent")
}
