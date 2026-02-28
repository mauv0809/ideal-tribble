package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mauv0809/ideal-tribble/internal/club"
	"github.com/mauv0809/ideal-tribble/internal/notifier"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeepHealthCheckHandler_AllHealthy(t *testing.T) {
	store := club.NewMock()
	notifierMock := notifier.NewMock()

	handler := DeepHealthCheckHandler(store, notifierMock)

	req := httptest.NewRequest(http.MethodGet, "/health/deep", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response HealthResponse
	err := json.NewDecoder(rr.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, "healthy", response.Status)
	assert.NotEmpty(t, response.Timestamp)
	assert.Len(t, response.Checks, 2)

	for _, check := range response.Checks {
		assert.Equal(t, "healthy", check.Status)
		assert.Empty(t, check.Error)
		assert.GreaterOrEqual(t, check.LatencyMs, int64(0))
	}
}

func TestDeepHealthCheckHandler_DatabaseUnhealthy(t *testing.T) {
	store := &unhealthyMockStore{}
	notifierMock := notifier.NewMock()

	handler := DeepHealthCheckHandler(store, notifierMock)

	req := httptest.NewRequest(http.MethodGet, "/health/deep", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)

	var response HealthResponse
	err := json.NewDecoder(rr.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, "unhealthy", response.Status)

	var dbCheck *HealthCheckResult
	for i := range response.Checks {
		if response.Checks[i].Name == "database" {
			dbCheck = &response.Checks[i]
			break
		}
	}

	require.NotNil(t, dbCheck)
	assert.Equal(t, "unhealthy", dbCheck.Status)
	assert.Equal(t, "database connection failed", dbCheck.Error)
}

func TestHealthCheckHandler_SimpleOK(t *testing.T) {
	store := club.NewMock()

	handler := HealthCheckHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "OK!", rr.Body.String())
}

// unhealthyMockStore is a mock store that always fails ping
type unhealthyMockStore struct {
	club.MockStore
}

func (u *unhealthyMockStore) Ping() error {
	return errors.New("database connection failed")
}
