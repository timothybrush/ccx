package autopilot

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// ── 路由注册 ──

// RegisterAdvisorRoutes 注册 advisor 管理 API 到给定路由组。
// 路由前缀由调用方控制（例如 router.Group("/advisor")）。
func RegisterAdvisorRoutes(router gin.IRouter, store *AdvisorDecisionStore) {
	group := router.Group("/advisor")
	{
		group.GET("/stats", handleAdvisorStats(store))
		group.GET("/decisions", handleAdvisorDecisions(store))
	}
}

// ── Handler 实现 ──

// handleAdvisorStats GET /api/autopilot/advisor/stats
// 返回 advisor 决策统计汇总。
func handleAdvisorStats(store *AdvisorDecisionStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		stats := store.GetStats()
		c.JSON(http.StatusOK, stats)
	}
}

// handleAdvisorDecisions GET /api/autopilot/advisor/decisions
// 查询参数：
//   - limit=N  返回最近 N 条（默认 50，最大 200）
//
// 脱敏输出：不含 promptHash、不含消息正文。
func handleAdvisorDecisions(store *AdvisorDecisionStore) gin.HandlerFunc {
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

		decisions := store.ListRecent(limit)
		c.JSON(http.StatusOK, gin.H{
			"decisions": decisions,
			"total":     len(decisions),
		})
	}
}
