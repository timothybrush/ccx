package config

import "sync/atomic"

var runtimeTimeouts atomic.Pointer[RuntimeTimeouts]

// RuntimeTimeouts 保存可热更新的全局上游请求超时参数（毫秒）。
type RuntimeTimeouts struct {
	RequestTimeoutMs        int
	ResponseHeaderTimeoutMs int
}

func SetRuntimeTimeouts(requestTimeoutMs int, responseHeaderTimeoutMs int) {
	runtimeTimeouts.Store(&RuntimeTimeouts{
		RequestTimeoutMs:        clampInt(requestTimeoutMs, MinRequestTimeoutMs, MaxRequestTimeoutMs),
		ResponseHeaderTimeoutMs: clampInt(responseHeaderTimeoutMs, MinResponseHeaderTimeoutMs, MaxResponseHeaderTimeoutMs),
	})
}

func GetRuntimeRequestTimeoutMs(fallbackMs int) int {
	if current := runtimeTimeouts.Load(); current != nil && current.RequestTimeoutMs > 0 {
		return current.RequestTimeoutMs
	}
	return clampInt(fallbackMs, MinRequestTimeoutMs, MaxRequestTimeoutMs)
}

func GetRuntimeResponseHeaderTimeoutMs(fallbackMs int) int {
	if current := runtimeTimeouts.Load(); current != nil && current.ResponseHeaderTimeoutMs > 0 {
		return current.ResponseHeaderTimeoutMs
	}
	return clampInt(fallbackMs, MinResponseHeaderTimeoutMs, MaxResponseHeaderTimeoutMs)
}
