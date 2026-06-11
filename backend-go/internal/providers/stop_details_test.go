package providers

import (
	"io"
	"strings"
	"testing"
)

func TestStopDetailsInMessageDelta(t *testing.T) {
	tests := []struct {
		name         string
		provider     Provider
		streamBody   string
		wantStopReason string
	}{
		{
			name:     "OpenAI end_turn",
			provider: &OpenAIProvider{},
			streamBody: strings.Join([]string{
				`data: {"id":"chatcmpl-1","choices":[{"index":0,"delta":{"role":"assistant","content":"Hi"},"finish_reason":null}]}`,
				`data: {"id":"chatcmpl-1","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
				`data: [DONE]`,
				"",
			}, "\n"),
			wantStopReason: "end_turn",
		},
		{
			name:     "OpenAI tool_use",
			provider: &OpenAIProvider{},
			streamBody: strings.Join([]string{
				`data: {"id":"chatcmpl-2","choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"get_weather","arguments":"{\"loc\":\"SF\"}"}}]},"finish_reason":null}]}`,
				`data: {"id":"chatcmpl-2","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
				`data: [DONE]`,
				"",
			}, "\n"),
			wantStopReason: "tool_use",
		},
		{
			name:     "Gemini end_turn",
			provider: &GeminiProvider{},
			streamBody: strings.Join([]string{
				`data: {"candidates":[{"content":{"parts":[{"text":"Hello"}],"role":"model"},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":5}}`,
				``,
			}, "\n"),
			wantStopReason: "end_turn",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eventChan, errChan, err := tt.provider.HandleStreamResponse(io.NopCloser(strings.NewReader(tt.streamBody)))
			if err != nil {
				t.Fatalf("HandleStreamResponse error: %v", err)
			}

			events := collectStreamEvents(eventChan)
			select {
			case streamErr := <-errChan:
				if streamErr != nil {
					t.Fatalf("stream error: %v", streamErr)
				}
			default:
			}

			messageDelta := extractMessageDelta(t, events)
			delta, ok := messageDelta["delta"].(map[string]interface{})
			if !ok {
				t.Fatalf("delta field missing in message_delta: %v", messageDelta)
			}

			stopReason, _ := delta["stop_reason"].(string)
			if stopReason != tt.wantStopReason {
				t.Errorf("stop_reason = %q, want %q", stopReason, tt.wantStopReason)
			}

			// 验证 stop_details 字段存在且为 nil
			stopDetails, hasStopDetails := delta["stop_details"]
			if !hasStopDetails {
				t.Errorf("stop_details field missing in delta")
			}
			if stopDetails != nil {
				t.Errorf("stop_details = %v, want nil", stopDetails)
			}
		})
	}
}
