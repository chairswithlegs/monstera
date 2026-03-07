package mastodon

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/testutil"
)

func TestMarkersHandler_GETMarkers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	markerSvc := service.NewMarkerService(st)
	handler := NewMarkersHandler(markerSvc)

	account, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/markers", nil)
		rec := httptest.NewRecorder()
		handler.GETMarkers(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("authenticated empty returns 200 and empty object", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/markers?timeline[]=home&timeline[]=notifications", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), account))
		rec := httptest.NewRecorder()
		handler.GETMarkers(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Empty(t, body)
	})
}

func TestMarkersHandler_POSTMarkers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	markerSvc := service.NewMarkerService(st)
	handler := NewMarkersHandler(markerSvc)

	account, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)

	t.Run("POST then GET returns set marker", func(t *testing.T) {
		body := POSTMarkersRequest{
			Home: &MarkerTimelineInput{LastReadID: "01HQXXX"},
		}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/markers", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), account))
		rec := httptest.NewRecorder()
		handler.POSTMarkers(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var out map[string]MarkerResponse
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&out))
		require.Contains(t, out, "home")
		assert.Equal(t, "01HQXXX", out["home"].LastReadID)
		assert.GreaterOrEqual(t, out["home"].Version, 0)

		req2 := httptest.NewRequest(http.MethodGet, "/api/v1/markers?timeline[]=home", nil)
		req2 = req2.WithContext(middleware.WithAccount(req2.Context(), account))
		rec2 := httptest.NewRecorder()
		handler.GETMarkers(rec2, req2)
		assert.Equal(t, http.StatusOK, rec2.Code)
		var getOut map[string]MarkerResponse
		require.NoError(t, json.NewDecoder(rec2.Body).Decode(&getOut))
		assert.Equal(t, "01HQXXX", getOut["home"].LastReadID)
	})
}
