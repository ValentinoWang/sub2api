package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type liandongToolkitHandlerServiceStub struct {
	status             *service.LiandongRestockStatus
	configuration      *service.LiandongRestockStatus
	connectivity       *service.LiandongToolkitConnectivityResult
	goods              *service.LiandongToolkitGoodsResult
	preview            *service.LiandongRestockPreview
	job                *service.LiandongRestockJobSummary
	export             *service.LiandongRestockJobExport
	startJobErr        error
	getJobErr          error
	resumeJobErr       error
	exportErr          error
	configurationInput service.LiandongRestockConfigurationUpdate
	selectedPreview    []int64
	selectedRun        []int64
	requestedJobID     string
	resumedJobID       string
}

func (s *liandongToolkitHandlerServiceStub) Status(context.Context) (*service.LiandongRestockStatus, error) {
	return s.status, nil
}

func (s *liandongToolkitHandlerServiceStub) UpdateConfiguration(_ context.Context, input service.LiandongRestockConfigurationUpdate) (*service.LiandongRestockStatus, error) {
	s.configurationInput = input
	return s.configuration, nil
}

func (s *liandongToolkitHandlerServiceStub) TestConfiguration(context.Context) (*service.LiandongToolkitConnectivityResult, error) {
	return s.connectivity, nil
}

func (s *liandongToolkitHandlerServiceStub) ListGoods(context.Context) (*service.LiandongToolkitGoodsResult, error) {
	return s.goods, nil
}

func (s *liandongToolkitHandlerServiceStub) Preview(_ context.Context, selected []int64) (*service.LiandongRestockPreview, error) {
	s.selectedPreview = append([]int64(nil), selected...)
	return s.preview, nil
}

func (s *liandongToolkitHandlerServiceStub) StartManualJob(_ context.Context, selected []int64) (*service.LiandongRestockJobSummary, error) {
	s.selectedRun = append([]int64(nil), selected...)
	return s.job, s.startJobErr
}

func (s *liandongToolkitHandlerServiceStub) GetJob(_ context.Context, id string) (*service.LiandongRestockJobSummary, error) {
	s.requestedJobID = id
	return s.job, s.getJobErr
}

func (s *liandongToolkitHandlerServiceStub) ResumeJob(_ context.Context, id string) (*service.LiandongRestockJobSummary, error) {
	s.resumedJobID = id
	return s.job, s.resumeJobErr
}

func (s *liandongToolkitHandlerServiceStub) ExportJob(context.Context, string) (*service.LiandongRestockJobExport, error) {
	return s.export, s.exportErr
}

func newLiandongToolkitHandlerRouter(h *LiandongToolkitHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/installation", h.GetInstallation)
	router.POST("/installation", h.Install)
	router.GET("/status", h.GetStatus)
	router.PUT("/config", h.UpdateConfig)
	router.POST("/config/test", h.TestConfig)
	router.GET("/goods", h.GetGoods)
	router.POST("/jobs/preview", h.Preview)
	router.POST("/jobs/run", h.Run)
	router.GET("/jobs/:id", h.GetJob)
	router.POST("/jobs/:id/resume", h.Resume)
	router.GET("/jobs/:id/export", h.Export)
	return router
}

func TestLiandongToolkitHandlerReturnsUnavailableRuntimeWithoutPretendingInstalled(t *testing.T) {
	router := newLiandongToolkitHandlerRouter(NewLiandongToolkitHandler(nil, nil))

	getRecorder := httptest.NewRecorder()
	router.ServeHTTP(getRecorder, httptest.NewRequest(http.MethodGet, "/installation", nil))
	require.Equal(t, http.StatusOK, getRecorder.Code)
	require.Contains(t, getRecorder.Body.String(), `"ready":false`)
	require.Contains(t, getRecorder.Body.String(), "runtime is unavailable")

	postRecorder := httptest.NewRecorder()
	router.ServeHTTP(postRecorder, httptest.NewRequest(http.MethodPost, "/installation", nil))
	require.Equal(t, http.StatusServiceUnavailable, postRecorder.Code)
	require.Contains(t, postRecorder.Body.String(), "LDXP_TOOLKIT_UNAVAILABLE")
}

func TestLiandongToolkitHandlerRejectsUserProvidedInstallerPath(t *testing.T) {
	dataDir := t.TempDir()
	assetPath := filepath.Join(t.TempDir(), "ldxp-toolkit")
	require.NoError(t, osWriteFileForLiandongToolkitTest(assetPath, []byte("safe asset"), 0o600))
	runtimeService, err := service.NewLiandongToolkitRuntime(service.LiandongToolkitRuntimeConfig{
		DataDir: dataDir, AssetPath: assetPath,
	})
	require.NoError(t, err)
	router := newLiandongToolkitHandlerRouter(NewLiandongToolkitHandler(nil, runtimeService))

	request := httptest.NewRequest(http.MethodPost, "/installation", strings.NewReader(`{"path":"/tmp/evil","command":"rm -rf /"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "INVALID_REQUEST")
	_, statErr := os.Stat(service.DefaultLiandongToolkitProgramPath(dataDir))
	require.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestLiandongToolkitHandlerSerializesConfigWithoutMerchantToken(t *testing.T) {
	const token = "merchant-token-that-must-not-return"
	stub := &liandongToolkitHandlerServiceStub{
		configuration: &service.LiandongRestockStatus{
			IntegrationMode:         "sales_channel",
			PaymentReadiness:        "NOT_READY",
			Configured:              true,
			MerchantTokenConfigured: true,
			CodeSecretConfigured:    true,
		},
	}
	router := newLiandongToolkitHandlerRouter(NewLiandongToolkitHandler(stub, nil))
	body := `{"merchant_token":"` + token + `","generate_code_secret":false,"products":[{"goods_id":42,"cny_amount":20,"usd_credit":2.78,"target_stock":50000,"enabled":true}]}`
	request := httptest.NewRequest(http.MethodPut, "/config", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, token, stub.configurationInput.MerchantToken)
	require.Len(t, stub.configurationInput.Products, 1)
	require.Equal(t, int64(42), stub.configurationInput.Products[0].GoodsID)
	require.Equal(t, 50000, stub.configurationInput.Products[0].TargetStock)
	require.NotContains(t, recorder.Body.String(), token)
}

func TestLiandongToolkitHandlerUsesFixedGoodsQueryAndCoreJobMethods(t *testing.T) {
	job := &service.LiandongRestockJobSummary{JobID: "job-1", Status: "completed"}
	stub := &liandongToolkitHandlerServiceStub{
		connectivity: &service.LiandongToolkitConnectivityResult{Configured: true, Reachable: true, ReadOnly: true},
		goods:        &service.LiandongToolkitGoodsResult{Goods: []service.LiandongToolkitGood{{GoodsID: 42, Name: "balance", Type: "card"}}},
		preview:      &service.LiandongRestockPreview{},
		job:          job,
	}
	router := newLiandongToolkitHandlerRouter(NewLiandongToolkitHandler(stub, nil))

	goodsRecorder := httptest.NewRecorder()
	router.ServeHTTP(goodsRecorder, httptest.NewRequest(http.MethodGet, "/goods?is_proxy=1", nil))
	require.Equal(t, http.StatusOK, goodsRecorder.Code)

	previewRequest := httptest.NewRequest(http.MethodPost, "/jobs/preview", strings.NewReader(`{"selected_goods":[42]}`))
	previewRequest.Header.Set("Content-Type", "application/json")
	previewRecorder := httptest.NewRecorder()
	router.ServeHTTP(previewRecorder, previewRequest)
	require.Equal(t, http.StatusOK, previewRecorder.Code)
	require.Equal(t, []int64{42}, stub.selectedPreview)

	runRequest := httptest.NewRequest(http.MethodPost, "/jobs/run", strings.NewReader(`{"selected_goods":[42]}`))
	runRequest.Header.Set("Content-Type", "application/json")
	runRecorder := httptest.NewRecorder()
	router.ServeHTTP(runRecorder, runRequest)
	require.Equal(t, http.StatusAccepted, runRecorder.Code)
	require.Equal(t, []int64{42}, stub.selectedRun)

	jobRecorder := httptest.NewRecorder()
	router.ServeHTTP(jobRecorder, httptest.NewRequest(http.MethodGet, "/jobs/job-1", nil))
	require.Equal(t, http.StatusOK, jobRecorder.Code)
	require.Equal(t, "job-1", stub.requestedJobID)

	resumeRecorder := httptest.NewRecorder()
	router.ServeHTTP(resumeRecorder, httptest.NewRequest(http.MethodPost, "/jobs/job-1/resume", nil))
	require.Equal(t, http.StatusAccepted, resumeRecorder.Code)
	require.Equal(t, "job-1", stub.resumedJobID)
}

func TestLiandongToolkitHandlerMapsDomainErrorsToStableResponses(t *testing.T) {
	const sensitiveDetail = "merchant-token-and-code-material"

	tests := []struct {
		name      string
		status    int
		reason    string
		method    string
		path      string
		body      string
		configure func(*liandongToolkitHandlerServiceStub, error)
	}{
		{
			name:   "job not found",
			status: http.StatusNotFound,
			reason: "LDXP_JOB_NOT_FOUND",
			method: http.MethodGet,
			path:   "/jobs/job-1",
			configure: func(stub *liandongToolkitHandlerServiceStub, err error) {
				stub.getJobErr = err
			},
		},
		{
			name:   "run busy",
			status: http.StatusConflict,
			reason: "LDXP_RUN_BUSY",
			method: http.MethodPost,
			path:   "/jobs/run",
			body:   `{"selected_goods":[42]}`,
			configure: func(stub *liandongToolkitHandlerServiceStub, err error) {
				stub.startJobErr = err
			},
		},
		{
			name:   "needs reconciliation",
			status: http.StatusConflict,
			reason: "LDXP_NEEDS_RECONCILIATION",
			method: http.MethodPost,
			path:   "/jobs/job-1/resume",
			configure: func(stub *liandongToolkitHandlerServiceStub, err error) {
				stub.resumeJobErr = err
			},
		},
		{
			name:   "job not resumable",
			status: http.StatusConflict,
			reason: "LDXP_JOB_NOT_RESUMABLE",
			method: http.MethodPost,
			path:   "/jobs/job-1/resume",
			configure: func(stub *liandongToolkitHandlerServiceStub, err error) {
				stub.resumeJobErr = err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stub := &liandongToolkitHandlerServiceStub{
				job: &service.LiandongRestockJobSummary{JobID: "job-1", Status: "running"},
			}
			test.configure(stub, fmt.Errorf("%s: %w", sensitiveDetail, domainErrorForLiandongToolkitTest(test.reason)))
			router := newLiandongToolkitHandlerRouter(NewLiandongToolkitHandler(stub, nil))

			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			if test.body != "" {
				request.Header.Set("Content-Type", "application/json")
			}
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)

			require.Equal(t, test.status, recorder.Code)
			var payload struct {
				Code    int    `json:"code"`
				Reason  string `json:"reason"`
				Message string `json:"message"`
			}
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
			require.Equal(t, test.status, payload.Code)
			require.Equal(t, test.reason, payload.Reason)
			require.NotContains(t, recorder.Body.String(), sensitiveDetail)
			require.NotContains(t, recorder.Body.String(), "Liandong restock")
		})
	}
}

func domainErrorForLiandongToolkitTest(reason string) error {
	switch reason {
	case "LDXP_JOB_NOT_FOUND":
		return service.ErrLiandongJobNotFound
	case "LDXP_RUN_BUSY":
		return service.ErrLiandongRunBusy
	case "LDXP_NEEDS_RECONCILIATION":
		return service.ErrLiandongNeedsReconciliation
	case "LDXP_JOB_NOT_RESUMABLE":
		return service.ErrLiandongJobNotResumable
	default:
		panic("unknown LDXP test reason: " + reason)
	}
}

func TestLiandongToolkitHandlerExportsOnlyAsAttachment(t *testing.T) {
	const completeCode = "abcdefghijklmnopqrst"
	stub := &liandongToolkitHandlerServiceStub{
		export: &service.LiandongRestockJobExport{
			Filename:    "ldxp-job-1.txt",
			ContentType: "text/plain; charset=utf-8",
			Reader:      io.NopCloser(strings.NewReader(completeCode + "\n")),
		},
	}
	router := newLiandongToolkitHandlerRouter(NewLiandongToolkitHandler(stub, nil))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/jobs/job-1/export", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Header().Get("Content-Disposition"), "attachment")
	require.Equal(t, "text/plain; charset=utf-8", recorder.Header().Get("Content-Type"))
	require.Equal(t, completeCode+"\n", recorder.Body.String())
	require.NotContains(t, recorder.Body.String(), `"data"`)
}

func TestLiandongToolkitHandlerDoesNotEchoSecretsInStatusOrJobJSON(t *testing.T) {
	const token = "merchant-token-never-in-json"
	const completeCode = "abcdefghijklmnopqrst"
	stub := &liandongToolkitHandlerServiceStub{
		status: &service.LiandongRestockStatus{
			Configured:              true,
			MerchantTokenConfigured: true,
			CodeSecretConfigured:    true,
		},
		job: &service.LiandongRestockJobSummary{
			JobID:  "job-1",
			Status: "completed",
		},
	}
	router := newLiandongToolkitHandlerRouter(NewLiandongToolkitHandler(stub, nil))
	for _, path := range []string{"/status", "/jobs/job-1"} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusOK, recorder.Code, path)
		require.NotContains(t, recorder.Body.String(), token, path)
		require.NotContains(t, recorder.Body.String(), completeCode, path)
	}
}

func osWriteFileForLiandongToolkitTest(path string, content []byte, mode uint32) error {
	return os.WriteFile(path, content, os.FileMode(mode))
}
