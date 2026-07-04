package responses

import "testing"

func TestExtractLastResponsesUserInputUsesLastInputText(t *testing.T) {
	input := []interface{}{
		map[string]interface{}{
			"role": "user",
			"content": []interface{}{
				map[string]interface{}{"type": "input_text", "text": "<system-reminder>ignore</system-reminder>"},
				map[string]interface{}{"type": "input_text", "text": "ls"},
			},
		},
		map[string]interface{}{"type": "function_call", "name": "Bash"},
		map[string]interface{}{"type": "function_call_output", "output": "ignored"},
	}

	if got := extractLastResponsesUserInput(input); got != "ls" {
		t.Fatalf("extractLastResponsesUserInput() = %q, want ls", got)
	}
	if got := countResponsesUserMessages(input); got != 1 {
		t.Fatalf("countResponsesUserMessages() = %d, want 1", got)
	}
}

func TestExtractLastResponsesUserInputSkipsInjectedAgentsInstructions(t *testing.T) {
	input := []interface{}{
		map[string]interface{}{
			"role": "user",
			"content": []interface{}{
				map[string]interface{}{"type": "input_text", "text": "# AGENTS.md instructions for /Users/example/project\n<INSTRUCTIONS>\nAlways respond in Chinese\n</INSTRUCTIONS>"},
				map[string]interface{}{"type": "input_text", "text": "这个展开的对话卡片应该优化"},
			},
		},
	}

	if got := extractLastResponsesUserInput(input); got != "这个展开的对话卡片应该优化" {
		t.Fatalf("extractLastResponsesUserInput() = %q, want %q", got, "这个展开的对话卡片应该优化")
	}
	if got := countResponsesUserMessages(input); got != 1 {
		t.Fatalf("countResponsesUserMessages() = %d, want 1", got)
	}
}

func TestExtractLastResponsesUserInputSkipsClaudeMetaRetryPrompt(t *testing.T) {
	input := []interface{}{
		map[string]interface{}{"role": "user", "content": []interface{}{map[string]interface{}{"type": "input_text", "text": "真实用户问题"}}},
		map[string]interface{}{"role": "user", "content": []interface{}{map[string]interface{}{"type": "input_text", "text": "[Your previous response had no visible output. Please continue and produce a user-visible response.]"}}},
	}

	if got := extractLastResponsesUserInput(input); got != "真实用户问题" {
		t.Fatalf("extractLastResponsesUserInput() = %q, want %q", got, "真实用户问题")
	}
	if got := countResponsesUserMessages(input); got != 1 {
		t.Fatalf("countResponsesUserMessages() = %d, want 1", got)
	}
}

func TestExtractLastResponsesUserInputJoinsShortInputs(t *testing.T) {
	input := []interface{}{
		map[string]interface{}{"role": "user", "content": []interface{}{map[string]interface{}{"type": "input_text", "text": "第一个"}}},
		map[string]interface{}{"role": "assistant", "content": []interface{}{map[string]interface{}{"type": "output_text", "text": "回答"}}},
		map[string]interface{}{"role": "user", "content": []interface{}{map[string]interface{}{"type": "input_text", "text": "第二个"}}},
	}

	if got := extractLastResponsesUserInput(input); got != "第一个 / 第二个" {
		t.Fatalf("extractLastResponsesUserInput() = %q, want %q", got, "第一个 / 第二个")
	}
	if got := countResponsesUserMessages(input); got != 2 {
		t.Fatalf("countResponsesUserMessages() = %d, want 2", got)
	}
}

func TestExtractRecentResponsesUserInputsKeepsOneItemWithDelimiters(t *testing.T) {
	input := []interface{}{
		map[string]interface{}{
			"role": "user",
			"content": []interface{}{
				map[string]interface{}{"type": "input_text", "text": "Base directory for this skill: /private/tmp/skill"},
				map[string]interface{}{"type": "input_text", "text": "| API | socket | send the request |"},
			},
		},
	}

	want := "Base directory for this skill: /private/tmp/skill\n| API | socket | send the request |"
	got := extractRecentResponsesUserInputs(input)
	if len(got) != 1 || got[0] != want {
		t.Fatalf("extractRecentResponsesUserInputs() = %#v, want []string{%q}", got, want)
	}
	if last := extractLastResponsesUserInput(input); last != want {
		t.Fatalf("extractLastResponsesUserInput() = %q, want %q", last, want)
	}
	if count := countResponsesUserMessages(input); count != 1 {
		t.Fatalf("countResponsesUserMessages() = %d, want 1", count)
	}
}
