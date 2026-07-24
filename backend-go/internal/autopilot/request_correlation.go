package autopilot

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

type requestCorrelationIDKey struct{}

// ContextWithRequestCorrelationID 将服务端逻辑请求关联 ID 写入 context。
// 在入站请求最早处生成一次，贯穿所有渠道尝试和 trace。
func ContextWithRequestCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestCorrelationIDKey{}, id)
}

// RequestCorrelationIDFromContext 返回当前请求的逻辑关联 ID。
func RequestCorrelationIDFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	id, ok := ctx.Value(requestCorrelationIDKey{}).(string)
	return id, ok
}

// GenerateRequestCorrelationID 生成碰撞安全的请求关联 ID。
func GenerateRequestCorrelationID() string {
	var buf [12]byte
	if _, err := rand.Read(buf[:]); err != nil {
		// crypto/rand 失败在正常运行时几乎不可能发生
		fallback := hex.EncodeToString([]byte("corr"))
		return "corr_" + fallback
	}
	return "corr_" + hex.EncodeToString(buf[:])
}
