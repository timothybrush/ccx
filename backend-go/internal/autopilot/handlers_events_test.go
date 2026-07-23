package autopilot

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/BenedictKing/ccx/internal/errutil"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func newTestManagerForHandlers(t *testing.T) *Manager {
	t.Helper()
	db := newTestDB(t)
	changelogStore, err := NewProfileChangelogStoreWithDB(db)
	if err != nil {
		t.Fatalf("创建 ProfileChangelogStore 失败: %v", err)
	}
	return &Manager{
		changelogStore: changelogStore,
		eventHub:       NewEventHub(),
	}
}

// ── REST /health-center/changelog ──

func TestHandleChangelog_DefaultLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mgr := newTestManagerForHandlers(t)
	for i := 0; i < 5; i++ {
		mgr.ChangelogStore().Record(ProfileChangeEvent{ChannelUID: "ch-1", EventType: EventTypeHealthChanged})
	}

	router := gin.New()
	RegisterRoutes(router, mgr)

	req := httptest.NewRequest(http.MethodGet, "/health-center/changelog", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, want 200", w.Code)
	}
}

func TestHandleChangelog_LimitClampedTo200(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mgr := newTestManagerForHandlers(t)

	router := gin.New()
	RegisterRoutes(router, mgr)

	req := httptest.NewRequest(http.MethodGet, "/health-center/changelog?limit=99999", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, want 200", w.Code)
	}
}

func TestHandleChangelog_FilterByChannelUID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mgr := newTestManagerForHandlers(t)
	mgr.ChangelogStore().Record(ProfileChangeEvent{ChannelUID: "ch-a", EventType: EventTypeHealthChanged})
	mgr.ChangelogStore().Record(ProfileChangeEvent{ChannelUID: "ch-b", EventType: EventTypeHealthChanged})

	router := gin.New()
	RegisterRoutes(router, mgr)

	req := httptest.NewRequest(http.MethodGet, "/health-center/changelog?channelUid=ch-a", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"ch-a"`) {
		t.Errorf("响应应包含 ch-a: %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), `"ch-b"`) {
		t.Errorf("响应不应包含 ch-b: %s", w.Body.String())
	}
}

func TestHandleChangelog_NilChangelogStore_ReturnsEmpty(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mgr := &Manager{} // changelogStore 为 nil

	router := gin.New()
	RegisterRoutes(router, mgr)

	req := httptest.NewRequest(http.MethodGet, "/health-center/changelog", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"total":0`) {
		t.Errorf("nil changelogStore 应返回空列表: %s", w.Body.String())
	}
}

// ── WebSocket /health-center/events ──

func TestHandleChangelogEvents_ConnectReceiveDisconnect(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mgr := newTestManagerForHandlers(t)

	router := gin.New()
	RegisterRoutes(router, mgr)
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/health-center/events"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket 连接失败: %v", err)
	}
	defer errutil.IgnoreDeferred(conn.Close)

	// 等一小段时间确保 handler 内部已完成 hub.Subscribe()，再发布事件
	time.Sleep(50 * time.Millisecond)

	mgr.EventHub().Publish(ProfileChangeEvent{
		ChannelUID: "ch-ws-test",
		EventType:  EventTypeHealthChanged,
		Summary:    "healthy → degraded",
	})

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var received ProfileChangeEvent
	if err := conn.ReadJSON(&received); err != nil {
		t.Fatalf("读取事件失败: %v", err)
	}
	if received.ChannelUID != "ch-ws-test" {
		t.Errorf("ChannelUID = %q, want ch-ws-test", received.ChannelUID)
	}
	if received.EventType != EventTypeHealthChanged {
		t.Errorf("EventType = %q, want %q", received.EventType, EventTypeHealthChanged)
	}
}

// TestHandleChangelogEvents_EchoesSecWebSocketProtocol 验证浏览器原生 WebSocket
// 通过 Sec-WebSocket-Protocol 传入的子协议（承载 API key）会被服务端原样回显，
// 满足部分浏览器要求握手响应包含已选定子协议的行为（Phase 3A 鉴权基础）。
func TestHandleChangelogEvents_EchoesSecWebSocketProtocol(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mgr := newTestManagerForHandlers(t)

	router := gin.New()
	RegisterRoutes(router, mgr)
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/health-center/events"
	dialer := *websocket.DefaultDialer
	dialer.Subprotocols = []string{"my-secret-key"}
	conn, resp, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket 连接失败: %v", err)
	}
	defer errutil.IgnoreDeferred(conn.Close)

	if got := resp.Header.Get("Sec-WebSocket-Protocol"); got != "my-secret-key" {
		t.Errorf("响应应回显子协议 my-secret-key, got %q", got)
	}
	if conn.Subprotocol() != "my-secret-key" {
		t.Errorf("conn.Subprotocol() = %q, want my-secret-key", conn.Subprotocol())
	}
}

func TestHandleChangelogEvents_UnsubscribesOnDisconnect(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mgr := newTestManagerForHandlers(t)

	router := gin.New()
	RegisterRoutes(router, mgr)
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/health-center/events"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket 连接失败: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	if mgr.EventHub().SubscriberCount() != 1 {
		t.Fatalf("连接后应有 1 个订阅者，got %d", mgr.EventHub().SubscriberCount())
	}
	_ = conn.Close()

	// 服务端检测到断开需要一点时间
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if mgr.EventHub().SubscriberCount() == 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("断开后应取消订阅，got %d 个订阅者", mgr.EventHub().SubscriberCount())
}

func TestHandleChangelogEvents_NilEventHub_ReturnsServiceUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mgr := &Manager{} // eventHub 为 nil

	router := gin.New()
	RegisterRoutes(router, mgr)
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/health-center/events"
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err == nil {
		t.Fatal("nil EventHub 时不应成功升级为 WebSocket")
	}
	if resp == nil || resp.StatusCode != http.StatusServiceUnavailable {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		t.Errorf("状态码 = %d, want 503", status)
	}
}
