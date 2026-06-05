package common

import (
	"log"
	"time"

	"github.com/BenedictKing/ccx/internal/utils"
)

// StreamProgressLogger 记录流式响应的进度，便于定位首字后吞吐下降或断流。
// TPS 为估算值：文本路径使用 token 估算，纯透传路径按字节做保守估算。
type StreamProgressLogger struct {
	component   string
	enabled     bool
	startTime   time.Time
	lastLogTime time.Time

	totalTokens int
	totalChars  int
	lastTokens  int
	lastChars   int
}

func NewStreamProgressLogger(component string, startTime time.Time, enabled bool) *StreamProgressLogger {
	if startTime.IsZero() {
		startTime = time.Now()
	}
	return &StreamProgressLogger{
		component:   component,
		enabled:     enabled,
		startTime:   startTime,
		lastLogTime: startTime,
	}
}

func (l *StreamProgressLogger) AddText(text string) {
	if l == nil || !l.enabled || text == "" {
		return
	}
	tokens := utils.EstimateTokens(text)
	if tokens <= 0 {
		tokens = 1
	}
	l.totalTokens += tokens
	l.totalChars += len([]rune(text))
}

func (l *StreamProgressLogger) AddBytes(n int) {
	if l == nil || !l.enabled || n <= 0 {
		return
	}
	// 纯透传场景无法稳定解析内容，按 4 bytes/token 做保守估算。
	tokens := n / 4
	if tokens <= 0 {
		tokens = 1
	}
	l.totalTokens += tokens
	l.totalChars += n
}

func (l *StreamProgressLogger) Tick() {
	if l == nil || !l.enabled {
		return
	}
	now := time.Now()
	elapsed := now.Sub(l.lastLogTime)
	if elapsed < time.Second {
		return
	}
	l.log(now, "progress")
}

func (l *StreamProgressLogger) Finish(status string) {
	if l == nil || !l.enabled {
		return
	}
	l.log(time.Now(), status)
}

func (l *StreamProgressLogger) log(now time.Time, status string) {
	windowSeconds := now.Sub(l.lastLogTime).Seconds()
	if windowSeconds <= 0 {
		windowSeconds = 1
	}
	elapsedSeconds := now.Sub(l.startTime).Seconds()
	if elapsedSeconds <= 0 {
		elapsedSeconds = 1
	}

	windowTokens := l.totalTokens - l.lastTokens
	windowChars := l.totalChars - l.lastChars
	windowTPS := float64(windowTokens) / windowSeconds
	avgTPS := float64(l.totalTokens) / elapsedSeconds

	log.Printf("[%s-Stream-Progress] status=%s elapsed=%dms windowTPS=%.2f avgTPS=%.2f windowTokens=%d totalTokens=%d windowChars=%d totalChars=%d",
		l.component,
		status,
		int(now.Sub(l.startTime).Milliseconds()),
		windowTPS,
		avgTPS,
		windowTokens,
		l.totalTokens,
		windowChars,
		l.totalChars,
	)

	l.lastLogTime = now
	l.lastTokens = l.totalTokens
	l.lastChars = l.totalChars
}
