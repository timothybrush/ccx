package autopilot

import (
	"encoding/json"
	"fmt"

	"github.com/BenedictKing/ccx/internal/config"
	"github.com/BenedictKing/ccx/internal/presetstore"
)

// applyProviderPreset 将指定 collection 下某 provider 的预设字段应用到 upstream。
//
// 用于 provider 模板化添加：后端在创建渠道时直接 apply 预设（modelMapping / reasoningMapping /
// passbackReasoningContent / noVisionModels / visionFallbackModel / rateLimitRpm / 各兼容开关等），
// 无需依赖前端 apply，避免前后端两套逻辑漂移。
//
// 实现方式：预设 JSON 的字段名与 UpstreamConfig 的 json tag 一致，
// 直接把 provider 预设的原始 JSON unmarshal 到目标 upstream 之上。
// 预设 JSON 中不含 baseUrl/apiKeys/name/serviceType 等键，故这些已设值不会被覆盖。
//
// collection 形如 "claudeMessages"；其原始结构为 {"providers": {"<providerID>": {...预设字段...}}}。
// 若 collection/provider 不存在，返回 nil（无预设可用不视为错误，渠道仍可创建）。
func applyProviderPreset(bundle *presetstore.PresetBundle, collection, providerID string, upstream *config.UpstreamConfig) error {
	if bundle == nil || bundle.ChannelPresets == nil || upstream == nil {
		return nil
	}
	raw, ok := bundle.ChannelPresets.Collections[collection]
	if !ok || len(raw) == 0 {
		return nil
	}

	// collection 结构：{"schemaVersion":1,"providers":{...}} 或 {"presets":{...}}
	var wrapper struct {
		Providers map[string]json.RawMessage `json:"providers"`
		Presets   map[string]json.RawMessage `json:"presets"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return fmt.Errorf("[Autopilot-Preset] 解析 collection %q 失败: %w", collection, err)
	}

	providerRaw, ok := wrapper.Providers[providerID]
	if !ok {
		providerRaw, ok = wrapper.Presets[providerID]
	}
	if !ok || len(providerRaw) == 0 {
		return nil
	}

	// 预设字段名与 UpstreamConfig json tag 一致，直接叠加到目标 upstream。
	// json.Unmarshal 只写入 JSON 中出现的键，不影响 upstream 已设的 baseUrl/apiKeys/name 等。
	if err := json.Unmarshal(providerRaw, upstream); err != nil {
		return fmt.Errorf("[Autopilot-Preset] 应用 provider %q 预设失败: %w", providerID, err)
	}
	return nil
}
