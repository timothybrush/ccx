package utils

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
)

// StreamSynthesizer 流式响应内容合成器
type StreamSynthesizer struct {
	serviceType         string
	synthesizedContent  strings.Builder
	toolCallAccumulator map[int]*ToolCall
	parseFailed         bool

	// responses专用累积器
	responsesText      map[int]*strings.Builder
	responsesReasoning map[int]*strings.Builder
}

// ToolCall 工具调用累积器
type ToolCall struct {
	ID        string
	Name      string
	Arguments string
}

// NewStreamSynthesizer 创建新的流合成器
func NewStreamSynthesizer(serviceType string) *StreamSynthesizer {
	return &StreamSynthesizer{
		serviceType:         serviceType,
		toolCallAccumulator: make(map[int]*ToolCall),
		responsesText:       make(map[int]*strings.Builder),
		responsesReasoning:  make(map[int]*strings.Builder),
	}
}

// ProcessLine 处理SSE流的一行
func (s *StreamSynthesizer) ProcessLine(line string) {
	trimmedLine := strings.TrimSpace(line)
	if trimmedLine == "" {
		return
	}

	if !strings.HasPrefix(trimmedLine, "data:") {
		return
	}

	jsonStr := strings.TrimSpace(strings.TrimPrefix(trimmedLine, "data:"))
	if jsonStr == "[DONE]" || jsonStr == "" {
		return
	}

	// 解析JSON - 不再因失败而停止处理
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		// 记录解析失败但继续处理后续行，而不是完全停止
		if !s.parseFailed {
			s.parseFailed = true
			s.synthesizedContent.WriteString("\n[解析警告: 部分JSON解析失败，将显示原始文本内容]")
		}
		return
	}

	// 如果之前解析失败，但现在成功了，重置失败标记
	if s.parseFailed {
		s.parseFailed = false
	}

	// 根据服务类型解析
	switch s.serviceType {
	case "gemini":
		s.processGemini(data)
	case "openai":
		s.processOpenAI(data)
	case "claude":
		s.processClaude(data)
	case "responses":
		s.processResponses(data)
	}
}

// processResponses 处理OpenAI Responses流
func (s *StreamSynthesizer) processResponses(data map[string]interface{}) {
	typeStr, _ := data["type"].(string)

	switch typeStr {
	case "response.output_text.delta":
		if delta, ok := data["delta"].(string); ok {
			s.appendResponsesText(responseOutputIndex(data), delta)
		}
	case "response.output_text.done", "response.output_json.done":
		if text, ok := data["text"].(string); ok && text != "" {
			s.setResponsesText(responseOutputIndex(data), text)
		}
	case "response.output_json.delta", "response.content_part.delta", "response.audio_transcript.delta":
		s.appendResponsesText(responseOutputIndex(data), firstResponsesString(data, "delta", "text", "transcript"))
	case "response.audio.delta":
		if delta, ok := data["delta"].(string); ok && delta != "" {
			s.appendSynthesizedLine("[Audio delta omitted]")
		}
	case "response.content_part.added", "response.content_part.done":
		if part, ok := data["part"].(map[string]interface{}); ok {
			if text := responsesContentText(part); text != "" {
				if typeStr == "response.content_part.done" {
					s.setResponsesText(responseOutputIndex(data), text)
				} else {
					s.appendResponsesText(responseOutputIndex(data), text)
				}
			}
		}
	case "response.reasoning_summary_text.delta", "response.reasoning_text.delta":
		s.appendResponsesReasoning(responseOutputIndex(data), firstResponsesString(data, "delta", "text"))
	case "response.reasoning_summary_text.done":
		if text := firstResponsesString(data, "text"); text != "" {
			s.setResponsesReasoning(responseOutputIndex(data), text)
		}
	case "response.reasoning_summary_part.added", "response.reasoning_summary_part.done":
		if part, ok := data["part"].(map[string]interface{}); ok {
			if text := responsesContentText(part); text != "" {
				if typeStr == "response.reasoning_summary_part.done" {
					s.setResponsesReasoning(responseOutputIndex(data), text)
				} else {
					s.appendResponsesReasoning(responseOutputIndex(data), text)
				}
			}
		}
	case "response.completed":
		s.processResponsesOutput(data, true)
	case "response.output_item.added":
		if item, ok := data["item"].(map[string]interface{}); ok {
			s.processResponsesItem(responseOutputIndex(data), item, false)
		}
	case "response.function_call_arguments.delta":
		acc := s.getToolCall(responseOutputIndex(data))
		if id, ok := data["item_id"].(string); ok && id != "" {
			acc.ID = id
		}
		if delta, ok := data["delta"].(string); ok {
			acc.Arguments += delta
		}
	case "response.function_call_arguments.done":
		acc := s.getToolCall(responseOutputIndex(data))
		if id, ok := data["item_id"].(string); ok && id != "" {
			acc.ID = id
		}
		if args, ok := data["arguments"].(string); ok && args != "" {
			acc.Arguments = args
		}
		if item, ok := data["item"].(map[string]interface{}); ok {
			if name, ok := item["name"].(string); ok && name != "" {
				acc.Name = name
			}
		}
	case "response.custom_tool_call_input.delta":
		acc := s.getToolCall(responseOutputIndex(data))
		if id, ok := data["item_id"].(string); ok && id != "" {
			acc.ID = id
		}
		if delta, ok := data["delta"].(string); ok {
			acc.Arguments += delta
		}
	case "response.custom_tool_call_input.done":
		acc := s.getToolCall(responseOutputIndex(data))
		if id, ok := data["item_id"].(string); ok && id != "" {
			acc.ID = id
		}
		if input, ok := data["input"].(string); ok && input != "" {
			acc.Arguments = input
		}
	case "response.output_item.done":
		if item, ok := data["item"].(map[string]interface{}); ok {
			s.processResponsesItem(responseOutputIndex(data), item, true)
		}
	case "response.error", "response.failed", "error":
		s.appendResponsesError(data)
	default:
		s.processGenericResponsesEvent(typeStr, data)
	}
}

func responseOutputIndex(data map[string]interface{}) int {
	if idx, ok := data["output_index"].(float64); ok {
		return int(idx)
	}
	return 0
}

func (s *StreamSynthesizer) responseTextBuilder(index int) *strings.Builder {
	if s.responsesText[index] == nil {
		s.responsesText[index] = &strings.Builder{}
	}
	return s.responsesText[index]
}

func (s *StreamSynthesizer) responseReasoningBuilder(index int) *strings.Builder {
	if s.responsesReasoning[index] == nil {
		s.responsesReasoning[index] = &strings.Builder{}
	}
	return s.responsesReasoning[index]
}

func (s *StreamSynthesizer) appendResponsesText(index int, text string) {
	if text == "" {
		return
	}
	s.responseTextBuilder(index).WriteString(text)
}

func (s *StreamSynthesizer) setResponsesText(index int, text string) {
	if text == "" {
		return
	}
	builder := s.responseTextBuilder(index)
	builder.Reset()
	builder.WriteString(text)
}

func (s *StreamSynthesizer) appendResponsesReasoning(index int, text string) {
	if text == "" {
		return
	}
	s.responseReasoningBuilder(index).WriteString(text)
}

func (s *StreamSynthesizer) setResponsesReasoning(index int, text string) {
	if text == "" {
		return
	}
	builder := s.responseReasoningBuilder(index)
	builder.Reset()
	builder.WriteString(text)
}

func (s *StreamSynthesizer) appendSynthesizedLine(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	if s.synthesizedContent.Len() > 0 {
		s.synthesizedContent.WriteString("\n")
	}
	s.synthesizedContent.WriteString(line)
}

func (s *StreamSynthesizer) getToolCall(index int) *ToolCall {
	if s.toolCallAccumulator[index] == nil {
		s.toolCallAccumulator[index] = &ToolCall{}
	}
	return s.toolCallAccumulator[index]
}

func (s *StreamSynthesizer) processResponsesOutput(data map[string]interface{}, final bool) {
	respObj, ok := data["response"].(map[string]interface{})
	if !ok {
		respObj = data
	}
	outputArr, ok := respObj["output"].([]interface{})
	if !ok {
		return
	}
	for i, item := range outputArr {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		s.processResponsesItem(i, itemMap, final)
	}
}

func (s *StreamSynthesizer) processResponsesItem(index int, item map[string]interface{}, final bool) {
	itemType, _ := item["type"].(string)
	switch {
	case itemType == "message" || itemType == "text":
		if text := responsesContentText(item); text != "" {
			if final {
				s.setResponsesText(index, text)
			} else {
				s.appendResponsesText(index, text)
			}
		}
	case itemType == "reasoning":
		if text := responsesReasoningText(item); text != "" {
			if final {
				s.setResponsesReasoning(index, text)
			} else {
				s.appendResponsesReasoning(index, text)
			}
		} else if firstResponsesString(item, "encrypted_content") != "" {
			s.setResponsesReasoning(index, "[encrypted_content omitted]")
		}
	case itemType == "compaction" || itemType == "compaction_summary":
		if firstResponsesString(item, "encrypted_content") != "" {
			s.appendSynthesizedLine("Compaction: encrypted_content omitted")
		}
	case isResponsesCallItemType(itemType):
		s.upsertResponsesToolCall(index, item)
	case isResponsesOutputItemType(itemType):
		s.appendResponsesToolOutput(item)
	default:
		if text := responsesContentText(item); text != "" {
			if final {
				s.setResponsesText(index, text)
			} else {
				s.appendResponsesText(index, text)
			}
		}
	}
}

func (s *StreamSynthesizer) upsertResponsesToolCall(index int, item map[string]interface{}) {
	acc := s.getToolCall(index)
	if id := firstResponsesString(item, "id", "call_id"); id != "" {
		acc.ID = id
	}
	name := firstResponsesString(item, "name")
	if namespace := firstResponsesString(item, "namespace"); namespace != "" && name != "" {
		name = namespace + "." + name
	}
	if name == "" {
		name = firstResponsesString(item, "type")
	}
	if name != "" {
		acc.Name = name
	}
	if args := firstResponsesString(item, "arguments", "input", "query", "queries", "code", "action"); args != "" {
		acc.Arguments = args
	}
}

func (s *StreamSynthesizer) appendResponsesToolOutput(item map[string]interface{}) {
	name := firstResponsesString(item, "name", "type")
	id := firstResponsesString(item, "call_id", "id")
	output := firstResponsesString(item, "output", "result", "results", "content", "text")
	if output == "" {
		output = firstResponsesString(item, "status")
	}
	var line strings.Builder
	line.WriteString("Tool Output: ")
	if name == "" {
		name = "tool_output"
	}
	line.WriteString(name)
	if output != "" {
		line.WriteString("(")
		line.WriteString(output)
		line.WriteString(")")
	}
	if id != "" {
		line.WriteString(" [ID: ")
		line.WriteString(id)
		line.WriteString("]")
	}
	s.appendSynthesizedLine(line.String())
}

func (s *StreamSynthesizer) appendResponsesError(data map[string]interface{}) {
	if errObj, ok := data["error"].(map[string]interface{}); ok {
		if msg := firstResponsesString(errObj, "message", "code", "type"); msg != "" {
			s.appendSynthesizedLine("Error: " + msg)
			return
		}
	}
	if resp, ok := data["response"].(map[string]interface{}); ok {
		if errObj, ok := resp["error"].(map[string]interface{}); ok {
			if msg := firstResponsesString(errObj, "message", "code", "type"); msg != "" {
				s.appendSynthesizedLine("Error: " + msg)
				return
			}
		}
	}
	if msg := firstResponsesString(data, "message", "error"); msg != "" {
		s.appendSynthesizedLine("Error: " + msg)
	}
}

func (s *StreamSynthesizer) processGenericResponsesEvent(typeStr string, data map[string]interface{}) {
	if !strings.HasPrefix(typeStr, "response.") {
		return
	}
	if item, ok := data["item"].(map[string]interface{}); ok {
		s.processResponsesItem(responseOutputIndex(data), item, strings.HasSuffix(typeStr, ".done"))
	}
	s.processResponsesOutput(data, strings.HasSuffix(typeStr, ".done") || typeStr == "response.completed")
	if strings.HasSuffix(typeStr, ".delta") || strings.HasSuffix(typeStr, ".done") {
		text := firstResponsesString(data, "delta", "text", "transcript")
		if strings.Contains(typeStr, "reasoning") {
			s.appendResponsesReasoning(responseOutputIndex(data), text)
		} else {
			s.appendResponsesText(responseOutputIndex(data), text)
		}
	}
	if callID := firstResponsesString(data, "call_id"); callID != "" {
		s.appendSynthesizedLine(typeStr + " [call_id: " + callID + "]")
	}
}

func responsesContentText(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case map[string]interface{}:
		if text := firstResponsesString(v, "text", "delta", "transcript"); text != "" {
			return text
		}
		if part, ok := v["part"].(map[string]interface{}); ok {
			return responsesContentText(part)
		}
		if content, ok := v["content"]; ok {
			return responsesContentText(content)
		}
	case []interface{}:
		var builder strings.Builder
		for _, item := range v {
			text := responsesContentText(item)
			if text == "" {
				continue
			}
			if builder.Len() > 0 {
				builder.WriteString("\n")
			}
			builder.WriteString(text)
		}
		return builder.String()
	}
	return ""
}

func responsesReasoningText(item map[string]interface{}) string {
	if text := firstResponsesString(item, "text", "delta"); text != "" {
		return text
	}
	if summary, ok := item["summary"]; ok {
		return responsesContentText(summary)
	}
	return ""
}

func firstResponsesString(data map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := data[key]; ok {
			if text := responsesStringValue(value); text != "" {
				return text
			}
		}
	}
	return ""
}

func responsesStringValue(value interface{}) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	default:
		encoded, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		result := string(encoded)
		if result == "null" {
			return ""
		}
		return result
	}
}

func isResponsesCallItemType(itemType string) bool {
	return itemType == "function_call" || itemType == "custom_tool_call" || strings.HasSuffix(itemType, "_call")
}

func isResponsesOutputItemType(itemType string) bool {
	return strings.HasSuffix(itemType, "_output")
}

// processGemini 处理Gemini格式
func (s *StreamSynthesizer) processGemini(data map[string]interface{}) {
	candidates, ok := data["candidates"].([]interface{})
	if !ok || len(candidates) == 0 {
		return
	}

	candidate, ok := candidates[0].(map[string]interface{})
	if !ok {
		return
	}

	content, ok := candidate["content"].(map[string]interface{})
	if !ok {
		return
	}

	parts, ok := content["parts"].([]interface{})
	if !ok {
		return
	}

	for _, part := range parts {
		partMap, ok := part.(map[string]interface{})
		if !ok {
			continue
		}

		// 文本内容
		if text, ok := partMap["text"].(string); ok {
			s.synthesizedContent.WriteString(text)
		}

		// 函数调用
		if functionCall, ok := partMap["functionCall"].(map[string]interface{}); ok {
			name, _ := functionCall["name"].(string)
			args, _ := functionCall["args"]
			argsJSON, _ := json.Marshal(args)
			s.synthesizedContent.WriteString("\nTool Call: ")
			s.synthesizedContent.WriteString(name)
			s.synthesizedContent.WriteString("(")
			s.synthesizedContent.Write(argsJSON)
			s.synthesizedContent.WriteString(")")
		}
	}
}

// processOpenAI 处理OpenAI格式
func (s *StreamSynthesizer) processOpenAI(data map[string]interface{}) {
	choices, ok := data["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return
	}

	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		return
	}

	delta, ok := choice["delta"].(map[string]interface{})
	if !ok {
		return
	}

	// 文本内容
	if content, ok := delta["content"].(string); ok {
		s.synthesizedContent.WriteString(content)
	}

	// 工具调用
	if toolCalls, ok := delta["tool_calls"].([]interface{}); ok {
		for _, tc := range toolCalls {
			toolCallMap, ok := tc.(map[string]interface{})
			if !ok {
				continue
			}

			index := 0
			if idx, ok := toolCallMap["index"].(float64); ok {
				index = int(idx)
			}

			if s.toolCallAccumulator[index] == nil {
				s.toolCallAccumulator[index] = &ToolCall{}
			}

			accumulated := s.toolCallAccumulator[index]

			if id, ok := toolCallMap["id"].(string); ok {
				accumulated.ID = id
			}

			if function, ok := toolCallMap["function"].(map[string]interface{}); ok {
				if name, ok := function["name"].(string); ok {
					accumulated.Name = name
				}
				if args, ok := function["arguments"].(string); ok {
					accumulated.Arguments += args
				}
			}
		}
	}
}

// processClaude 处理Claude格式
func (s *StreamSynthesizer) processClaude(data map[string]interface{}) {
	eventType, _ := data["type"].(string)

	switch eventType {
	case "message_start":
		// 从 message_start 中提取初始内容（如果有）
		if msg, ok := data["message"].(map[string]interface{}); ok {
			if content, ok := msg["content"].([]interface{}); ok {
				for _, c := range content {
					if cm, ok := c.(map[string]interface{}); ok {
						if text, ok := cm["text"].(string); ok {
							s.synthesizedContent.WriteString(text)
						}
					}
				}
			}
		}

	case "content_block_start":
		contentBlock, ok := data["content_block"].(map[string]interface{})
		if !ok {
			return
		}

		blockIndex := 0
		if idx, ok := data["index"].(float64); ok {
			blockIndex = int(idx)
		}

		blockType, _ := contentBlock["type"].(string)

		switch blockType {
		case "tool_use":
			if s.toolCallAccumulator[blockIndex] == nil {
				s.toolCallAccumulator[blockIndex] = &ToolCall{}
			}
			accumulated := s.toolCallAccumulator[blockIndex]
			if id, ok := contentBlock["id"].(string); ok {
				accumulated.ID = id
			}
			if name, ok := contentBlock["name"].(string); ok {
				accumulated.Name = name
			}
		case "text":
			// text 类型的 content_block_start 可能包含初始文本
			if text, ok := contentBlock["text"].(string); ok && text != "" {
				s.synthesizedContent.WriteString(text)
			}
		}

	case "content_block_delta":
		delta, ok := data["delta"].(map[string]interface{})
		if !ok {
			return
		}

		deltaType, _ := delta["type"].(string)

		switch deltaType {
		case "text_delta":
			if text, ok := delta["text"].(string); ok {
				s.synthesizedContent.WriteString(text)
			}
		case "input_json_delta":
			if partialJSON, ok := delta["partial_json"].(string); ok {
				blockIndex := 0
				if idx, ok := data["index"].(float64); ok {
					blockIndex = int(idx)
				}

				if s.toolCallAccumulator[blockIndex] == nil {
					s.toolCallAccumulator[blockIndex] = &ToolCall{}
				}

				accumulated := s.toolCallAccumulator[blockIndex]
				accumulated.Arguments += partialJSON
			}
		case "thinking_delta":
			// thinking 内容不记录到合成内容中（可选：如需记录可取消注释）
			// if thinking, ok := delta["thinking"].(string); ok {
			// 	s.synthesizedContent.WriteString(thinking)
			// }
		}

	case "message_delta":
		// message_delta 通常包含 stop_reason 和 usage，不包含文本内容
		// 但某些情况下可能有额外数据，这里做兜底处理
		if delta, ok := data["delta"].(map[string]interface{}); ok {
			if text, ok := delta["text"].(string); ok {
				s.synthesizedContent.WriteString(text)
			}
		}
	}
}

// GetSynthesizedContent 获取合成的内容
func (s *StreamSynthesizer) GetSynthesizedContent() string {
	// 不再完全失败，即使有解析错误也返回部分结果
	var result string

	if s.serviceType == "responses" {
		var parts []string
		if content := strings.TrimSpace(s.synthesizedContent.String()); content != "" {
			parts = append(parts, content)
		}
		if reasoning := collectResponseBuilders(s.responsesReasoning); reasoning != "" {
			parts = append(parts, "Reasoning:\n"+reasoning)
		}
		if text := collectResponseBuilders(s.responsesText); text != "" {
			parts = append(parts, text)
		}
		result = strings.Join(parts, "\n")
	} else {
		result = s.synthesizedContent.String()
	}

	// 添加工具调用信息
	if len(s.toolCallAccumulator) > 0 {
		// 修复分裂的工具调用：检测并合并元数据和参数分离的情况
		s.mergeSplitToolCalls()

		// 按 index 排序输出，避免 map 遍历顺序不稳定
		indices := make([]int, 0, len(s.toolCallAccumulator))
		for idx := range s.toolCallAccumulator {
			indices = append(indices, idx)
		}
		sort.Ints(indices)

		var toolCallsBuilder strings.Builder
		for _, index := range indices {
			tool := s.toolCallAccumulator[index]
			args := tool.Arguments
			if args == "" {
				args = "{}"
			}

			name := tool.Name
			if name == "" {
				name = "unknown_tool"
			}

			id := tool.ID
			if id == "" {
				id = "tool_" + strconv.Itoa(index)
			}

			toolCallsBuilder.WriteString("\nTool Call: ")
			toolCallsBuilder.WriteString(name)
			toolCallsBuilder.WriteString("(")

			// 尝试格式化JSON
			var parsedArgs interface{}
			if err := json.Unmarshal([]byte(args), &parsedArgs); err == nil {
				prettyArgs, _ := json.Marshal(parsedArgs)
				toolCallsBuilder.Write(prettyArgs)
			} else {
				toolCallsBuilder.WriteString(args)
			}

			toolCallsBuilder.WriteString(") [ID: ")
			toolCallsBuilder.WriteString(id)
			toolCallsBuilder.WriteString("]")
		}

		toolCalls := toolCallsBuilder.String()
		if result == "" {
			result = strings.TrimPrefix(toolCalls, "\n")
		} else {
			result += toolCalls
		}
	}

	return result
}

func collectResponseBuilders(builders map[int]*strings.Builder) string {
	if len(builders) == 0 {
		return ""
	}
	keys := make([]int, 0, len(builders))
	for k := range builders {
		keys = append(keys, k)
	}
	sort.Ints(keys)

	var builder strings.Builder
	for _, k := range keys {
		if builders[k] == nil {
			continue
		}
		text := builders[k].String()
		if text == "" {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(text)
	}
	return builder.String()
}

// mergeSplitToolCalls 修复分裂的工具调用
// 问题场景：上游返回的工具调用被意外分成两个 content_block：
// - 第一个 block 有 name 和 id，但参数为空 "{}"
// - 第二个 block 没有 name（显示为 unknown_function），但有完整参数
// 此方法检测并合并这种情况
func (s *StreamSynthesizer) mergeSplitToolCalls() {
	if len(s.toolCallAccumulator) < 2 {
		return
	}

	// 收集所有索引并排序
	indices := make([]int, 0, len(s.toolCallAccumulator))
	for idx := range s.toolCallAccumulator {
		indices = append(indices, idx)
	}
	sort.Ints(indices)

	// 检测分裂模式：有 name 但参数为空/"{}" 的 block，后面紧跟无 name 但有参数的 block
	toDelete := make(map[int]bool)

	for i := 0; i < len(indices)-1; i++ {
		currIdx := indices[i]
		nextIdx := indices[i+1]

		// 约束：只合并连续的 index（防止误合并不相关的调用）
		if nextIdx != currIdx+1 {
			continue
		}

		curr := s.toolCallAccumulator[currIdx]
		next := s.toolCallAccumulator[nextIdx]

		// 检测分裂条件：
		// 1. 当前 block 有 name 和 id，但参数为空或只有 "{}"
		// 2. 下一个 block 没有 name，但有实际参数
		// 3. 如果 next 有 ID，必须与 curr 相同（或 curr 无 ID）
		currArgsEmpty := curr.Arguments == "" || curr.Arguments == "{}"
		nextHasNoName := next.Name == ""
		nextHasArgs := next.Arguments != "" && next.Arguments != "{}"
		idMatch := next.ID == "" || curr.ID == "" || next.ID == curr.ID

		if curr.Name != "" && currArgsEmpty && nextHasNoName && nextHasArgs && idMatch {
			// 合并：将 next 的参数移到 curr，补全缺失字段
			curr.Arguments = next.Arguments
			if curr.ID == "" && next.ID != "" {
				curr.ID = next.ID
			}
			toDelete[nextIdx] = true
			// 跳过下一个，因为已经处理了
			i++
		}
	}

	// 删除已合并的 block
	for idx := range toDelete {
		delete(s.toolCallAccumulator, idx)
	}
}

// IsParseFailed 检查解析是否失败
func (s *StreamSynthesizer) IsParseFailed() bool {
	return s.parseFailed
}

// HasToolCalls 检查是否有工具调用被处理
func (s *StreamSynthesizer) HasToolCalls() bool {
	return len(s.toolCallAccumulator) > 0
}
