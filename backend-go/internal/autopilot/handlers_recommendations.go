package autopilot

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// ─── 渠道推荐 API（Phase 4 Item 4）───────────────────────────────────────────────
//
// GET /api/autopilot/recommendations —— 按 proxyKeyMask 或全局聚合两种粒度返回渠道推荐。
// 纯只读展示：不做任何自动切换渠道的动作，采纳与否完全由用户在渠道设置页手动调整。

// RecommendationsResponse GET /api/autopilot/recommendations 响应体。
type RecommendationsResponse struct {
	// ProxyKeyMask 请求参数回显；全局聚合粒度时为空。
	ProxyKeyMask    string                  `json:"proxyKeyMask,omitempty"`
	Recommendations []ChannelRecommendation `json:"recommendations"`
}

// RegisterRecommendationRoutes 注册渠道推荐只读 API 到给定路由组。
func RegisterRecommendationRoutes(router gin.IRouter, mgr *Manager) {
	router.GET("/autopilot/recommendations", handleRecommendations(mgr))
}

// handleRecommendations GET /api/autopilot/recommendations?proxyKeyMask=xxx&windowDays=7
// proxyKeyMask 缺省时聚合当前累积器中出现过的全部 proxyKeyMask，逐个生成推荐后合并返回。
func handleRecommendations(mgr *Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		acc := mgr.UsagePatternAccumulator()
		if acc == nil {
			c.JSON(http.StatusOK, RecommendationsResponse{Recommendations: []ChannelRecommendation{}})
			return
		}

		windowDays := defaultRecommendationOptions().WindowDays
		if v := c.Query("windowDays"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				windowDays = n
			}
		}
		opts := RecommendationOptions{WindowDays: windowDays}

		lookup := buildChannelScoreLookup(mgr)

		proxyKeyMask := c.Query("proxyKeyMask")
		var recommendations []ChannelRecommendation
		if proxyKeyMask != "" {
			recommendations = BuildRecommendationsForUser(proxyKeyMask, acc, lookup, opts)
		} else {
			for _, mask := range acc.AllProxyKeyMasks() {
				recommendations = append(recommendations, BuildRecommendationsForUser(mask, acc, lookup, opts)...)
			}
		}
		if recommendations == nil {
			recommendations = []ChannelRecommendation{}
		}

		c.JSON(http.StatusOK, RecommendationsResponse{
			ProxyKeyMask:    proxyKeyMask,
			Recommendations: recommendations,
		})
	}
}

// buildChannelScoreLookup 遍历所有渠道的 endpoint 画像，聚合成 ChannelProfile，
// 结合 ModelProfileStore 的域优势数据，为每个渠道计算 8 个 TaskDomain 的综合分。
// 同时标记渠道健康性（dead/misconfigured 视为不健康，不参与推荐）。
func buildChannelScoreLookup(mgr *Manager) channelScoreLookup {
	lookup := channelScoreLookup{
		scores:  make(map[string]map[TaskDomain]float64),
		healthy: make(map[string]bool),
	}

	profileStore := mgr.ProfileStore()
	if profileStore == nil {
		return lookup
	}

	allProfiles := profileStore.ListAll()
	grouped := make(map[string][]KeyEndpointProfile)
	for _, p := range allProfiles {
		if p == nil {
			continue
		}
		grouped[p.ChannelUID] = append(grouped[p.ChannelUID], *p)
	}

	modelProfileStore := mgr.ModelProfileStore()

	for channelUID, endpoints := range grouped {
		if len(endpoints) == 0 {
			continue
		}
		cp := AggregateChannelProfile(channelUID, endpoints[0].ChannelID, endpoints[0].ChannelKind, endpoints)

		lookup.healthy[channelUID] = cp.HealthState != HealthStateDead && cp.HealthState != HealthStateMisconfigured

		var modelProfiles []ModelProfile
		if modelProfileStore != nil {
			modelProfiles = modelProfileStore.ListByChannel(channelUID)
		}

		domainScores := make(map[TaskDomain]float64, len(AllTaskDomains()))
		for _, domain := range AllTaskDomains() {
			domainScores[domain] = ChannelDomainScore(cp, domain, modelProfiles)
		}
		lookup.scores[channelUID] = domainScores
	}

	return lookup
}
