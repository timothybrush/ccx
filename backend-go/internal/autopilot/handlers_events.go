package autopilot

import (
	"net/http"
	"strconv"
	"time"

	"github.com/BenedictKing/ccx/internal/errutil"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// ── Phase 3A：画像变更事件 API（只读展示，不影响调度）──

// ChangelogResponse GET /api/health-center/changelog 返回结构。
type ChangelogResponse struct {
	Events []*ProfileChangeEvent `json:"events"`
	Total  int                   `json:"total"`
}

// handleChangelog GET /api/health-center/changelog
// 查询参数：
//   - limit=N        返回最近 N 条（默认 50，最大 200）
//   - channelUid=xxx  只返回指定渠道的变更记录
func handleChangelog(mgr *Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		if mgr == nil || mgr.ChangelogStore() == nil {
			c.JSON(http.StatusOK, ChangelogResponse{Events: []*ProfileChangeEvent{}, Total: 0})
			return
		}

		limit := 50
		if limitStr := c.Query("limit"); limitStr != "" {
			if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
				limit = parsed
			}
		}
		if limit > 200 {
			limit = 200
		}

		var events []*ProfileChangeEvent
		if channelUID := c.Query("channelUid"); channelUID != "" {
			events = mgr.ChangelogStore().ListByChannel(channelUID, limit)
		} else {
			events = mgr.ChangelogStore().ListRecent(limit)
		}

		c.JSON(http.StatusOK, ChangelogResponse{Events: events, Total: len(events)})
	}
}

// changelogEventsUpgrader 用于 /health-center/events 的 WebSocket 升级。
// 同源部署，CORS 已由 middleware.CORSMiddleware 统一处理。鉴权由挂载在整个
// gin engine 上的 middleware.WebAuthMiddleware 覆盖（该中间件先于 apiGroup 注册，
// 对 /api/* 下的所有路由生效，包括本 WS upgrade 请求）：由于浏览器原生 WebSocket
// API 无法设置自定义请求头，前端把 key 作为 Sec-WebSocket-Protocol 子协议传入，
// middleware.getAPIKey 已支持从该 header 回退读取 key，在到达本 handler 前完成校验。
var changelogEventsUpgrader = websocket.Upgrader{
	CheckOrigin: func(*http.Request) bool { return true },
}

const changelogEventsWriteTimeout = 15 * time.Second

// handleChangelogEvents GET /api/health-center/events（WebSocket）
// 推送 ProfileChangeEvent：profile_updated / health_changed /
// discovery_completed / auto_mapping_applied。纯只读广播，不接收/处理客户端消息，
// 不影响调度。事件本身已脱敏（不含明文 API Key / Authorization / prompt）。
func handleChangelogEvents(mgr *Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		if mgr == nil || mgr.EventHub() == nil {
			c.Status(http.StatusServiceUnavailable)
			return
		}

		// 若客户端通过 Sec-WebSocket-Protocol 传了子协议（即 API key），
		// 原样在握手响应中回显，避免部分浏览器因未选定子协议而拒绝连接。
		// 此时 key 已在 WebAuthMiddleware 中校验通过，这里只做协议回显，不重复鉴权。
		var respHeader http.Header
		if proto := c.GetHeader("Sec-WebSocket-Protocol"); proto != "" {
			respHeader = http.Header{"Sec-WebSocket-Protocol": []string{proto}}
		}

		conn, err := changelogEventsUpgrader.Upgrade(c.Writer, c.Request, respHeader)
		if err != nil {
			return
		}
		defer errutil.IgnoreDeferred(conn.Close)

		ch, unsubscribe := mgr.EventHub().Subscribe()
		defer unsubscribe()

		// 只读连接：起一个 goroutine 消费/丢弃客户端消息，仅用于检测连接关闭。
		closed := make(chan struct{})
		go func() {
			defer close(closed)
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					return
				}
			}
		}()

		for {
			select {
			case <-closed:
				return
			case ev, ok := <-ch:
				if !ok {
					return
				}
				_ = conn.SetWriteDeadline(time.Now().Add(changelogEventsWriteTimeout))
				if err := conn.WriteJSON(ev); err != nil {
					return
				}
			}
		}
	}
}
