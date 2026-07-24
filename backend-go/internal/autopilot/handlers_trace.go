package autopilot

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

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
		group.GET("/:traceUid", handleGetTraceDetail(store))
	}
}

// ── 响应类型 ──

// TraceListResponse GET /api/autopilot/traces 返回结构。
type TraceListResponse struct {
	Traces  []TraceSummary `json:"traces"`
	Total   int            `json:"total"`
	Partial bool           `json:"partial,omitempty"` // 坏行被跳过时为 true
	HasMore bool           `json:"hasMore"`
}

// TraceDetailResponse GET /api/autopilot/traces/:traceUid 返回结构。
type TraceDetailResponse struct {
	Trace *TraceDetailV2 `json:"trace"`
}

// TraceStatsResponse GET /api/autopilot/traces/stats 返回结构。
type TraceStatsResponse struct {
	TotalCount      int            `json:"totalCount"`
	ComparedCount   int            `json:"comparedCount"`
	MatchedCount    int            `json:"matchedCount"`
	MismatchCount   int            `json:"mismatchCount"`
	UncomparedCount int            `json:"uncomparedCount"`
	SuccessCount    int            `json:"successCount"`
	FailOpenCount   int            `json:"failOpenCount"`
	MismatchRate    float64        `json:"mismatchRate"`
	TaskClassDist   map[string]int `json:"taskClassDist"`
	ModeDist        map[string]int `json:"modeDist"`
}

// ── 查询超时 ──

const traceQueryDeadline = 2 * time.Second

// ── Handler 实现 ──

// handleListTraces GET /api/autopilot/traces
// 查询参数：
//   - limit=N         返回最近 N 条（默认 50，最大 200）
//   - mismatch=true   只返回 shadow 与实际不一致的记录
//   - release=releaseId 过滤发布批次
//   - cohort=treatment/control/bypass 过滤分桶
//   - mode=shadow/assist/auto/active 过滤模式
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
		releaseFilter := c.Query("release")
		cohortFilter := c.Query("cohort")
		modeFilter := c.Query("mode")

		traces, partial, hasMore, err := store.ListTraceSummary(limit, mismatchOnly, releaseFilter, cohortFilter, modeFilter)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "存储暂不可用", "retryable": true})
			return
		}

		c.JSON(http.StatusOK, TraceListResponse{
			Traces:  traces,
			Total:   len(traces),
			Partial: partial,
			HasMore: hasMore,
		})
	}
}

// handleGetTraceDetail GET /api/autopilot/traces/:traceUid
// 按 UID 查询单条 trace 的安全详情。
// 记录不存在、已过期或未被持久化时返回 404。
func handleGetTraceDetail(store *TraceStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		traceUID := c.Param("traceUid")
		if traceUID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 traceUid 参数"})
			return
		}

		detail, err := store.GetTraceDetail(traceUID)
		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusNotFound, gin.H{"error": "记录不存在、已过期或未被采样", "traceUid": traceUID})
				return
			}
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "存储暂不可用", "retryable": true})
			return
		}

		c.JSON(http.StatusOK, TraceDetailResponse{Trace: detail})
	}
}

// handleTraceStats GET /api/autopilot/traces/stats
// 返回追踪统计：总数、三态比较率、各 TaskClass/Mode 分布。
func handleTraceStats(store *TraceStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		stats := store.GetV2Stats()
		c.JSON(http.StatusOK, stats)
	}
}
