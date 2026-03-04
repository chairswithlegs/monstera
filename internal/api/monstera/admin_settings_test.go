package monstera

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chairswithlegs/monstera-fed/internal/api/monstera/apimodel"
	"github.com/chairswithlegs/monstera-fed/internal/service"
	"github.com/chairswithlegs/monstera-fed/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminSettingsHandler_GETSettings(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	instanceSvc := service.NewInstanceService(st)
	handler := NewAdminSettingsHandler(instanceSvc)

	t.Run("returns 200 and settings", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/settings", nil)
		rec := httptest.NewRecorder()
		handler.GETSettings(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body apimodel.AdminSettings
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	})
}

func TestAdminSettingsHandler_PUTSettings(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	instanceSvc := service.NewInstanceService(st)
	handler := NewAdminSettingsHandler(instanceSvc)

	t.Run("with valid body returns 204", func(t *testing.T) {
		body := map[string]any{"settings": map[string]string{"registration_mode": "open"}}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPut, "/admin/settings", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.PUTSettings(rec, req)
		assert.Equal(t, http.StatusNoContent, rec.Code)
	})
}
