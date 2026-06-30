package copilot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultClientID      = "Iv1.b507a08c87ecfe98"
	deviceCodeURL        = "https://github.com/login/device/code"
	accessTokenURL       = "https://github.com/login/oauth/access_token"
	userURL              = "https://api.github.com/user"
	defaultDeviceScope   = "read:user"
	defaultRequestTimout = 30 * time.Second
)

// DeviceCodeResponse 是 GitHub OAuth Device Flow 的设备码响应。
type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// AccessTokenResponse 是 GitHub OAuth Device Flow 轮询 token 的响应。
type AccessTokenResponse struct {
	AccessToken      string `json:"access_token,omitempty"`
	TokenType        string `json:"token_type,omitempty"`
	Scope            string `json:"scope,omitempty"`
	Error            string `json:"error,omitempty"`
	ErrorDescription string `json:"error_description,omitempty"`
}

// User 表示用于验证 token 的 GitHub 用户信息子集。
type User struct {
	Login     string `json:"login"`
	ID        int64  `json:"id"`
	AvatarURL string `json:"avatar_url"`
	HTMLURL   string `json:"html_url"`
}

// OAuthClient 封装 GitHub Copilot OAuth Device Flow。
type OAuthClient struct {
	HTTPClient     *http.Client
	ClientID       string
	DeviceCodeURL  string
	AccessTokenURL string
	UserURL        string
}

// NewOAuthClient 创建 OAuthClient。
func NewOAuthClient(client *http.Client) *OAuthClient {
	if client == nil {
		client = &http.Client{Timeout: defaultRequestTimout}
	}
	return &OAuthClient{
		HTTPClient:     client,
		ClientID:       defaultClientID,
		DeviceCodeURL:  deviceCodeURL,
		AccessTokenURL: accessTokenURL,
		UserURL:        userURL,
	}
}

// RequestDeviceCode 请求 GitHub 设备授权码。
func (c *OAuthClient) RequestDeviceCode(ctx context.Context) (*DeviceCodeResponse, error) {
	payload := map[string]string{
		"client_id": c.clientID(),
		"scope":     defaultDeviceScope,
	}
	var out DeviceCodeResponse
	if err := c.postJSON(ctx, c.deviceCodeURL(), payload, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PollAccessToken 用 device_code 轮询 GitHub OAuth access token。
func (c *OAuthClient) PollAccessToken(ctx context.Context, deviceCode string) (*AccessTokenResponse, error) {
	payload := map[string]string{
		"client_id":   c.clientID(),
		"device_code": strings.TrimSpace(deviceCode),
		"grant_type":  "urn:ietf:params:oauth:grant-type:device_code",
	}
	var out AccessTokenResponse
	if err := c.postJSON(ctx, c.accessTokenURL(), payload, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// VerifyUser 使用 GitHub OAuth token 验证用户身份。
func (c *OAuthClient) VerifyUser(ctx context.Context, token string) (*User, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.userURL(), nil)
	if err != nil {
		return nil, err
	}
	ApplyGitHubHeaders(req.Header)
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GitHub 用户验证请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GitHub 用户验证失败: status=%d body=%s", resp.StatusCode, string(body))
	}

	var user User
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (c *OAuthClient) postJSON(ctx context.Context, url string, payload interface{}, out interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	ApplyGitHubHeaders(req.Header)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("GitHub OAuth 请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("GitHub OAuth 请求失败: status=%d body=%s", resp.StatusCode, string(respBody))
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return err
	}
	return nil
}

func (c *OAuthClient) clientID() string {
	if strings.TrimSpace(c.ClientID) != "" {
		return strings.TrimSpace(c.ClientID)
	}
	return defaultClientID
}

func (c *OAuthClient) deviceCodeURL() string {
	if strings.TrimSpace(c.DeviceCodeURL) != "" {
		return c.DeviceCodeURL
	}
	return deviceCodeURL
}

func (c *OAuthClient) accessTokenURL() string {
	if strings.TrimSpace(c.AccessTokenURL) != "" {
		return c.AccessTokenURL
	}
	return accessTokenURL
}

func (c *OAuthClient) userURL() string {
	if strings.TrimSpace(c.UserURL) != "" {
		return c.UserURL
	}
	return userURL
}
