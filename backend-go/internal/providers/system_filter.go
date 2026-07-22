package providers

import (
	"regexp"
	"strings"
)

// SystemHeaderFilterLevel 定义 system header 过滤层级
type SystemHeaderFilterLevel int

const (
	// LevelNoFilter 不过滤（上游需要 billing header 计费）
	LevelNoFilter SystemHeaderFilterLevel = 0
	// LevelCCIdentity 仅过滤 CC identity 和 subagent header（上游需要 billing header 但不需要 CC identity）
	LevelCCIdentity SystemHeaderFilterLevel = 1
	// LevelAllCCHeaders 过滤所有 CC 注入的 header（包括 billing header）
	LevelAllCCHeaders SystemHeaderFilterLevel = 2
	// LevelFirstBlock 过滤第一条 system block（如果存在）
	LevelFirstBlock SystemHeaderFilterLevel = 3
)

// billingHeaderPattern 匹配完整的 billing header 行
var billingHeaderPattern = regexp.MustCompile(`^x-anthropic-billing-header:.*$`)

// ccIdentityPatterns 匹配 CC identity 和 subagent header
var ccIdentityPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^You are Claude Code, Anthropic's official CLI for Claude\.`),
	regexp.MustCompile(`^You are a Claude agent, built on Anthropic's Claude Agent SDK\.`),
	regexp.MustCompile(`^You are an? .+ (?:specialist|agent) for Claude Code`),
}

// isBillingHeader 判断是否为 billing header
func isBillingHeader(text string) bool {
	return billingHeaderPattern.MatchString(strings.TrimSpace(text))
}

// isCCIdentityHeader 判断是否为 CC identity 或 subagent header
func isCCIdentityHeader(text string) bool {
	text = strings.TrimSpace(text)
	for _, pattern := range ccIdentityPatterns {
		if pattern.MatchString(text) {
			return true
		}
	}
	return false
}

// FilterSystemHeader 根据过滤层级过滤 system header
func FilterSystemHeader(system interface{}, level SystemHeaderFilterLevel) interface{} {
	arr, ok := system.([]interface{})
	if !ok {
		return system
	}

	if level == LevelNoFilter {
		return system // 不过滤
	}

	newSystem := make([]interface{}, 0, len(arr))
	for i, item := range arr {
		obj, ok := item.(map[string]interface{})
		if !ok {
			newSystem = append(newSystem, item)
			continue
		}

		text, ok := obj["text"].(string)
		if !ok {
			newSystem = append(newSystem, item)
			continue
		}

		// Level 3: 跳过第一条
		if level >= LevelFirstBlock && i == 0 {
			continue
		}

		// Level 2: 跳过 billing header
		if level >= LevelAllCCHeaders && isBillingHeader(text) {
			continue
		}

		// Level 1: 跳过 CC identity 和 subagent header
		if level >= LevelCCIdentity && isCCIdentityHeader(text) {
			continue
		}

		newSystem = append(newSystem, item)
	}

	return newSystem
}
