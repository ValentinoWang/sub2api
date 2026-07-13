package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// openAIFirstTokenWatchdog owns the upstream request context until a
// client-visible Responses event is ready to be sent. It intentionally does
// not treat response.created or response.in_progress as output.
type openAIFirstTokenWatchdog struct {
	timer     *time.Timer
	cancel    context.CancelFunc
	timeoutCh chan struct{}
	timeout   time.Duration
	timedOut  atomic.Bool
	stopped   atomic.Bool
}

func newOpenAIFirstTokenWatchdog(ctx context.Context, timeout time.Duration) (context.Context, *openAIFirstTokenWatchdog) {
	if ctx == nil {
		ctx = context.Background()
	}
	if timeout <= 0 {
		return ctx, nil
	}

	upstreamCtx, cancel := context.WithCancel(ctx)
	watchdog := &openAIFirstTokenWatchdog{
		cancel:    cancel,
		timeoutCh: make(chan struct{}),
		timeout:   timeout,
	}
	watchdog.timer = time.AfterFunc(timeout, func() {
		watchdog.timedOut.Store(true)
		close(watchdog.timeoutCh)
		cancel()
	})
	return upstreamCtx, watchdog
}

func (w *openAIFirstTokenWatchdog) Timeout() <-chan struct{} {
	if w == nil {
		return nil
	}
	return w.timeoutCh
}

func (w *openAIFirstTokenWatchdog) TimedOut() bool {
	return w != nil && w.timedOut.Load()
}

func (w *openAIFirstTokenWatchdog) Duration() time.Duration {
	if w == nil {
		return 0
	}
	return w.timeout
}

// Stop returns false when the deadline already fired. Once stopped, later
// calls remain successful unless the timer had already won the race.
func (w *openAIFirstTokenWatchdog) Stop() bool {
	if w == nil {
		return true
	}
	if !w.stopped.CompareAndSwap(false, true) {
		return !w.timedOut.Load()
	}
	return w.timer.Stop()
}

func (w *openAIFirstTokenWatchdog) Close() {
	if w == nil {
		return
	}
	_ = w.Stop()
	w.cancel()
}

func (s *OpenAIGatewayService) openAIFirstTokenTimeout() time.Duration {
	if s == nil || s.cfg == nil || s.cfg.Gateway.OpenAIFirstTokenTimeout <= 0 {
		return 0
	}
	return time.Duration(s.cfg.Gateway.OpenAIFirstTokenTimeout) * time.Second
}

func (s *OpenAIGatewayService) newOpenAIFirstTokenTimeoutFailoverError(
	c *gin.Context,
	account *Account,
	requestedModel string,
	upstreamRequestID string,
	timeout time.Duration,
) *UpstreamFailoverError {
	message := fmt.Sprintf("OpenAI stream produced no client-visible output within %s", timeout)
	fields := []zap.Field{
		zap.String("model", requestedModel),
		zap.Duration("timeout", timeout),
		zap.String("upstream_request_id", upstreamRequestID),
	}
	if account != nil {
		fields = append(fields, zap.Int64("account_id", account.ID), zap.String("account_name", account.Name))
	}
	logger.L().Warn("openai.first_token_timeout", fields...)
	message = s.recordOpenAIStreamUpstreamError(c, account, false, upstreamRequestID, "first_token_timeout", nil, message)
	body, _ := json.Marshal(gin.H{
		"error": gin.H{
			"type":    "upstream_error",
			"message": message,
		},
	})
	return &UpstreamFailoverError{
		StatusCode:   http.StatusGatewayTimeout,
		ResponseBody: body,
	}
}
