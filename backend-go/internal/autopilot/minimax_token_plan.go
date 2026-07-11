package autopilot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const miniMaxTokenPlanUsageTTL = 5 * time.Minute

// MiniMaxTokenPlanModelUsage 描述单个模型的当前窗口与周窗口额度。
type MiniMaxTokenPlanModelUsage struct {
	ModelName                       string  `json:"modelName"`
	CurrentIntervalUsageCount       int64   `json:"currentIntervalUsageCount"`
	CurrentIntervalTotalCount       int64   `json:"currentIntervalTotalCount"`
	CurrentIntervalRemainingPercent float64 `json:"currentIntervalRemainingPercent"`
	CurrentWeeklyUsageCount         int64   `json:"currentWeeklyUsageCount"`
	CurrentWeeklyTotalCount         int64   `json:"currentWeeklyTotalCount"`
	CurrentWeeklyRemainingPercent   float64 `json:"currentWeeklyRemainingPercent"`
	RemainsTimeMs                   int64   `json:"remainsTimeMs"`
	WeeklyStartTime                 string  `json:"weeklyStartTime,omitempty"`
	WeeklyEndTime                   string  `json:"weeklyEndTime,omitempty"`
}

// MiniMaxTokenPlanUsage 是 MiniMax Token Plan 的 provider 专用用量快照。
type MiniMaxTokenPlanUsage struct {
	Models    []MiniMaxTokenPlanModelUsage `json:"models"`
	FetchedAt time.Time                    `json:"fetchedAt"`
	SourceURL string                       `json:"sourceUrl"`
}

type miniMaxTokenPlanResponse struct {
	ModelRemains []struct {
		ModelName                       string   `json:"model_name"`
		CurrentIntervalUsageCount       int64    `json:"current_interval_usage_count"`
		CurrentIntervalTotalCount       int64    `json:"current_interval_total_count"`
		CurrentIntervalRemainingPercent *float64 `json:"current_interval_remaining_percent"`
		CurrentWeeklyUsageCount         int64    `json:"current_weekly_usage_count"`
		CurrentWeeklyTotalCount         int64    `json:"current_weekly_total_count"`
		CurrentWeeklyRemainingPercent   *float64 `json:"current_weekly_remaining_percent"`
		RemainsTime                     int64    `json:"remains_time"`
		WeeklyStartTime                 string   `json:"weekly_start_time"`
		WeeklyEndTime                   string   `json:"weekly_end_time"`
	} `json:"model_remains"`
	BaseResp struct {
		StatusCode int    `json:"status_code"`
		StatusMsg  string `json:"status_msg"`
	} `json:"base_resp"`
}

// MiniMaxTokenPlanFetcher 查询官方 Token Plan 用量接口。
type MiniMaxTokenPlanFetcher struct {
	HTTPClient       *http.Client
	EndpointOverride string
}

func (f *MiniMaxTokenPlanFetcher) Fetch(ctx context.Context, baseURL, apiKey string) (*MiniMaxTokenPlanUsage, error) {
	if !isMiniMaxTokenPlanKey(apiKey) {
		return nil, fmt.Errorf("不是 MiniMax Token Plan Key")
	}
	endpoints := miniMaxTokenPlanUsageEndpoints(baseURL)
	if f != nil && strings.TrimSpace(f.EndpointOverride) != "" {
		endpoints = []string{strings.TrimSpace(f.EndpointOverride)}
	}
	if len(endpoints) == 0 {
		return nil, fmt.Errorf("不是 MiniMax 官方端点")
	}

	client := http.DefaultClient
	if f != nil && f.HTTPClient != nil {
		client = f.HTTPClient
	}
	var lastErr error
	for _, endpoint := range endpoints {
		usage, err := fetchMiniMaxTokenPlanUsageEndpoint(ctx, client, endpoint, apiKey)
		if err == nil {
			return usage, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func fetchMiniMaxTokenPlanUsageEndpoint(ctx context.Context, client *http.Client, endpoint, apiKey string) (*MiniMaxTokenPlanUsage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("构造 MiniMax Token Plan 用量请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 MiniMax Token Plan 用量失败: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("读取 MiniMax Token Plan 用量失败: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("MiniMax Token Plan 用量接口 HTTP %d", resp.StatusCode)
	}

	var payload miniMaxTokenPlanResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("解析 MiniMax Token Plan 用量失败: %w", err)
	}
	if payload.BaseResp.StatusCode != 0 {
		return nil, fmt.Errorf("MiniMax Token Plan 用量接口错误 %d: %s", payload.BaseResp.StatusCode, payload.BaseResp.StatusMsg)
	}
	if len(payload.ModelRemains) == 0 {
		return nil, fmt.Errorf("MiniMax Token Plan 用量接口未返回 model_remains")
	}

	usage := &MiniMaxTokenPlanUsage{FetchedAt: time.Now(), SourceURL: endpoint}
	usage.Models = make([]MiniMaxTokenPlanModelUsage, 0, len(payload.ModelRemains))
	for _, model := range payload.ModelRemains {
		if strings.TrimSpace(model.ModelName) == "" {
			continue
		}
		usage.Models = append(usage.Models, MiniMaxTokenPlanModelUsage{
			ModelName:                       model.ModelName,
			CurrentIntervalUsageCount:       model.CurrentIntervalUsageCount,
			CurrentIntervalTotalCount:       model.CurrentIntervalTotalCount,
			CurrentIntervalRemainingPercent: miniMaxRemainingPercent(model.CurrentIntervalRemainingPercent, model.CurrentIntervalUsageCount, model.CurrentIntervalTotalCount),
			CurrentWeeklyUsageCount:         model.CurrentWeeklyUsageCount,
			CurrentWeeklyTotalCount:         model.CurrentWeeklyTotalCount,
			CurrentWeeklyRemainingPercent:   miniMaxRemainingPercent(model.CurrentWeeklyRemainingPercent, model.CurrentWeeklyUsageCount, model.CurrentWeeklyTotalCount),
			RemainsTimeMs:                   model.RemainsTime,
			WeeklyStartTime:                 model.WeeklyStartTime,
			WeeklyEndTime:                   model.WeeklyEndTime,
		})
	}
	if len(usage.Models) == 0 {
		return nil, fmt.Errorf("MiniMax Token Plan 用量接口未返回有效模型")
	}
	return usage, nil
}

func miniMaxRemainingPercent(reported *float64, used, total int64) float64 {
	clamp := func(value float64) float64 {
		if value < 0 {
			return 0
		}
		if value > 100 {
			return 100
		}
		return value
	}
	if reported != nil {
		return clamp(*reported)
	}
	if total <= 0 {
		return 0
	}
	return clamp(float64(total-used) / float64(total) * 100)
}

func miniMaxTokenPlanUsageEndpoints(baseURL string) []string {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return nil
	}
	host := strings.ToLower(parsed.Hostname())
	switch {
	case host == "api.minimaxi.com" || host == "www.minimaxi.com" || host == "api.minimax.chat":
		return []string{
			"https://api.minimaxi.com/v1/api/openplatform/coding_plan/remains",
			"https://www.minimaxi.com/v1/token_plan/remains",
		}
	case host == "api.minimax.io" || host == "www.minimax.io":
		return []string{
			"https://api.minimax.io/v1/api/openplatform/coding_plan/remains",
			"https://www.minimax.io/v1/token_plan/remains",
		}
	default:
		return nil
	}
}

func isMiniMaxTokenPlanKey(apiKey string) bool {
	return strings.HasPrefix(strings.TrimSpace(apiKey), "sk-cp")
}

func cloneMiniMaxTokenPlanUsage(source *MiniMaxTokenPlanUsage) *MiniMaxTokenPlanUsage {
	if source == nil {
		return nil
	}
	clone := *source
	clone.Models = append([]MiniMaxTokenPlanModelUsage(nil), source.Models...)
	return &clone
}

// RefreshMiniMaxTokenPlanUsage 刷新指定 endpoint 的 MiniMax Token Plan 用量。
func (m *Manager) RefreshMiniMaxTokenPlanUsage(ctx context.Context, endpointUID string, force bool) (*MiniMaxTokenPlanUsage, bool, error) {
	if m == nil || m.store == nil {
		return nil, false, fmt.Errorf("画像存储不可用")
	}
	profile := m.store.Get(endpointUID)
	if profile == nil {
		return nil, false, fmt.Errorf("endpoint 不存在")
	}
	if !force && profile.MiniMaxTokenPlanUsage != nil && time.Since(profile.MiniMaxTokenPlanUsage.FetchedAt) < miniMaxTokenPlanUsageTTL {
		return cloneMiniMaxTokenPlanUsage(profile.MiniMaxTokenPlanUsage), true, nil
	}

	keyHash := profile.KeyHash
	if keyHash == "" {
		keyHash = profile.MetricsKey
	}
	apiKey, ok := m.ResolveAPIKey(profile.ChannelUID, keyHash)
	if !ok {
		return nil, false, fmt.Errorf("无法解析 endpoint 凭证")
	}
	if len(miniMaxTokenPlanUsageEndpoints(profile.BaseURL)) == 0 || !isMiniMaxTokenPlanKey(apiKey) {
		return nil, false, fmt.Errorf("endpoint 不是 MiniMax Token Plan")
	}

	fetchCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	usage, err := (&MiniMaxTokenPlanFetcher{HTTPClient: &http.Client{Timeout: 10 * time.Second}}).Fetch(fetchCtx, profile.BaseURL, apiKey)
	profile.UpdatedAt = time.Now()
	if err != nil {
		profile.MiniMaxTokenPlanUsageError = err.Error()
		_ = m.store.Upsert(profile)
		_ = m.store.Flush()
		return nil, false, err
	}
	profile.MiniMaxTokenPlanUsage = usage
	profile.MiniMaxTokenPlanUsageError = ""
	if err := m.store.Upsert(profile); err != nil {
		return nil, false, err
	}
	if err := m.store.Flush(); err != nil {
		return nil, false, err
	}
	return cloneMiniMaxTokenPlanUsage(usage), false, nil
}
