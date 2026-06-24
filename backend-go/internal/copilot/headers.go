package copilot

import "net/http"

const (
	// DefaultAPIBaseURL 是 GitHub Copilot API 默认上游地址。
	DefaultAPIBaseURL = "https://api.githubcopilot.com"

	defaultIntegrationID       = "vscode-chat"
	defaultUserAgent           = "GitHubCopilotChat/0.26.7"
	defaultEditorVersion       = "vscode/1.104.1"
	defaultEditorPluginVersion = "copilot-chat/0.26.7"
)

// ApplyIdentityHeaders 注入 GitHub Copilot 识别客户端所需的稳定请求头。
func ApplyIdentityHeaders(headers http.Header) {
	headers.Set("Copilot-Integration-Id", defaultIntegrationID)
	headers.Set("User-Agent", defaultUserAgent)
	headers.Set("Editor-Version", defaultEditorVersion)
	headers.Set("Editor-Plugin-Version", defaultEditorPluginVersion)
}

// ApplyRuntimeHeaders 注入调用 api.githubcopilot.com 所需的认证与识别头。
func ApplyRuntimeHeaders(headers http.Header, token string) {
	headers.Set("Authorization", "Bearer "+token)
	headers.Set("openai-organization", "github-copilot")
	headers.Set("openai-intent", "conversation-panel")
	ApplyIdentityHeaders(headers)
}

// ApplyGitHubHeaders 注入调用 github.com/api.github.com OAuth 与 token exchange 所需头。
func ApplyGitHubHeaders(headers http.Header) {
	headers.Set("Accept", "application/json")
	headers.Set("Content-Type", "application/json")
	ApplyIdentityHeaders(headers)
}
