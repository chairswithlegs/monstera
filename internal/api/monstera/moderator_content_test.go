package monstera

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chairswithlegs/monstera-fed/internal/api/monstera/apimodel"
	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/service"
	"github.com/chairswithlegs/monstera-fed/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModeratorContentHandler_GETFilters(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	filterSvc := service.NewServerFilterService(st)
	handler := NewModeratorContentHandler(filterSvc)

	t.Run("returns 200 and filters list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/content/filters", nil)
		rec := httptest.NewRecorder()
		handler.GETFilters(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body apimodel.AdminServerFilterList
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.NotNil(t, body.Filters)
	})
}

func TestModeratorContentHandler_POSTFilters(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	filterSvc := service.NewServerFilterService(st)
	handler := NewModeratorContentHandler(filterSvc)

	t.Run("with valid body returns 201 and filter", func(t *testing.T) {
		body := map[string]any{"phrase": "spam", "scope": domain.ServerFilterScopeAll, "action": domain.ServerFilterActionHide}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/admin/content/filters", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.POSTFilters(rec, req)
		assert.Equal(t, http.StatusCreated, rec.Code)
		var out apimodel.AdminServerFilter
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&out))
		assert.Equal(t, "spam", out.Phrase)
	})
}

func TestModeratorContentHandler_PUTFilter(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	filterSvc := service.NewServerFilterService(st)
	handler := NewModeratorContentHandler(filterSvc)

	t.Run("with nonexistent id returns 404", func(t *testing.T) {
		body := map[string]any{"phrase": "updated", "scope": domain.ServerFilterScopeAll, "action": domain.ServerFilterActionHide}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPut, "/admin/content/filters/01nonexistent", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req = testutil.AddChiURLParam(req, "id", "01nonexistent")
		rec := httptest.NewRecorder()
		handler.PUTFilter(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestModeratorContentHandler_DELETEFilter(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	filterSvc := service.NewServerFilterService(st)
	handler := NewModeratorContentHandler(filterSvc)

	t.Run("returns 204", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/admin/content/filters/01filterid", nil)
		req = testutil.AddChiURLParam(req, "id", "01filterid")
		rec := httptest.NewRecorder()
		handler.DELETEFilter(rec, req)
		assert.Equal(t, http.StatusNoContent, rec.Code)
	})
}
