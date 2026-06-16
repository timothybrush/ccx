package config

import "fmt"

const (
	MinRequestTimeoutMs        = 1000
	MaxRequestTimeoutMs        = 300000
	MinResponseHeaderTimeoutMs = 1000
	MaxResponseHeaderTimeoutMs = 300000
)

func validateRequestTimeoutMs(value int) error {
	if value == 0 {
		return nil
	}
	if value < MinRequestTimeoutMs || value > MaxRequestTimeoutMs {
		return fmt.Errorf("请求超时时间必须为 0（继承全局）或 %d-%d 之间", MinRequestTimeoutMs, MaxRequestTimeoutMs)
	}
	return nil
}

func validateResponseHeaderTimeoutMs(value int) error {
	if value == 0 {
		return nil
	}
	if value < MinResponseHeaderTimeoutMs || value > MaxResponseHeaderTimeoutMs {
		return fmt.Errorf("响应头等待超时必须为 0（继承全局）或 %d-%d 之间", MinResponseHeaderTimeoutMs, MaxResponseHeaderTimeoutMs)
	}
	return nil
}

func ValidateRuntimeRequestTimeoutMs(value int) error {
	if value < MinRequestTimeoutMs || value > MaxRequestTimeoutMs {
		return fmt.Errorf("requestTimeoutMs 必须在 %d-%d 之间", MinRequestTimeoutMs, MaxRequestTimeoutMs)
	}
	return nil
}

func ValidateRuntimeResponseHeaderTimeoutMs(value int) error {
	if value < MinResponseHeaderTimeoutMs || value > MaxResponseHeaderTimeoutMs {
		return fmt.Errorf("responseHeaderTimeoutMs 必须在 %d-%d 之间", MinResponseHeaderTimeoutMs, MaxResponseHeaderTimeoutMs)
	}
	return nil
}
