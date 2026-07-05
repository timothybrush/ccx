package utils

import (
	"strings"
	"testing"
)

func TestStreamSynthesizerResponsesCustomToolCall(t *testing.T) {
	synthesizer := NewStreamSynthesizer("responses")
	lines := []string{
		`data: {"type":"response.output_item.added","output_index":1,"item":{"id":"ctc_1","type":"custom_tool_call","status":"in_progress","call_id":"call_1","name":"apply_patch","input":""}}`,
		`data: {"type":"response.custom_tool_call_input.delta","output_index":1,"item_id":"ctc_1","delta":"*** Begin Patch\n"}`,
		`data: {"type":"response.custom_tool_call_input.delta","output_index":1,"item_id":"ctc_1","delta":"*** End Patch"}`,
		`data: {"type":"response.custom_tool_call_input.done","output_index":1,"item_id":"ctc_1","input":"*** Begin Patch\n*** End Patch"}`,
		`data: {"type":"response.output_item.done","output_index":1,"item":{"id":"ctc_1","type":"custom_tool_call","status":"completed","call_id":"call_1","name":"apply_patch","input":"*** Begin Patch\n*** End Patch"}}`,
	}

	for _, line := range lines {
		synthesizer.ProcessLine(line)
	}

	content := synthesizer.GetSynthesizedContent()
	if content == "" {
		t.Fatal("expected synthesized custom tool content")
	}
	if strings.Contains(content, "unknown_tool") {
		t.Fatalf("expected custom tool name to be preserved, got %q", content)
	}
	for _, want := range []string{"Tool Call: apply_patch(", "*** Begin Patch", "*** End Patch", "[ID: ctc_1]"} {
		if !strings.Contains(content, want) {
			t.Fatalf("synthesized content missing %q: %s", want, content)
		}
	}
	if synthesizer.IsParseFailed() {
		t.Fatal("custom tool call stream should not mark parse failure")
	}
}

func TestStreamSynthesizerResponsesBroadEventCoverage(t *testing.T) {
	synthesizer := NewStreamSynthesizer("responses")
	lines := []string{
		`data: {"type":"response.reasoning_summary_text.delta","output_index":0,"text":"先分析"}`,
		`data: {"type":"response.content_part.added","output_index":1,"part":{"type":"output_text","text":"答"}}`,
		`data: {"type":"response.output_text.delta","output_index":1,"delta":"案"}`,
		`data: {"type":"response.output_json.delta","output_index":1,"delta":"{\"ok\":true}"}`,
		`data: {"type":"response.output_item.added","output_index":2,"item":{"id":"ts_1","type":"tool_search_call","status":"in_progress","call_id":"call_search","arguments":{"query":"docs"}}}`,
		`data: {"type":"response.output_item.done","output_index":3,"item":{"id":"wso_1","type":"web_search_output","output":"result text"}}`,
		`data: {"type":"response.web_search_text.delta","output_index":4,"delta":"future text"}`,
	}

	for _, line := range lines {
		synthesizer.ProcessLine(line)
	}

	content := synthesizer.GetSynthesizedContent()
	for _, want := range []string{
		"Reasoning:\n先分析",
		"答案{\"ok\":true}",
		"Tool Call: tool_search_call(",
		`"query":"docs"`,
		"Tool Output: web_search_output(result text)",
		"future text",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("synthesized content missing %q:\n%s", want, content)
		}
	}
}

func TestStreamSynthesizerResponsesCompletedFallback(t *testing.T) {
	synthesizer := NewStreamSynthesizer("responses")
	synthesizer.ProcessLine(`data: {"type":"response.completed","response":{"output":[{"type":"reasoning","summary":[{"type":"summary_text","text":"done reasoning"}]},{"type":"message","content":[{"type":"output_text","text":"done text"}]},{"type":"function_call","id":"fc_1","name":"Read","arguments":"{\"path\":\"README.md\"}"}]}}`)

	content := synthesizer.GetSynthesizedContent()
	for _, want := range []string{
		"Reasoning:\ndone reasoning",
		"done text",
		"Tool Call: Read(",
		`"path":"README.md"`,
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("completed fallback missing %q:\n%s", want, content)
		}
	}
}
