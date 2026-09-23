package admin

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// CodexHarvestHandler serves the admin "Codex tickets" page: harvest proxy
// pool, speed controls, per-proxy learning, the event log and manual runs.
type CodexHarvestHandler struct {
	gateway *service.OpenAIGatewayService
	harvest *service.CodexHarvestService
}

func NewCodexHarvestHandler(gateway *service.OpenAIGatewayService, harvest *service.CodexHarvestService) *CodexHarvestHandler {
	return &CodexHarvestHandler{gateway: gateway, harvest: harvest}
}

// Get returns the page snapshot.
// GET /api/v1/admin/codex-harvest
func (h *CodexHarvestHandler) Get(c *gin.Context) {
	snapshot, err := h.gateway.CodexHarvestSnapshot(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, snapshot)
}

// UpdateControls saves the speed preset and the harvest proxy pool.
// PUT /api/v1/admin/codex-harvest/controls
func (h *CodexHarvestHandler) UpdateControls(c *gin.Context) {
	var req service.CodexHarvestControls
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if err := h.harvest.SaveControls(c.Request.Context(), req); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	controls, _, _ := h.harvest.Controls(c.Request.Context())
	response.Success(c, controls)
}

// ListNodes returns per-proxy learning records, newest first.
// GET /api/v1/admin/codex-harvest/nodes
func (h *CodexHarvestHandler) ListNodes(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	if pageSize > 100 {
		pageSize = 100
	}
	result, err := h.harvest.ListNodes(c.Request.Context(), (page-1)*pageSize, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, result.Items, result.Total, page, pageSize)
}

type resetCodexHarvestNodesRequest struct {
	RecordID int64 `json:"record_id"`
}

// ResetNodes clears one learning record, or all of them when record_id is 0.
// POST /api/v1/admin/codex-harvest/nodes/reset
func (h *CodexHarvestHandler) ResetNodes(c *gin.Context) {
	var req resetCodexHarvestNodesRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.RecordID < 0 {
		response.BadRequest(c, "Invalid record ID")
		return
	}
	if err := h.harvest.ResetNodes(c.Request.Context(), req.RecordID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"reset": true})
}

// StartManual starts a bounded background harvest for one account.
// POST /api/v1/admin/codex-harvest/accounts/:id/manual
func (h *CodexHarvestHandler) StartManual(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || accountID <= 0 {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	var req service.CodexHarvestManualRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	run, err := h.gateway.StartCodexHarvestManual(c.Request.Context(), accountID, req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Accepted(c, run)
}
