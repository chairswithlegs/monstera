package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func decodeErrorResponse(t *testing.T, rec *httptest.ResponseRecorder) ErrorResponse {
	t.Helper()
	var resp ErrorResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	return resp
}

func TestHandleError_DomainSentinels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"not found (domain)", domain.ErrNotFound, http.StatusNotFound, CodeNotFound},
		{"not found (api)", ErrNotFound, http.StatusNotFound, CodeNotFound},
		{"conflict", domain.ErrConflict, http.StatusConflict, CodeConflict},
		{"forbidden (domain)", domain.ErrForbidden, http.StatusForbidden, CodeForbidden},
		{"forbidden (api)", ErrForbidden, http.StatusForbidden, CodeForbidden},
		{"unauthorized (domain)", domain.ErrUnauthorized, http.StatusUnauthorized, CodeUnauthorized},
		{"unauthorized (api)", ErrUnauthorized, http.StatusUnauthorized, CodeUnauthorized},
		{"validation", domain.ErrValidation, http.StatusBadRequest, CodeBadRequest},
		{"bad request", ErrBadRequest, http.StatusBadRequest, CodeBadRequest},
		{"unprocessable (domain)", domain.ErrUnprocessable, http.StatusUnprocessableEntity, CodeValidationFailed},
		{"unprocessable (api)", ErrUnprocessable, http.StatusUnprocessableEntity, CodeValidationFailed},
		{"rate limited", domain.ErrRateLimited, http.StatusTooManyRequests, CodeRateLimited},
		{"gone", domain.ErrGone, http.StatusGone, CodeGone},
		{"account suspended", domain.ErrAccountSuspended, http.StatusForbidden, CodeAccountSuspended},
		{"internal server error", ErrInternalServerError, http.StatusInternalServerError, CodeInternalError},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()
			HandleError(rec, req, tc.err)
			assert.Equal(t, tc.wantStatus, rec.Code)
			resp := decodeErrorResponse(t, rec)
			assert.Equal(t, tc.wantCode, resp.Code)
			assert.Empty(t, resp.Params)
		})
	}
}

func TestHandleError_WrappedDomainErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		wantError  string
	}{
		{
			"wrapped not found",
			fmt.Errorf("GetAccountByID(123): %w", domain.ErrNotFound),
			http.StatusNotFound, CodeNotFound, "Record not found",
		},
		{
			"wrapped conflict",
			fmt.Errorf("CreateAccount: %w", domain.ErrConflict),
			http.StatusConflict, CodeConflict, "CreateAccount: conflict",
		},
		{
			"wrapped validation",
			fmt.Errorf("title: %w", domain.ErrValidation),
			http.StatusBadRequest, CodeBadRequest, "title: validation error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()
			HandleError(rec, req, tc.err)
			assert.Equal(t, tc.wantStatus, rec.Code)
			resp := decodeErrorResponse(t, rec)
			assert.Equal(t, tc.wantCode, resp.Code)
			assert.Equal(t, tc.wantError, resp.Error)
		})
	}
}

func TestHandleError_UnhandledError(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	HandleError(rec, req, errors.New("some unexpected error"))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	resp := decodeErrorResponse(t, rec)
	assert.Equal(t, CodeInternalError, resp.Code)
}

func TestHandleError_APIError_WithParams(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	HandleError(rec, req, NewMissingRequiredFieldError("display_name"))
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	resp := decodeErrorResponse(t, rec)
	assert.Equal(t, CodeValidationFailed, resp.Code)
	assert.Equal(t, "unprocessable entity: display_name is required", resp.Error)
	assert.Equal(t, "display_name", resp.Params[ParamKeyField])
	assert.Equal(t, "missing_required_field", resp.Params[ParamKeyReason])
}

func TestAPIError_ErrorsIs(t *testing.T) {
	t.Parallel()
	err := NewMissingRequiredFieldError("display_name")
	assert.ErrorIs(t, err, ErrUnprocessable)
}

func TestAPIError_WithParams_Chaining(t *testing.T) {
	t.Parallel()
	params := map[string]string{"field": "bio"}
	err := NewMissingRequiredFieldError("bio").WithParams(params)
	assert.Equal(t, params, err.params)
}
