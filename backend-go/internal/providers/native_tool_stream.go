package providers

import (
	"reflect"
	"strings"

	"github.com/BenedictKing/ccx/internal/config"
)

// ApplyNativeToolStreaming 为支持该扩展的官方 OpenAI Chat 上游开启工具参数流式输出。
// 客户端显式传入 tool_stream 时保留其选择。
func ApplyNativeToolStreaming(req map[string]interface{}, upstream *config.UpstreamConfig) {
	if req == nil || upstream == nil ||
		!strings.EqualFold(strings.TrimSpace(upstream.ProviderID), "glm") ||
		!strings.EqualFold(strings.TrimSpace(upstream.ServiceType), "openai") {
		return
	}
	if _, exists := req["tool_stream"]; exists {
		return
	}
	stream, _ := req["stream"].(bool)
	if !stream || !hasCollectionItems(req["tools"]) {
		return
	}
	req["tool_stream"] = true
}

func hasCollectionItems(value interface{}) bool {
	if value == nil {
		return false
	}
	collection := reflect.ValueOf(value)
	return (collection.Kind() == reflect.Array || collection.Kind() == reflect.Slice) && collection.Len() > 0
}
