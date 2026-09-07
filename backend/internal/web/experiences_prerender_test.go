//go:build embed

package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestExperienceRoutesServeReadablePrerenderedDocuments(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := newPrerenderTestServer(t)
	router := gin.New()
	router.Use(server.Middleware())

	tests := []struct {
		path    string
		markers []string
	}{
		{
			path: "/experiences",
			markers: []string{
				"AI 使用经验分享",
				"接入与排障",
				"GPT-6 已接入，为什么 Codex 仍然看不见？",
				`href="/error-experiences/gpt-6-astra-not-visible"`,
				"适用：Codex 桌面使用者",
			},
		},
		{
			path: "/error-experiences/gpt-6-astra-not-visible",
			markers: []string{
				"GPT-6 已接入，为什么 Codex 仍然看不见？",
				`href="/experiences"`,
				"支持、可见与选中不是同一个开关",
				"恢复验收",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, test.path, nil))
			if recorder.Code != http.StatusOK {
				t.Fatalf("GET %s: expected 200, got %d", test.path, recorder.Code)
			}

			body := recorder.Body.String()
			for _, marker := range test.markers {
				if !strings.Contains(body, marker) {
					t.Errorf("GET %s: expected readable prerendered content %q", test.path, marker)
				}
			}
			if !strings.Contains(body, `script type="module"`) {
				t.Errorf("GET %s: prerendered document must retain the Vue client entry point", test.path)
			}
		})
	}
}
