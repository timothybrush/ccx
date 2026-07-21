package common

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/BenedictKing/ccx/internal/autopilot"
	"github.com/BenedictKing/ccx/internal/scheduler"
	"github.com/BenedictKing/ccx/internal/types"
	"github.com/gin-gonic/gin"
)

func newAutopilotProfileTestContext(t *testing.T, path, body string, agentCtx *types.AgentContext) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", path, strings.NewReader(body))
	SetRequestLogContextWithAgent(c, "session-test", 1, agentCtx)
	return c
}

func TestAttachAutopilotRequestProfileExtractsProtocolFeatures(t *testing.T) {
	t.Run("messages tools and subagent", func(t *testing.T) {
		body := `{"model":"glm-5.2","messages":[{"role":"user","content":"hello"}],"tools":[{"name":"search"}]}`
		c := newAutopilotProfileTestContext(t, "/v1/messages", body, &types.AgentContext{
			AgentRole: "subagent",
			AgentType: "claude_code_subagent",
		})

		profile := AttachAutopilotRequestProfile(c, scheduler.ChannelKindMessages, "glm-5.2", "completion", "session-test", []byte(body), 0)
		if !profile.ToolUseNeed {
			t.Fatal("ToolUseNeed = false, want true")
		}
		if profile.AgentRole != "subagent" || profile.AgentType != "claude_code_subagent" {
			t.Fatalf("unexpected agent context: role=%q type=%q", profile.AgentRole, profile.AgentType)
		}
		if profile.EstTokens <= 0 || profile.ContextNeed != profile.EstTokens {
			t.Fatalf("unexpected token estimate: est=%d context=%d", profile.EstTokens, profile.ContextNeed)
		}
		if profile.TaskClass != autopilot.TaskClassWorker {
			t.Fatalf("TaskClass = %q, want worker", profile.TaskClass)
		}
		if profile.PromptHash == "" {
			t.Fatal("PromptHash is empty")
		}
	})

	t.Run("responses reasoning blocks lightweight whitelist", func(t *testing.T) {
		body := `{"model":"claude-sonnet-5","input":"summarize this","reasoning":{"effort":"high"}}`
		c := newAutopilotProfileTestContext(t, "/v1/responses", body, nil)

		profile := AttachAutopilotRequestProfile(c, scheduler.ChannelKindResponses, "claude-sonnet-5", "summarize", "session-test", []byte(body), 0)
		if !profile.ReasoningNeed {
			t.Fatal("ReasoningNeed = false, want true")
		}
		if profile.Operation != "summarize" {
			t.Fatalf("Operation = %q, want summarize", profile.Operation)
		}
		if profile.TaskClass == autopilot.TaskClassLightweight {
			t.Fatal("reasoning request must not be classified as lightweight")
		}
	})

	t.Run("messages MCP tool names require tool support", func(t *testing.T) {
		body := `{"model":"claude-opus-4-8","messages":[{"role":"user","content":"inspect this repository"}],"tools":["Bash","mcp__serena__find_symbol"]}`
		c := newAutopilotProfileTestContext(t, "/v1/messages", body, nil)

		profile := AttachAutopilotRequestProfile(c, scheduler.ChannelKindMessages, "claude-opus-4-8", "completion", "session-test", []byte(body), 0)
		if !profile.ToolUseNeed {
			t.Fatal("MCP 工具列表必须设置 ToolUseNeed=true")
		}
	})

	t.Run("disabled thinking remains false", func(t *testing.T) {
		body := `{"model":"claude-sonnet-5","messages":[{"role":"user","content":"hello"}],"thinking":{"type":"disabled"}}`
		c := newAutopilotProfileTestContext(t, "/v1/messages", body, nil)

		profile := AttachAutopilotRequestProfile(c, scheduler.ChannelKindMessages, "claude-sonnet-5", "completion", "session-test", []byte(body), 0)
		if profile.ReasoningNeed {
			t.Fatal("ReasoningNeed = true for disabled thinking")
		}
	})

	t.Run("zero Gemini thinking budget remains false", func(t *testing.T) {
		body := `{"contents":[{"role":"user","parts":[{"text":"hello"}]}],"generationConfig":{"thinkingConfig":{"thinkingBudget":0}}}`
		c := newAutopilotProfileTestContext(t, "/v1beta/models/gemini-3.5-flash:generateContent", body, nil)

		profile := AttachAutopilotRequestProfile(c, scheduler.ChannelKindGemini, "gemini-3.5-flash", "completion", "session-test", []byte(body), 0)
		if profile.ReasoningNeed {
			t.Fatal("ReasoningNeed = true for zero thinking budget")
		}
	})

	t.Run("vectors preserves dimensions", func(t *testing.T) {
		body := `{"model":"text-embedding-3-small","input":"hello","dimensions":1536}`
		c := newAutopilotProfileTestContext(t, "/v1/embeddings", body, nil)

		profile := AttachAutopilotRequestProfile(c, scheduler.ChannelKindVectors, "text-embedding-3-small", "embedding", "session-test", []byte(body), 1536)
		if profile.TaskClass != autopilot.TaskClassEmbedding {
			t.Fatalf("TaskClass = %q, want embedding", profile.TaskClass)
		}
		if profile.EmbeddingDimension != 1536 {
			t.Fatalf("EmbeddingDimension = %d, want 1536", profile.EmbeddingDimension)
		}
	})
}

func TestEnsureAutopilotRequestProfilePreservesAttachedProfile(t *testing.T) {
	body := `{"model":"claude-sonnet-5","input":"summary"}`
	c := newAutopilotProfileTestContext(t, "/v1/responses/compact", body, nil)
	attached := AttachAutopilotRequestProfile(c, scheduler.ChannelKindResponses, "claude-sonnet-5", "summarize", "session-test", []byte(body), 0)

	ensured := EnsureAutopilotRequestProfile(c, scheduler.ChannelKindResponses, "other-model", "completion", "other-session", []byte(body))
	if ensured.Model != attached.Model || ensured.Operation != attached.Operation || ensured.PromptHash != attached.PromptHash {
		t.Fatalf("EnsureAutopilotRequestProfile replaced attached profile: got %+v want %+v", ensured, attached)
	}
}
