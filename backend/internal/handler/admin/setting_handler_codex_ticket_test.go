package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// The harvest proxy moved to the managed proxy pool on the Codex tickets page;
// the settings API neither returns nor stores a proxy URL any more.
func TestSettingsNoLongerAcceptHarvestProxyURL(t *testing.T) {
	const legacyKey = "openai_codex_ticket_harvest_proxy_url"
	h, repo := newStepUpSwitchTestHandler(t, map[string]string{})
	rec := doUpdateSettings(t, h, map[string]any{legacyKey: "socks5h://user:new-secret@new.example.com:1080"}, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	_, stored := repo.values[legacyKey]
	require.False(t, stored)
	require.NotContains(t, rec.Body.String(), "harvest_proxy")
	require.NotContains(t, rec.Body.String(), "new-secret")

	get := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(get)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings", nil)
	h.GetSettings(c)
	require.Equal(t, http.StatusOK, get.Code)
	require.NotContains(t, get.Body.String(), "harvest_proxy")
	require.Contains(t, get.Body.String(), `"openai_codex_ticket_enabled"`)
}
