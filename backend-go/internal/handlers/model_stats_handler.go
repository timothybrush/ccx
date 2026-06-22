package handlers

import (
	"time"

	"github.com/BenedictKing/ccx/internal/metrics"
	"github.com/gin-gonic/gin"
)

// GetModelStatsHistory 获取按模型分组的历史统计
func GetModelStatsHistory(metricsManager *metrics.MetricsManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		durationStr := c.DefaultQuery("duration", "24h")

		var duration time.Duration
		var err error

		duration, err = parseExtendedDuration(durationStr)
		if err != nil {
			c.JSON(400, gin.H{"error": "Invalid duration parameter. Use: 1h, 6h, 24h, or today"})
			return
		}

		// 模型统计基于内存 requestHistory，只保留 24 小时数据
		maxDuration := 24 * time.Hour
		if duration > maxDuration {
			c.JSON(400, gin.H{"error": "Model stats only support up to 24 hours. Use: 1h, 6h, 24h, or today"})
			return
		}

		// 根据 duration 自动选择聚合粒度
		interval := selectIntervalForDuration(c.Query("interval"), duration)

		models := metricsManager.GetModelStatsHistory(duration, interval)

		c.JSON(200, gin.H{
			"models":   models,
			"duration": durationStr,
			"interval": interval.String(),
		})
	}
}
