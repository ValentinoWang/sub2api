package middleware

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

// LDXPDevice budgets the device identity without granting an administrator subject.
func (p *PanelRateLimiter) LDXPDevice() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetString("ldxp_browser_device")
		if p == nil || p.limiter == nil || id == "" {
			if c.Request.Method != http.MethodGet {
				abortPanelRateLimitUnavailable(c)
				return
			}
			c.Next()
			return
		}
		result, err := p.limiter.Allow(c.Request.Context(), "panel:ldxp:device:"+id, 120, panelRateLimitWindow)
		if err != nil {
			abortPanelRateLimitUnavailable(c)
			return
		}
		if !result.Allowed {
			abortPanelRateLimited(c, result.RetryAfter)
			return
		}
		c.Next()
	}
}
