package chat

import (
	"encoding/json"
	"testing"

	"github.com/BenedictKing/ccx/internal/config"
)

func TestBuildChatCompletionRequestBody_GLMAutoToolStream(t *testing.T) {
	body := []byte(`{"model":"glm-5.2","stream":true,"messages":[{"role":"user","content":"天气"}],"tools":[{"type":"function","function":{"name":"get_weather","parameters":{"type":"object"}}}]}`)
	upstream := &config.UpstreamConfig{ProviderID: "glm", ServiceType: "openai"}

	converted, err := buildChatCompletionRequestBody(body, "glm-5.2", "glm-5.2", upstream, true)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(converted, &got); err != nil {
		t.Fatal(err)
	}
	if got["tool_stream"] != true {
		t.Fatalf("tool_stream = %#v, body=%s", got["tool_stream"], converted)
	}
}
