package admin

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const liandongToolkitMaxJSONBody = 4 << 20

type LiandongToolkitHandler struct {
	browser *service.LiandongRestockService
}

func NewLiandongToolkitHandler(browser *service.LiandongRestockService) *LiandongToolkitHandler {
	return &LiandongToolkitHandler{browser: browser}
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
