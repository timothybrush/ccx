package handlers

import (
	"log"
	"net/http"
	"time"

	"github.com/BenedictKing/ccx/internal/metrics"
	"github.com/gin-gonic/gin"
)

// CostReportDeps 成本报表 handler 的依赖
type CostReportDeps struct {
	// MetricsManagers 按 apiType 查找对应的 MetricsManager（均共享同一 SQLiteStore）
	MetricsManagers map[string]*metrics.MetricsManager
}

// costReportRow 成本报表响应行
type costReportRow struct {
	GroupKey            string  `json:"groupKey"`
	TotalRequests       int64   `json:"totalRequests"`
	SuccessCount        int64   `json:"successCount"`
	InputTokens         int64   `json:"inputTokens"`
	OutputTokens        int64   `json:"outputTokens"`
	CacheCreationTokens int64   `json:"cacheCreationTokens"`
	CacheReadTokens     int64   `json:"cacheReadTokens"`
	ListCostUSD         float64 `json:"listCostUSD"`
	EffectiveCostUSD    float64 `json:"effectiveCostUSD"`
}

// GetCostReport 返回按维度聚合的成本报表。
// GET /api/reports/cost?groupBy=user|model|key&duration=7d&type=messages
//
// groupBy 默认 "user"（按 proxyKeyMask）；duration 默认 7d；type 默认 messages。
// ListCostUSD 为官方定价，EffectiveCostUSD 为应用 EffectiveCostMultiplier 后的实际成本。
// 当 multiplier 未配置时两者相同。
func GetCostReport(deps *CostReportDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		groupBy := c.DefaultQuery("groupBy", "user")
		if groupBy != "user" && groupBy != "model" && groupBy != "key" {
			groupBy = "user"
		}

		apiType := c.DefaultQuery("type", "messages")
		mgr, ok := deps.MetricsManagers[apiType]
		if !ok || mgr == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的 apiType: " + apiType})
			return
		}
		store := mgr.GetPersistenceStore()
		if store == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "成本报表需要启用 SQLite 持久化存储"})
			return
		}

		duration := parseCostReportDuration(c.DefaultQuery("duration", "7d"))
		since := time.Now().Add(-duration)

		rows, err := store.QueryCostReport(apiType, since, groupBy)
		if err != nil {
			log.Printf("[CostReport-Query] 查询失败: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查询成本报表失败"})
			return
		}

		result := make([]costReportRow, 0, len(rows))
		for _, row := range rows {
			cr := costReportRow{
				GroupKey:            row.GroupKey,
				TotalRequests:       row.TotalRequests,
				SuccessCount:        row.SuccessCount,
				InputTokens:         row.InputTokens,
				OutputTokens:        row.OutputTokens,
				CacheCreationTokens: row.CacheCreationTokens,
				CacheReadTokens:     row.CacheReadTokens,
			}

			// 按模型明细计算成本（需要逐模型定价）
			modelBreakdowns, err := store.QueryModelCostBreakdown(apiType, since, groupBy, row.GroupKey)
			if err != nil {
				log.Printf("[CostReport-CostCalc] 查询模型明细失败 (groupKey=%s): %v", row.GroupKey, err)
				// 成本计算失败不阻断报表，留零
				result = append(result, cr)
				continue
			}

			for _, mb := range modelBreakdowns {
				listCost := metrics.CalculateTokenCostUSD(mb.Model, mb.InputTokens, mb.OutputTokens, mb.CacheCreationTokens, mb.CacheReadTokens)
				cr.ListCostUSD += listCost
				// EffectiveCostMultiplier 由 autopilot 层管理；报表 handler 不反向依赖 autopilot，
				// 暂时 EffectiveCostUSD == ListCostUSD。后续接入 autopilot.KeyEndpointProfile 后，
				// 调用 metrics.ApplyEffectiveCostMultiplier(listCost, multiplier) 即可。
				cr.EffectiveCostUSD += listCost
			}

			result = append(result, cr)
		}

		c.JSON(http.StatusOK, gin.H{
			"groupBy":  groupBy,
			"apiType":  apiType,
			"duration": c.DefaultQuery("duration", "7d"),
			"rows":     result,
		})
	}
}

// parseCostReportDuration 解析成本报表的时间范围参数（简化版，支持 7d/30d/90d/365d）
func parseCostReportDuration(s string) time.Duration {
	switch s {
	case "1h":
		return time.Hour
	case "6h":
		return 6 * time.Hour
	case "24h", "1d":
		return 24 * time.Hour
	case "7d", "":
		return 7 * 24 * time.Hour
	case "30d":
		return 30 * 24 * time.Hour
	case "90d":
		return 90 * 24 * time.Hour
	case "365d":
		return 365 * 24 * time.Hour
	default:
		// 尝试标准 Go duration
		d, err := time.ParseDuration(s)
		if err != nil {
			return 7 * 24 * time.Hour
		}
		return d
	}
}
