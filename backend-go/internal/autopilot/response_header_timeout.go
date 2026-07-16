package autopilot

import "time"

const (
	adaptiveResponseHeaderMinSamples = 20
	adaptiveResponseHeaderFloorMs    = 30_000
	adaptiveResponseHeaderPaddingMs  = 5_000
	adaptiveResponseHeaderMultiplier = 4
	adaptiveFirstByteStatsMaxAge     = time.Hour
)

// SuggestResponseHeaderTimeout 根据成功请求的 TTFB 画像，为明确的轻任务给出保守响应头超时。
// 返回 0 表示样本不足、任务不适合自适应，或建议值无法缩短当前继承值。
func SuggestResponseHeaderTimeout(profile *KeyEndpointProfile, request *RequestProfile, inheritedMs int, isStream bool) int {
	if profile == nil || request == nil || inheritedMs <= adaptiveResponseHeaderFloorMs {
		return 0
	}
	if request.TaskClass != TaskClassLightweight || request.HasImage || request.ToolUseNeed || request.ReasoningNeed || request.ContextNeed >= 10_000 {
		return 0
	}
	// 非流式响应通常在整段生成完成后才返回 Header；仅对天然有界的小操作缩短等待。
	// 流式请求在 Header 后由独立的首内容/空闲超时接管，因此不受输出长度影响。
	if !isStream && !isBoundedNonStreamOperation(request.Operation) {
		return 0
	}
	if profile.FirstByteSampleCount < adaptiveResponseHeaderMinSamples || profile.P95FirstByteLatencyMs <= 0 {
		return 0
	}
	if !isFirstByteStatsFresh(profile.FirstByteStatsUpdatedAt, time.Now()) {
		return 0
	}
	if profile.P95FirstByteLatencyMs >= int64(inheritedMs) {
		return 0
	}

	suggestedMs := profile.P95FirstByteLatencyMs*adaptiveResponseHeaderMultiplier + adaptiveResponseHeaderPaddingMs
	if suggestedMs < adaptiveResponseHeaderFloorMs {
		suggestedMs = adaptiveResponseHeaderFloorMs
	}
	if suggestedMs >= int64(inheritedMs) {
		return 0
	}
	return int(suggestedMs)
}

func isFirstByteStatsFresh(updatedAt *time.Time, now time.Time) bool {
	if updatedAt == nil {
		return false
	}
	age := now.Sub(*updatedAt)
	return age >= 0 && age <= adaptiveFirstByteStatsMaxAge
}

func isBoundedNonStreamOperation(operation string) bool {
	switch operation {
	case "count_tokens", "title_generation", "classification", "format_conversion":
		return true
	default:
		return false
	}
}

func buildResponseHeaderTimeoutLookup(store *ProfileStore, request *RequestProfile) func(endpointUID string, inheritedMs int, isStream bool) int {
	if store == nil || request == nil {
		return nil
	}
	requestCopy := *request
	return func(endpointUID string, inheritedMs int, isStream bool) int {
		if endpointUID == "" {
			return 0
		}
		return SuggestResponseHeaderTimeout(store.Get(endpointUID), &requestCopy, inheritedMs, isStream)
	}
}
