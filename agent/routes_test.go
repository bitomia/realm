package agent

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobsRouteIsRegistered(t *testing.T) {
	router := mux.NewRouter()
	createBaseRoutes(router)

	var match mux.RouteMatch
	require.True(t, router.Match(httptest.NewRequest(http.MethodPost, "/jobs", nil), &match),
		"POST /jobs must be routed")
	assert.NoError(t, match.MatchErr)
	assert.NotNil(t, match.Handler)
}

func TestJobsRouteRejectsOtherMethods(t *testing.T) {
	router := mux.NewRouter()
	createBaseRoutes(router)

	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		var match mux.RouteMatch
		router.Match(httptest.NewRequest(method, "/jobs", nil), &match)
		assert.ErrorIs(t, match.MatchErr, mux.ErrMethodMismatch, "%s /jobs must not be routed", method)
	}
}
