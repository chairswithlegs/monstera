package monstera

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chairswithlegs/monstera/internal/api/monstera/apimodel"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminSettingsHandler_GETSettings(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	settingsSvc := service.NewMonsteraSettingsService(st)
	handler := NewAdminSettingsHandler(settingsSvc)

	t.Run("returns 200 and settings", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/settings", nil)
		rec := httptest.NewRecorder()
		handler.GETSettings(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body apimodel.AdminSettings
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Equal(t, "open", body.RegistrationMode)
	})
}

func TestAdminSettingsHandler_PUTSettings(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	settingsSvc := service.NewMonsteraSettingsService(st)
	handler := NewAdminSettingsHandler(settingsSvc)

	t.Run("with valid body returns 204", func(t *testing.T) {
		body := apimodel.AdminSettings{RegistrationMode: "approval"}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPut, "/admin/settings", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.PUTSettings(rec, req)
		assert.Equal(t, http.StatusNoContent, rec.Code)
	})

	t.Run("with invalid registration_mode returns 422", func(t *testing.T) {
		body := apimodel.AdminSettings{RegistrationMode: "invalid"}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPut, "/admin/settings", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.PUTSettings(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})
}
