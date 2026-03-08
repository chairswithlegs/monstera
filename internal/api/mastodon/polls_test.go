package mastodon

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/events"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/chairswithlegs/monstera/internal/uid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPollsHandler_GETPoll(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	statusSvc := service.NewStatusService(st, service.NoopFederationPublisher, events.NoopEventBus, nil, "https://example.com", "example.com", 500, slog.Default())
	handler := NewPollsHandler(statusSvc)

	acc, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)

	// Create a status with a poll via the service
	result, err := statusSvc.CreateWithContent(ctx, service.CreateWithContentInput{
		AccountID:         acc.ID,
		Username:          acc.Username,
		Text:              "Poll?",
		Visibility:        "public",
		DefaultVisibility: "public",
		Poll: &service.PollInput{
			Options:          []string{"A", "B"},
			ExpiresInSeconds: 3600,
			Multiple:         false,
		},
		PollLimits: &service.PollLimits{
			MaxOptions:    4,
			MinExpiration: 300,
			MaxExpiration: 2629746,
		},
	})
	require.NoError(t, err)
	require.NotNil(t, result.Poll)
	pollID := result.Poll.Poll.ID

	t.Run("unauthenticated returns poll", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/polls/"+pollID, nil)
		req = testutil.AddChiURLParam(req, "id", pollID)
		rec := httptest.NewRecorder()
		handler.GETPoll(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var out map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&out))
		assert.Equal(t, pollID, out["id"])
		assert.Equal(t, false, out["multiple"])
		assert.NotEqual(t, true, out["voted"])
	})

	t.Run("authenticated returns voted and own_votes when not voted", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/polls/"+pollID, nil)
		req = testutil.AddChiURLParam(req, "id", pollID)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.GETPoll(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var out map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&out))
		assert.NotEqual(t, true, out["voted"])
	})

	t.Run("404 for unknown poll", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/polls/01HXYZ0000000000000000000", nil)
		req = testutil.AddChiURLParam(req, "id", "01HXYZ0000000000000000000")
		rec := httptest.NewRecorder()
		handler.GETPoll(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestPollsHandler_POSTVotes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	statusSvc := service.NewStatusService(st, service.NoopFederationPublisher, events.NoopEventBus, nil, "https://example.com", "example.com", 500, slog.Default())
	handler := NewPollsHandler(statusSvc)

	acc, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "voter",
		Email:        "voter@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)

	result, err := statusSvc.CreateWithContent(ctx, service.CreateWithContentInput{
		AccountID:         acc.ID,
		Username:          acc.Username,
		Text:              "Vote?",
		Visibility:        "public",
		DefaultVisibility: "public",
		Poll: &service.PollInput{
			Options:          []string{"X", "Y", "Z"},
			ExpiresInSeconds: 3600,
			Multiple:         true,
		},
		PollLimits: &service.PollLimits{
			MaxOptions:    4,
			MinExpiration: 300,
			MaxExpiration: 2629746,
		},
	})
	require.NoError(t, err)
	require.NotNil(t, result.Poll)
	pollID := result.Poll.Poll.ID

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		body := bytes.NewBufferString(`{"choices":[0]}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/polls/"+pollID+"/votes", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.POSTVotes(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("vote returns updated poll", func(t *testing.T) {
		body := bytes.NewBufferString(`{"choices":[0,2]}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/polls/"+pollID+"/votes", body)
		req = testutil.AddChiURLParam(req, "id", pollID)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.POSTVotes(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var out map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&out))
		assert.Equal(t, true, out["voted"])
		ownVotes, ok := out["own_votes"].([]any)
		require.True(t, ok)
		require.Len(t, ownVotes, 2)
		assert.InDelta(t, 0.0, ownVotes[0].(float64), 0.01)
		assert.InDelta(t, 2.0, ownVotes[1].(float64), 0.01)
	})

	// Expired poll: create one with expires_at in the past via store
	t.Run("422 when poll expired", func(t *testing.T) {
		expiredAt := time.Now().Add(-time.Hour)
		expiredStatusID := uid.New()
		_, err := st.CreateStatus(ctx, store.CreateStatusInput{
			ID:         expiredStatusID,
			URI:        "https://example.com/statuses/" + expiredStatusID,
			AccountID:  acc.ID,
			Text:       strPtr("Expired poll"),
			Content:    strPtr("<p>Expired poll</p>"),
			Visibility: domain.VisibilityPublic,
			APID:       "https://example.com/statuses/" + expiredStatusID,
			Local:      true,
		})
		require.NoError(t, err)
		expiredPollID := uid.New()
		_, err = st.CreatePoll(ctx, store.CreatePollInput{
			ID:        expiredPollID,
			StatusID:  expiredStatusID,
			ExpiresAt: &expiredAt,
			Multiple:  false,
		})
		require.NoError(t, err)
		_, err = st.CreatePollOption(ctx, store.CreatePollOptionInput{ID: uid.New(), PollID: expiredPollID, Title: "A", Position: 0})
		require.NoError(t, err)
		_, err = st.CreatePollOption(ctx, store.CreatePollOptionInput{ID: uid.New(), PollID: expiredPollID, Title: "B", Position: 1})
		require.NoError(t, err)

		body := bytes.NewBufferString(`{"choices":[0]}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/polls/"+expiredPollID+"/votes", body)
		req = testutil.AddChiURLParam(req, "id", expiredPollID)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.POSTVotes(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})
}

func strPtr(s string) *string { return &s }
