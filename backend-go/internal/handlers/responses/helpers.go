package responses

import (
	"encoding/json"
	"strings"

	"github.com/BenedictKing/ccx/internal/handlers/common"
	"github.com/BenedictKing/ccx/internal/types"
)

func parseInputToItems(input interface{}) ([]types.ResponsesItem, error) {
	return types.ParseResponsesInput(input)
}

func countResponsesUserMessages(input interface{}) int {
	return len(extractResponsesUserInputTexts(input))
}

func extractLastResponsesUserInput(input interface{}) string {
	const maxLen = 80
	texts := extractResponsesUserInputTexts(input)
	if len(texts) == 0 {
		return ""
	}

	var parts []string
	totalLen := 0
	for i := len(texts) - 1; i >= 0; i-- {
		parts = append(parts, texts[i])
		totalLen += len([]rune(texts[i]))
		if totalLen >= maxLen {
			break
		}
	}
	for left, right := 0, len(parts)-1; left < right; left, right = left+1, right-1 {
		parts[left], parts[right] = parts[right], parts[left]
	}
	return strings.Join(parts, " / ")
}

func extractResponsesUserInputTexts(input interface{}) []string {
	switch v := input.(type) {
	case string:
		if cleaned := cleanResponsesUserText(v); cleaned != "" {
			return []string{cleaned}
		}
		return nil
	case []interface{}:
		var texts []string
		for _, item := range v {
			m, ok := item.(map[string]interface{})
			if !ok || m["role"] != "user" {
				continue
			}
			texts = append(texts, extractResponsesContentTexts(m["content"])...)
		}
		return texts
	}
	return nil
}

func extractResponsesContentTexts(content interface{}) []string {
	switch v := content.(type) {
	case string:
		if cleaned := cleanResponsesUserText(v); cleaned != "" {
			return []string{cleaned}
		}
	case []interface{}:
		var texts []string
		for _, block := range v {
			m, ok := block.(map[string]interface{})
			if !ok || m["type"] != "input_text" {
				continue
			}
			if text, ok := m["text"].(string); ok {
				if cleaned := cleanResponsesUserText(text); cleaned != "" {
					texts = append(texts, cleaned)
				}
			}
		}
		return texts
	}
	return nil
}

func cleanResponsesUserText(text string) string {
	text = removeResponsesTaggedBlocks(text, "system-reminder")
	text = removeResponsesTaggedBlocks(text, "local-command-caveat")
	text = removeResponsesTaggedBlocks(text, "command-name")
	text = removeResponsesTaggedBlocks(text, "command-message")
	text = removeResponsesTaggedBlocks(text, "command-args")
	text = removeResponsesTaggedBlocks(text, "local-command-stdout")
	text = removeResponsesTaggedBlocks(text, "local-command-stderr")
	text = strings.TrimSpace(text)
	if isInjectedContextTitleText(text) {
		return ""
	}
	if strings.HasPrefix(text, "<") && strings.Contains(text, ">") {
		return ""
	}
	return text
}

func isInjectedContextTitleText(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
	injectedPrefixes := []string{
		"# agents.md instructions",
		"# claude.md instructions",
		"# codebase and user instructions",
		"<instructions>",
	}
	for _, prefix := range injectedPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return strings.Contains(lower, "project-doc") && strings.Contains(lower, "agents.md")
}

func removeResponsesTaggedBlocks(text, tag string) string {
	for {
		start := strings.Index(text, "<"+tag+">")
		if start < 0 {
			return text
		}
		endTag := "</" + tag + ">"
		end := strings.Index(text[start:], endTag)
		if end < 0 {
			return strings.TrimSpace(text[:start])
		}
		end += start + len(endTag)
		text = text[:start] + text[end:]
	}
}

// hasResponsesFunctionCall 检查 Responses 事件中是否包含工具调用
func hasResponsesFunctionCall(event string) bool {
	lines := strings.Split(event, "\n")
	for _, line := range lines {
		var jsonStr string
		if strings.HasPrefix(line, "data:") {
			jsonStr = strings.TrimPrefix(line, "data:")
			jsonStr = strings.TrimPrefix(jsonStr, " ")
		} else {
			continue
		}

		var data map[string]interface{}
		if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
			continue
		}

		// 检查 response.output 中是否有 function_call 类型
		if response, ok := data["response"].(map[string]interface{}); ok {
			if output, ok := response["output"].([]interface{}); ok {
				for _, item := range output {
					if itemMap, ok := item.(map[string]interface{}); ok {
						if itemType, ok := itemMap["type"].(string); ok && itemType == "function_call" {
							return true
						}
					}
				}
			}
		}
	}
	return false
}

func buildResponsesPreflightDiagnostic(seenEvent, seenCompleted, seenUsageOnly, seenUnknown bool, unknownEventType, text string) string {
	switch {
	case !seenEvent:
		return "未收到任何转换后的 Responses 事件"
	case seenUsageOnly && common.IsEffectivelyEmptyStreamText(text):
		return "仅收到 usage/计数类 Responses 事件，没有文本或语义内容"
	case seenUnknown && common.IsEffectivelyEmptyStreamText(text):
		if unknownEventType != "" {
			return "收到了未识别的 Responses 事件类型=" + unknownEventType + "，但没有文本或语义内容"
		}
		return "收到了未识别的 Responses 事件类型，但没有文本或语义内容"
	case seenCompleted && common.IsEffectivelyEmptyStreamText(text):
		return "流正常结束(response.completed)，但未检测到文本或语义内容"
	default:
		return "检测到空的 Responses 流，但未匹配到明确类别"
	}
}

func isResponsesUsageOnlyEvent(event string) bool {
	lines := strings.Split(event, "\n")
	for _, line := range lines {
		var jsonStr string
		if strings.HasPrefix(line, "data:") {
			jsonStr = strings.TrimPrefix(line, "data:")
			jsonStr = strings.TrimPrefix(jsonStr, " ")
		} else {
			continue
		}
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
			continue
		}
		if data["type"] == "response.completed" {
			if response, ok := data["response"].(map[string]interface{}); ok {
				if usage, ok := response["usage"].(map[string]interface{}); ok && len(usage) > 0 {
					if output, ok := response["output"].([]interface{}); !ok || len(output) == 0 {
						return true
					}
				}
			}
		}
	}
	return false
}

func firstUnknownResponsesEventType(event string) (string, bool) {
	knownTypes := map[string]struct{}{
		"response.output_text.delta": {}, "response.function_call_arguments.delta": {}, "response.function_call_arguments.done": {},
		"response.custom_tool_call_input.delta": {}, "response.custom_tool_call_input.done": {},
		"response.reasoning_summary_text.delta": {}, "response.reasoning_summary_text.done": {}, "response.reasoning_summary_part.added": {}, "response.reasoning_summary_part.done": {},
		"response.output_json.delta": {}, "response.content_part.delta": {}, "response.audio.delta": {}, "response.audio_transcript.delta": {},
		"response.output_item.added": {}, "response.output_item.done": {}, "response.completed": {},
	}
	lines := strings.Split(event, "\n")
	for _, line := range lines {
		var jsonStr string
		if strings.HasPrefix(line, "data:") {
			jsonStr = strings.TrimPrefix(line, "data:")
			jsonStr = strings.TrimPrefix(jsonStr, " ")
		} else {
			continue
		}
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
			continue
		}
		if t, _ := data["type"].(string); t != "" {
			if _, ok := knownTypes[t]; !ok {
				return t, true
			}
		}
	}
	return "", false
}
