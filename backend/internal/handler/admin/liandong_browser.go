package admin

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *LiandongToolkitHandler) browserService(c *gin.Context) *service.LiandongRestockService {
	c.Header("Cache-Control", "private, no-store")
	if h != nil && h.browser != nil {
		return h.browser
	}
	response.Error(c, 503, "Chrome 补货服务不可用")
	return nil
}
func browserBind(c *gin.Context, v any) bool {
	present, err := bindLiandongToolkitJSON(c, v, true)
	if err != nil || !present {
		writeLiandongToolkitRequestError(c, "invalid Chrome restock request")
		return false
	}
	return true
}
func browserRespond(c *gin.Context, value any, err error) {
	if err != nil {
		writeLiandongToolkitError(c, err, "Chrome restock")
		return
	}
	response.Success(c, value)
}
func (h *LiandongToolkitHandler) BrowserStatus(c *gin.Context) {
	s := h.browserService(c)
	if s == nil {
		return
	}
	v, e := s.BrowserStatus(c.Request.Context())
	browserRespond(c, v, e)
}
func (h *LiandongToolkitHandler) BrowserSaveConfig(c *gin.Context) {
	s := h.browserService(c)
	if s == nil {
		return
	}
	var v service.LiandongBrowserConfig
	if !browserBind(c, &v) {
		return
	}
	out, e := s.BrowserSaveConfig(c.Request.Context(), v)
	browserRespond(c, out, e)
}
func (h *LiandongToolkitHandler) BrowserCreateDevice(c *gin.Context) {
	s := h.browserService(c)
	if s == nil {
		return
	}
	var v struct {
		Name     string  `json:"name"`
		GoodsIDs []int64 `json:"goods_ids"`
	}
	if !browserBind(c, &v) {
		return
	}
	d, key, e := s.BrowserCreateDevice(c.Request.Context(), v.Name, v.GoodsIDs)
	browserRespond(c, gin.H{"device": d, "device_key": key}, e)
}
func (h *LiandongToolkitHandler) BrowserRevokeDevice(c *gin.Context) {
	s := h.browserService(c)
	if s == nil {
		return
	}
	e := s.BrowserRevokeDevice(c.Request.Context(), c.Param("id"))
	browserRespond(c, gin.H{"revoked": true}, e)
}
func (h *LiandongToolkitHandler) BrowserAdminResume(c *gin.Context) {
	s := h.browserService(c)
	if s == nil {
		return
	}
	var v struct{}
	if !browserBind(c, &v) {
		return
	}
	if _, e := s.BrowserResume(c.Request.Context(), ""); e != nil {
		browserRespond(c, nil, e)
		return
	}
	out, e := s.BrowserStatus(c.Request.Context())
	browserRespond(c, out, e)
}
func (h *LiandongToolkitHandler) BrowserPublicProducts(c *gin.Context) {
	s := h.browserService(c)
	if s == nil {
		return
	}
	v, e := s.BrowserPublicProducts(c.Request.Context())
	browserRespond(c, gin.H{"products": v}, e)
}

// Device credentials are resolved only here; they are not application API keys.
func (h *LiandongToolkitHandler) BrowserDeviceAuth(c *gin.Context) {
	s := h.browserService(c)
	if s == nil {
		c.Abort()
		return
	}
	header := c.GetHeader("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		response.Error(c, 401, "补货设备凭据无效或已撤销")
		c.Abort()
		return
	}
	id, e := s.BrowserAuthenticate(c.Request.Context(), strings.TrimPrefix(header, "Bearer "))
	if e != nil {
		browserRespond(c, nil, e)
		c.Abort()
		return
	}
	c.Set("ldxp_browser_device", id)
	c.Next()
}
func (h *LiandongToolkitHandler) BrowserDeviceConfig(c *gin.Context) {
	s := h.browserService(c)
	if s == nil {
		return
	}
	config, device, e := s.BrowserConfig(c.Request.Context(), c.GetString("ldxp_browser_device"))
	if e != nil {
		browserRespond(c, nil, e)
		return
	}
	browserRespond(c, gin.H{"enabled": config.Enabled, "paused_reason": config.PausedReason, "products": config.Products, "device": device}, nil)
}
func (h *LiandongToolkitHandler) BrowserInventory(c *gin.Context) {
	s := h.browserService(c)
	if s == nil {
		return
	}
	var v service.LiandongBrowserInventoryReport
	if !browserBind(c, &v) {
		return
	}
	out, e := s.BrowserInventory(c.Request.Context(), c.GetString("ldxp_browser_device"), v)
	browserRespond(c, out, e)
}
func (h *LiandongToolkitHandler) BrowserClaim(c *gin.Context) {
	s := h.browserService(c)
	if s == nil {
		return
	}
	var v struct {
		GoodsID int64 `json:"goods_id"`
	}
	if !browserBind(c, &v) {
		return
	}
	out, e := s.BrowserClaim(c.Request.Context(), c.GetString("ldxp_browser_device"), v.GoodsID)
	browserRespond(c, gin.H{"batch": out}, e)
}
func (h *LiandongToolkitHandler) BrowserStart(c *gin.Context) {
	s := h.browserService(c)
	if s == nil {
		return
	}
	var v struct{}
	if !browserBind(c, &v) {
		return
	}
	out, e := s.BrowserBatchAction(c.Request.Context(), c.GetString("ldxp_browser_device"), c.Param("id"), "start")
	browserRespond(c, gin.H{"batch": out}, e)
}
func (h *LiandongToolkitHandler) BrowserResult(c *gin.Context) {
	s := h.browserService(c)
	if s == nil {
		return
	}
	var v struct {
		Outcome string `json:"outcome"`
	}
	if !browserBind(c, &v) {
		return
	}
	if v.Outcome == "start" {
		writeLiandongToolkitRequestError(c, "invalid result outcome")
		return
	}
	out, e := s.BrowserBatchAction(c.Request.Context(), c.GetString("ldxp_browser_device"), c.Param("id"), v.Outcome)
	browserRespond(c, gin.H{"batch": out}, e)
}
func (h *LiandongToolkitHandler) BrowserHeartbeat(c *gin.Context) {
	s := h.browserService(c)
	if s == nil {
		return
	}
	var v struct {
		Authorization string `json:"authorization"`
	}
	if !browserBind(c, &v) {
		return
	}
	out, e := s.BrowserHeartbeat(c.Request.Context(), c.GetString("ldxp_browser_device"), v.Authorization)
	if e != nil {
		browserRespond(c, nil, e)
		return
	}
	browserRespond(c, gin.H{"enabled": out.Enabled, "paused_reason": out.PausedReason}, nil)
}
func (h *LiandongToolkitHandler) BrowserDeviceResume(c *gin.Context) {
	s := h.browserService(c)
	if s == nil {
		return
	}
	var v struct{}
	if !browserBind(c, &v) {
		return
	}
	out, e := s.BrowserResume(c.Request.Context(), c.GetString("ldxp_browser_device"))
	if e != nil {
		browserRespond(c, nil, e)
		return
	}
	browserRespond(c, gin.H{"enabled": out.Enabled, "paused_reason": out.PausedReason}, nil)
}

func (h *LiandongToolkitHandler) BrowserDeviceStockTarget(c *gin.Context) {
	s := h.browserService(c)
	if s == nil {
		return
	}
	var req service.LiandongBrowserStockTargetRequest
	if !browserBind(c, &req) {
		return
	}
	products, err := s.BrowserSetStockTarget(c.Request.Context(), c.GetString("ldxp_browser_device"), req)
	browserRespond(c, gin.H{"products": products}, err)
}
