package autopilot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/BenedictKing/ccx/internal/config"
	"github.com/BenedictKing/ccx/internal/errutil"
)

const mimoConsoleBaseURL = "https://platform.xiaomimimo.com"
const maxMiMoConsoleCookieBytes = 32 << 10

type MiMoConsoleClient struct {
	HTTPClient *http.Client
	BaseURL    string
	Now        func() time.Time
}

type mimoConsoleVerification struct {
	APIKey   string
	Snapshot config.MiMoConsoleCredential
}

type mimoConsoleEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func (c *MiMoConsoleClient) Verify(ctx context.Context, cookie string) (*mimoConsoleVerification, error) {
	cookie, err := normalizeMiMoConsoleCookie(cookie)
	if err != nil {
		return nil, err
	}
	var profile struct {
		UserID string `json:"userId"`
	}
	if err := c.get(ctx, cookie, "/api/v1/userProfile", &profile); err != nil {
		return nil, fmt.Errorf("验证 MiMo 控制台 Cookie 失败: %w", err)
	}
	if strings.TrimSpace(profile.UserID) == "" {
		return nil, fmt.Errorf("MiMo 控制台 Cookie 未返回用户身份")
	}
	var detail struct {
		PlanCode         string `json:"planCode"`
		PlanName         string `json:"planName"`
		CurrentPeriodEnd string `json:"currentPeriodEnd"`
		Expired          bool   `json:"expired"`
	}
	if err := c.get(ctx, cookie, "/api/v1/tokenPlan/detail", &detail); err != nil {
		return nil, fmt.Errorf("查询 MiMo Token Plan 详情失败: %w", err)
	}
	if strings.TrimSpace(detail.PlanCode) == "" {
		return nil, fmt.Errorf("cookie 所属账号未查询到 MiMo Token Plan")
	}
	var usage struct {
		MonthUsage mimoConsoleUsageGroup `json:"monthUsage"`
		Usage      mimoConsoleUsageGroup `json:"usage"`
	}
	if err := c.get(ctx, cookie, "/api/v1/tokenPlan/usage", &usage); err != nil {
		return nil, fmt.Errorf("查询 MiMo Token Plan 用量失败: %w", err)
	}
	var apiKey string
	if err := c.get(ctx, cookie, "/api/v1/tokenPlan/apiKey/raw", &apiKey); err != nil {
		return nil, fmt.Errorf("读取 Cookie 所属 Token Plan Key 失败: %w", err)
	}
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, fmt.Errorf("cookie 所属 Token Plan Key 为空")
	}
	now := time.Now()
	if c != nil && c.Now != nil {
		now = c.Now()
	}
	return &mimoConsoleVerification{
		APIKey: apiKey,
		Snapshot: config.MiMoConsoleCredential{
			Cookie:           cookie,
			PlanCode:         detail.PlanCode,
			PlanName:         detail.PlanName,
			CurrentPeriodEnd: detail.CurrentPeriodEnd,
			Expired:          detail.Expired,
			MonthUsage:       usage.MonthUsage.quota(),
			CurrentUsage:     usage.Usage.quota(),
			ValidatedAt:      now,
		},
	}, nil
}

type mimoConsoleUsageGroup struct {
	Percent float64 `json:"percent"`
	Items   []struct {
		Used  int64 `json:"used"`
		Limit int64 `json:"limit"`
	} `json:"items"`
}

func (g mimoConsoleUsageGroup) quota() config.MiMoTokenPlanUsageQuota {
	quota := config.MiMoTokenPlanUsageQuota{UsedPercent: g.Percent}
	for _, item := range g.Items {
		quota.Used += item.Used
		quota.Limit += item.Limit
	}
	if quota.UsedPercent < 0 {
		quota.UsedPercent = 0
	} else if quota.UsedPercent > 1 {
		quota.UsedPercent = 1
	}
	return quota
}

func (c *MiMoConsoleClient) get(ctx context.Context, cookie, path string, target any) error {
	baseURL := mimoConsoleBaseURL
	client := http.DefaultClient
	if c != nil {
		if strings.TrimSpace(c.BaseURL) != "" {
			baseURL = strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
		}
		if c.HTTPClient != nil {
			client = c.HTTPClient
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Cookie", cookie)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Timezone", "Asia/Shanghai")
	req.Header.Set("Referer", mimoConsoleBaseURL+"/console/plan-manage")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer errutil.IgnoreDeferred(resp.Body.Close)
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var envelope mimoConsoleEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("响应不是有效 JSON")
	}
	if envelope.Code != 0 {
		return fmt.Errorf("错误 %d: %s", envelope.Code, envelope.Message)
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return fmt.Errorf("响应 data 为空")
	}
	if err := json.Unmarshal(envelope.Data, target); err != nil {
		return fmt.Errorf("解析响应 data 失败: %w", err)
	}
	return nil
}

func normalizeMiMoConsoleCookie(cookie string) (string, error) {
	cookie = strings.TrimSpace(cookie)
	if strings.HasPrefix(strings.ToLower(cookie), "cookie:") {
		cookie = strings.TrimSpace(cookie[len("cookie:"):])
	}
	if cookie == "" {
		return "", fmt.Errorf("cookie 不能为空")
	}
	if len(cookie) > maxMiMoConsoleCookieBytes {
		return "", fmt.Errorf("cookie 长度超过限制")
	}
	if strings.ContainsAny(cookie, "\r\n") {
		return "", fmt.Errorf("cookie 不能包含换行符")
	}
	hasServiceToken := false
	for part := range strings.SplitSeq(cookie, ";") {
		name, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if ok && name == "api-platform_serviceToken" && strings.TrimSpace(value) != "" {
			hasServiceToken = true
			break
		}
	}
	if !hasServiceToken {
		return "", fmt.Errorf("cookie 缺少 api-platform_serviceToken")
	}
	return cookie, nil
}
