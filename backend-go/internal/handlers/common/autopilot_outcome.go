package common

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/BenedictKing/ccx/internal/autopilot"
	"github.com/BenedictKing/ccx/internal/scheduler"
	"github.com/gin-gonic/gin"
)

const autopilotFirstByteAtKey = "ccx.autopilot.first_byte_at"

var routingOutcomeRecorderHook func(traceUID string, outcome autopilot.RoutingOutcome)

// SetRoutingOutcomeRecorderHook 注入请求终态记录器；nil 时保持原有请求路径。
func SetRoutingOutcomeRecorderHook(hook func(traceUID string, outcome autopilot.RoutingOutcome)) {
	routingOutcomeRecorderHook = hook
}

func resetAutopilotAttemptTelemetry(c *gin.Context) {
	if c != nil {
		c.Set(autopilotFirstByteAtKey, time.Time{})
	}
}

func recordAutopilotFirstByte(c *gin.Context, at time.Time) {
	if c != nil {
		c.Set(autopilotFirstByteAtKey, at)
	}
}

func autopilotFirstByteLatency(c *gin.Context, requestStartedAt time.Time) int64 {
	if c == nil {
		return 0
	}
	value, ok := c.Get(autopilotFirstByteAtKey)
	if !ok {
		return 0
	}
	at, ok := value.(time.Time)
	if !ok || at.IsZero() || at.Before(requestStartedAt) {
		return 0
	}
	return at.Sub(requestStartedAt).Milliseconds()
}

func buildRoutingOutcome(
	c *gin.Context,
	selection *scheduler.SelectionResult,
	result MultiChannelAttemptResult,
	terminal bool,
	channelFallback bool,
	requestStartedAt time.Time,
	duration time.Duration,
) autopilot.RoutingOutcome {
	statusCode := 0
	if result.FailoverError != nil {
		statusCode = result.FailoverError.Status
	}
	if terminal && c != nil && c.Writer.Status() > 0 {
		statusCode = c.Writer.Status()
	}

	outcomeName := "attempt_failed"
	success := terminal && result.SuccessKey != ""
	switch {
	case errors.Is(result.LastError, context.Canceled):
		outcomeName = "cancelled"
	case success:
		outcomeName = "success"
	case terminal && !result.Handled:
		outcomeName = "exhausted"
	case terminal:
		outcomeName = "upstream_error"
	}
	if success && statusCode == 0 {
		statusCode = http.StatusOK
	}

	return autopilot.RoutingOutcome{
		Terminal:           terminal,
		Success:            success,
		ChannelFallback:    terminal && channelFallback,
		StatusCode:         statusCode,
		RequestDurationMs:  duration.Milliseconds(),
		FirstByteLatencyMs: autopilotFirstByteLatency(c, requestStartedAt),
		Outcome:            outcomeName,
		CompletedAt:        time.Now().UTC(),
	}
}

func notifyRoutingOutcome(selection *scheduler.SelectionResult, outcome autopilot.RoutingOutcome) {
	if routingOutcomeRecorderHook == nil || selection == nil || selection.AutopilotTraceUID == "" {
		return
	}
	routingOutcomeRecorderHook(selection.AutopilotTraceUID, outcome)
}
