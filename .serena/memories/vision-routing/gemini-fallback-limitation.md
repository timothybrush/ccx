# Gemini 渠道 Vision Fallback 限制

Vision fallback 模型替换（`visionFallbackModel`）对 Gemini 渠道不生效。

## 原因

`TryUpstreamWithAllKeys` 中通过 `sjson.SetBytes(requestBody, "model", fallback)` 替换 model 字段，但 Gemini handler 的 `buildProviderRequest` 闭包使用的是独立的 `geminiReq` 结构体和外部 `model` 变量，不读取 `requestBody` 中的 model 字段。

参考：`backend-go/internal/handlers/gemini/handler.go:163-164`
```go
func(c *gin.Context, upstreamCopy *config.UpstreamConfig, apiKey string) (*http.Request, error) {
    return buildProviderRequest(c, upstreamCopy, upstreamCopy.BaseURL, apiKey, geminiReq, model, isStream)
}
```

## 影响

- 渠道级 `noVision=true` 正常工作（直接 failover，不涉及模型替换）
- `noVisionModels` + 无 fallback 正常工作（直接 failover）
- `noVisionModels` + `visionFallbackModel` 对 Gemini 渠道无效（替换不会传递到实际请求）

## 为什么可以接受

Gemini 3 系列模型全部支持多模态/vision，实际场景中不需要对 Gemini 渠道配置 vision fallback。如果未来需要支持，需要将 fallback model 传递到 Gemini handler 的 buildRequest 闭包中。
