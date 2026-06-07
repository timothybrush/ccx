# Strip Image Generation Tool Toggle

## Context
User wants a per-channel toggle that strips `image_generation` from the request `tools` array before forwarding to upstream. This applies to the Responses API flow where `image_generation` is a Responses API-specific tool type.

## Naming & Storage Pattern Decision
- **Field name**: `StripImageGenerationTool` (Go), `stripImageGenerationTool` (JSON/TS)
- **Pattern**: Follow `NoVision` / `StripEmptyTextBlocks` -- simple `bool` with `omitempty`, default `false`
- **Rationale**: No nil-default-true semantics needed. The toggle should default to off (don't strip). Simpler than `*bool` helper pattern used by `CodexToolCompat`.
- UpstreamConfig uses `bool`, UpstreamUpdate uses `*bool` (standard partial-update pattern).

## Reusable Patterns Found
All boolean toggles follow identical code at 5 locations (messages, chat, responses, gemini, images config files):
```go
// UpstreamUpdate field uses *bool for tri-state nil detection
if updates.FieldName != nil {
    upstream.FieldName = *updates.FieldName  // for simple bool
}
// OR for *bool fields:
if updates.FieldName != nil {
    v := *updates.FieldName
    upstream.FieldName = &v
}
```
