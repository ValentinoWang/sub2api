package admin

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const liandongToolkitMaxJSONBody = 4 << 20

// LiandongToolkitHandler owns only the LDXP administrator tool surface. The
// restock domain service is injected so the HTTP layer cannot reach payment
// handlers or construct credentials itself.
type LiandongToolkitHandler struct {
	toolkitService service.LiandongToolkitService
	runtime        *service.LiandongToolkitRuntime
}

// NewLiandongToolkitHandler creates a registration-ready handler. The global
// application wiring is intentionally left to a later integrator.
func NewLiandongToolkitHandler(toolkitService service.LiandongToolkitService, runtime *service.LiandongToolkitRuntime) *LiandongToolkitHandler {
	return &LiandongToolkitHandler{
		toolkitService: toolkitService,
		runtime:        runtime,
	}
}

// GetInstallation reports local facts only; it never executes the toolkit.
func (h *LiandongToolkitHandler) GetInstallation(c *gin.Context) {
	if h == nil || h.runtime == nil {
		response.Success(c, service.LiandongToolkitUnavailableInstallationStatus())
		return
	}
	c.Header("Cache-Control", "private, no-store")
	response.Success(c, h.runtime.Status())
}

// Install performs an atomic install from the configured local asset. The
// request has no path, URL, command, or archive field by design.
func (h *LiandongToolkitHandler) Install(c *gin.Context) {
	var request struct {
		Repair bool `json:"repair"`
	}
	if _, err := bindLiandongToolkitJSON(c, &request, true); err != nil {
		writeLiandongToolkitRequestError(c, "invalid LDXP toolkit installation request")
		return
	}
	if h == nil || h.runtime == nil {
		writeLiandongToolkitError(c, infraerrors.ServiceUnavailable("LDXP_TOOLKIT_UNAVAILABLE", "LDXP toolkit runtime is unavailable"), "installation")
		return
	}
	result, err := h.runtime.Install()
	if err != nil {
		writeLiandongToolkitError(c, err, "installation")
		return
	}
	response.Success(c, result)
}

// GetStatus delegates to the existing Liandong restock status contract.
func (h *LiandongToolkitHandler) GetStatus(c *gin.Context) {
	svc, ok := h.liandongToolkitService(c, "status")
	if !ok {
		return
	}
	status, err := svc.Status(c.Request.Context())
	if err != nil {
		writeLiandongToolkitError(c, err, "status")
		return
	}
	if status == nil {
		writeLiandongToolkitError(c, infraerrors.InternalServer("LDXP_TOOLKIT_INVALID_RESPONSE", "LDXP status service returned no status"), "status")
		return
	}
	c.Header("Cache-Control", "private, no-store")
	response.Success(c, status)
}

// UpdateConfig delegates configuration persistence to the existing restock
// service. The submitted merchant token is never included in the response.
func (h *LiandongToolkitHandler) UpdateConfig(c *gin.Context) {
	svc, ok := h.liandongToolkitService(c, "configuration")
	if !ok {
		return
	}
	var request service.LiandongRestockConfigurationUpdate
	present, err := bindLiandongToolkitJSON(c, &request, false)
	if err != nil {
		writeLiandongToolkitRequestError(c, "invalid LDXP configuration")
		return
	}
	if !present {
		writeLiandongToolkitRequestError(c, "LDXP configuration body is required")
		return
	}
	status, err := svc.UpdateConfiguration(c.Request.Context(), request)
	if err != nil {
		writeLiandongToolkitError(c, err, "configuration")
		return
	}
	if status == nil {
		writeLiandongToolkitError(c, infraerrors.InternalServer("LDXP_TOOLKIT_INVALID_RESPONSE", "LDXP configuration service returned no status"), "configuration")
		return
	}
	response.Success(c, status)
}

// TestConfig performs a read-only configuration and merchant connectivity
// check. It never accepts credentials in the request body.
func (h *LiandongToolkitHandler) TestConfig(c *gin.Context) {
	var request struct{}
	if _, err := bindLiandongToolkitJSON(c, &request, true); err != nil {
		writeLiandongToolkitRequestError(c, "configuration test does not accept request fields")
		return
	}
	svc, ok := h.liandongToolkitService(c, "configuration test")
	if !ok {
		return
	}
	result, err := svc.TestConfiguration(c.Request.Context())
	if err != nil {
		writeLiandongToolkitError(c, err, "configuration test")
		return
	}
	if result == nil {
		writeLiandongToolkitError(c, infraerrors.InternalServer("LDXP_TOOLKIT_INVALID_RESPONSE", "LDXP configuration test returned no result"), "configuration test")
		return
	}
	response.Success(c, result)
}

// GetGoods always requests non-proxy card goods. The caller cannot select the
// is_proxy value.
func (h *LiandongToolkitHandler) GetGoods(c *gin.Context) {
	svc, ok := h.liandongToolkitService(c, "goods")
	if !ok {
		return
	}
	result, err := svc.ListGoods(c.Request.Context())
	if err != nil {
		writeLiandongToolkitError(c, err, "goods")
		return
	}
	if result == nil {
		writeLiandongToolkitError(c, infraerrors.InternalServer("LDXP_TOOLKIT_INVALID_RESPONSE", "LDXP goods service returned no result"), "goods")
		return
	}
	response.Success(c, result)
}

// Preview computes a read-only job plan through the domain service.
func (h *LiandongToolkitHandler) Preview(c *gin.Context) {
	selectedGoods, ok := bindSelectedLiandongGoods(c)
	if !ok {
		return
	}
	svc, ok := h.liandongToolkitService(c, "preview")
	if !ok {
		return
	}
	result, err := svc.Preview(c.Request.Context(), selectedGoods)
	if err != nil {
		writeLiandongToolkitError(c, err, "preview")
		return
	}
	if result == nil {
		writeLiandongToolkitError(c, infraerrors.InternalServer("LDXP_TOOLKIT_INVALID_RESPONSE", "LDXP preview service returned no result"), "preview")
		return
	}
	response.Success(c, result)
}

// Run starts a durable manual job and returns its stable summary.
func (h *LiandongToolkitHandler) Run(c *gin.Context) {
	selectedGoods, ok := bindSelectedLiandongGoods(c)
	if !ok {
		return
	}
	svc, ok := h.liandongToolkitService(c, "run")
	if !ok {
		return
	}
	job, err := svc.StartManualJob(c.Request.Context(), selectedGoods)
	if err != nil {
		writeLiandongToolkitError(c, err, "run")
		return
	}
	if job == nil {
		writeLiandongToolkitError(c, infraerrors.InternalServer("LDXP_TOOLKIT_INVALID_RESPONSE", "LDXP job service returned no job"), "run")
		return
	}
	response.Accepted(c, job)
}

// GetJob returns a safe durable job summary without card contents.
func (h *LiandongToolkitHandler) GetJob(c *gin.Context) {
	id, ok := liandongToolkitJobID(c)
	if !ok {
		return
	}
	svc, ok := h.liandongToolkitService(c, "job status")
	if !ok {
		return
	}
	job, err := svc.GetJob(c.Request.Context(), id)
	if err != nil {
		writeLiandongToolkitError(c, err, "job status")
		return
	}
	if job == nil {
		writeLiandongToolkitError(c, infraerrors.NotFound("LDXP_JOB_NOT_FOUND", "LDXP job was not found"), "job status")
		return
	}
	response.Success(c, job)
}

// Resume resumes a durable job through the domain service.
func (h *LiandongToolkitHandler) Resume(c *gin.Context) {
	id, ok := liandongToolkitJobID(c)
	if !ok {
		return
	}
	svc, ok := h.liandongToolkitService(c, "job resume")
	if !ok {
		return
	}
	job, err := svc.ResumeJob(c.Request.Context(), id)
	if err != nil {
		writeLiandongToolkitError(c, err, "job resume")
		return
	}
	if job == nil {
		writeLiandongToolkitError(c, infraerrors.NotFound("LDXP_JOB_NOT_FOUND", "LDXP job was not found"), "job resume")
		return
	}
	response.Accepted(c, job)
}

// Export streams the domain-owned safe export as an attachment. It never
// serializes the export object or logs its contents.
func (h *LiandongToolkitHandler) Export(c *gin.Context) {
	id, ok := liandongToolkitJobID(c)
	if !ok {
		return
	}
	svc, ok := h.liandongToolkitService(c, "job export")
	if !ok {
		return
	}
	export, err := svc.ExportJob(c.Request.Context(), id)
	if err != nil {
		writeLiandongToolkitError(c, err, "job export")
		return
	}
	if export == nil || export.Reader == nil {
		writeLiandongToolkitError(c, infraerrors.InternalServer("LDXP_TOOLKIT_INVALID_EXPORT", "LDXP job export is unavailable"), "job export")
		return
	}
	defer func() { _ = export.Reader.Close() }()

	filename, err := liandongToolkitAttachmentFilename(export.Filename, id)
	if err != nil {
		writeLiandongToolkitError(c, err, "job export")
		return
	}
	contentType := liandongToolkitContentType(export.ContentType)
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, export.Reader)
}

func (h *LiandongToolkitHandler) liandongToolkitService(c *gin.Context, operation string) (service.LiandongToolkitService, bool) {
	if h == nil || h.toolkitService == nil {
		writeLiandongToolkitError(c, infraerrors.ServiceUnavailable("LDXP_TOOLKIT_SERVICE_UNAVAILABLE", "LDXP toolkit service is unavailable"), operation)
		return nil, false
	}
	return h.toolkitService, true
}

func bindLiandongToolkitJSON(c *gin.Context, destination any, strict bool) (bool, error) {
	if c == nil || c.Request == nil || c.Request.Body == nil {
		return false, nil
	}
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, liandongToolkitMaxJSONBody+1))
	if err != nil {
		return false, err
	}
	if len(raw) > liandongToolkitMaxJSONBody {
		return false, fmt.Errorf("request body exceeds limit")
	}
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return false, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if strict {
		decoder.DisallowUnknownFields()
	}
	if err := decoder.Decode(destination); err != nil {
		return false, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return true, fmt.Errorf("request body contains multiple JSON values")
		}
		return true, err
	}
	return true, nil
}

func bindSelectedLiandongGoods(c *gin.Context) ([]int64, bool) {
	var request struct {
		SelectedGoods []int64 `json:"selected_goods"`
	}
	if _, err := bindLiandongToolkitJSON(c, &request, true); err != nil {
		writeLiandongToolkitRequestError(c, "invalid selected goods")
		return nil, false
	}
	seen := make(map[int64]struct{}, len(request.SelectedGoods))
	for _, goodsID := range request.SelectedGoods {
		if goodsID <= 0 {
			writeLiandongToolkitRequestError(c, "selected goods IDs must be positive")
			return nil, false
		}
		if _, exists := seen[goodsID]; exists {
			writeLiandongToolkitRequestError(c, "selected goods IDs must be unique")
			return nil, false
		}
		seen[goodsID] = struct{}{}
	}
	return request.SelectedGoods, true
}

func liandongToolkitJobID(c *gin.Context) (string, bool) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" || len(id) > 128 || strings.ContainsAny(id, "/\\\r\n\x00") {
		writeLiandongToolkitRequestError(c, "invalid LDXP job ID")
		return "", false
	}
	for _, character := range id {
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			character == '-' || character == '_' || character == '.' || character == ':' {
			continue
		}
		writeLiandongToolkitRequestError(c, "invalid LDXP job ID")
		return "", false
	}
	return id, true
}

func liandongToolkitAttachmentFilename(value, jobID string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "ldxp-job-" + jobID + ".csv", nil
	}
	if len(value) > 128 || strings.ContainsAny(value, "/\\\r\n\x00") {
		return "", infraerrors.InternalServer("LDXP_TOOLKIT_INVALID_EXPORT", "LDXP job export filename is invalid")
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			character == '-' || character == '_' || character == '.' {
			continue
		}
		return "", infraerrors.InternalServer("LDXP_TOOLKIT_INVALID_EXPORT", "LDXP job export filename is invalid")
	}
	return value, nil
}

func liandongToolkitContentType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "text/csv":
		return "text/csv"
	case "text/plain", "text/plain; charset=utf-8":
		return "text/plain; charset=utf-8"
	case "application/octet-stream":
		return "application/octet-stream"
	default:
		return "application/octet-stream"
	}
}

func writeLiandongToolkitRequestError(c *gin.Context, message string) {
	response.ErrorFrom(c, infraerrors.BadRequest("INVALID_REQUEST", message))
}

func writeLiandongToolkitError(c *gin.Context, err error, operation string) {
	if err == nil {
		return
	}
	if mapped, ok := mapLiandongToolkitDomainError(err); ok {
		response.ErrorFrom(c, mapped)
		return
	}
	var applicationError *infraerrors.ApplicationError
	if ok := asLiandongToolkitApplicationError(err, &applicationError); ok {
		// Strip any wrapped cause before handing the error to the response/log
		// helper so upstream payloads or complete codes cannot be logged.
		response.ErrorFrom(c, infraerrors.New(int(applicationError.Code), applicationError.Reason, applicationError.Message))
		return
	}
	response.ErrorFrom(c, infraerrors.New(http.StatusBadGateway, "LDXP_TOOLKIT_OPERATION_FAILED", "LDXP toolkit "+operation+" failed"))
}

func mapLiandongToolkitDomainError(err error) (*infraerrors.ApplicationError, bool) {
	switch {
	case errors.Is(err, service.ErrLiandongJobNotFound):
		return infraerrors.NotFound("LDXP_JOB_NOT_FOUND", "LDXP job was not found"), true
	case errors.Is(err, service.ErrLiandongRunBusy):
		return infraerrors.Conflict("LDXP_RUN_BUSY", "LDXP restock run is already active"), true
	case errors.Is(err, service.ErrLiandongNeedsReconciliation):
		return infraerrors.Conflict("LDXP_NEEDS_RECONCILIATION", "LDXP job needs reconciliation before retry"), true
	case errors.Is(err, service.ErrLiandongJobNotResumable):
		return infraerrors.Conflict("LDXP_JOB_NOT_RESUMABLE", "LDXP job is not resumable"), true
	default:
		return nil, false
	}
}

func asLiandongToolkitApplicationError(err error, target **infraerrors.ApplicationError) bool {
	if err == nil || target == nil {
		return false
	}
	for current := err; current != nil; {
		if applicationError, ok := current.(*infraerrors.ApplicationError); ok {
			*target = applicationError
			return true
		}
		unwrapper, ok := current.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		current = unwrapper.Unwrap()
	}
	return false
}
