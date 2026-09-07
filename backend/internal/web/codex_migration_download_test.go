//go:build embed

package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMigrationDownloadMissingArtifactDoesNotFallBackToSPA(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := newPrerenderTestServer(t)
	router := gin.New()
	router.Use(server.Middleware())

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/codex-session-migrate-missing.zip", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected missing migration download to return 404, got %d", recorder.Code)
	}
}
