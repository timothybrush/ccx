// Package copilot
package copilot

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// DiagnoseResult 表示单次 Copilot 诊断结果。
type DiagnoseResult struct {
	GitHubUser       *User  `json:"githubUser,omitempty"`
	GitHubUserError  string `json:"githubUserError,omitempty"`
	CopilotToken     string `json:"-"`
	CopilotBaseURL   string `json:"copilotBaseUrl,omitempty"`
	TokenError       string `json:"tokenError,omitempty"`
	TokenErrorKind   string `json:"tokenErrorKind,omitempty"`
	ModelsURL        string `json:"modelsUrl,omitempty"`
	ModelsStatus     int    `json:"modelsStatus,omitempty"`
	ModelsError      string `json:"modelsError,omitempty"`
	ModelsBodyPrefix string `json:"modelsBodyPrefix,omitempty"`
}

// Diagnose 依次校验 GitHub 用户、Copilot token exchange、/models 可达性。
func Diagnose(ctx context.Context, client *http.Client, githubToken string, proxyURL string, insecureSkipVerify bool) (*DiagnoseResult, error) {
	githubToken = strings.TrimSpace(githubToken)
	if githubToken == "" {
		return nil, fmt.Errorf("accessToken is required")
	}
	if client == nil {
		client = &http.Client{Timeout: defaultRequestTimout}
	}
	oauth := NewOAuthClient(client)
	tokenMgr := NewTokenManager(client)

	result := &DiagnoseResult{}

	user, err := oauth.VerifyUser(ctx, githubToken)
	if err != nil {
		result.GitHubUserError = err.Error()
	} else {
		result.GitHubUser = user
	}

	copilotToken, copilotBaseURL, err := tokenMgr.ResolveToken(ctx, githubToken)
	if err != nil {
		if te, ok := err.(*TokenExchangeError); ok {
			result.TokenError = te.Message
			result.TokenErrorKind = string(te.Kind)
		} else {
			result.TokenError = err.Error()
			result.TokenErrorKind = string(TokenErrorUnknown)
		}
		return result, nil
	}
	result.CopilotToken = copilotToken
	result.CopilotBaseURL = copilotBaseURL

	modelsURL := strings.TrimRight(copilotBaseURL, "/") + "/models"
	result.ModelsURL = modelsURL
	status, bodyPrefix, err := probeModels(ctx, client, modelsURL, copilotToken)
	if err != nil {
		result.ModelsError = err.Error()
	} else {
		result.ModelsStatus = status
		result.ModelsBodyPrefix = bodyPrefix
	}
	return result, nil
}

func probeModels(ctx context.Context, client *http.Client, url string, copilotToken string) (int, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, "", err
	}
	ApplyRuntimeHeaders(req.Header, copilotToken)

	resp, err := client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512))
	if err != nil {
		return resp.StatusCode, "", err
	}
	return resp.StatusCode, truncate(string(body), 256), nil
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
