package autopilot

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// ── 路由注册 ──

// RegisterTraceRoutes 注册路由追踪只读 API 到给定路由组。
// 路由前缀由调用方控制（例如 router.Group("/traces")）。
func RegisterTraceRoutes(router gin.IRouter, store *TraceStore) {
	group := router.Group("/traces")
	{
		group.GET("", handleListTraces(store))
		group.GET("/stats", handleTraceStats(store))
	}
}

// ── 响应类型 ──

// TraceListResponse GET /api/autopilot/traces 返回结构。
type TraceListResponse struct {
	Traces []*RoutingDecisionTrace `json:"traces"`
	Total  int                     `json:"total"`
}

// ── Handler 实现 ──

// handleListTraces GET /api/autopilot/traces
// 查询参数：
//   - limit=N         返回最近 N 条（默认 50，最大 200）
//   - mismatch=true   只返回 shadow 与实际不一致的记录
//
// 脱敏输出：不含 promptHash、key 已掩码、不含消息正文。
func handleListTraces(store *TraceStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit := 50
		if limitStr := c.Query("limit"); limitStr != "" {
			if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
				limit = parsed
			}
		}
		if limit > 200 {
			limit = 200
		}

		mismatchOnly := c.Query("mismatch") == "true"

		traces := store.ListRecentWithFilter(limit, mismatchOnly)
		c.JSON(http.StatusOK, TraceListResponse{
			Traces: traces,
			Total:  len(traces),
		})
	}
}

// handleTraceStats GET /api/autopilot/traces/stats
// 返回追踪统计：总数、不一致率、各 TaskClass 分布。
func handleTraceStats(store *TraceStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		stats := store.GetStats()
		c.JSON(http.StatusOK, stats)
	}
}
