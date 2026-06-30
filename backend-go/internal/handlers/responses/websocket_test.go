package responses

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/BenedictKing/ccx/internal/config"
	"github.com/BenedictKing/ccx/internal/metrics"
	"github.com/BenedictKing/ccx/internal/scheduler"
	"github.com/BenedictKing/ccx/internal/session"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func TestNormalizeWebSocketResponseCreatePayload(t *testing.T) {
	body, warmup, err := normalizeWebSocketResponseCreatePayload([]byte(`{
		"type":"response.create",
		"model":"gpt-5",
		"input":"hello",
		"stream":false,
		"generate":false,
		"client_metadata":{"x-codex-installation-id":"install"}
	}`))
	if err != nil {
		t.Fatalf("normalizeWebSocketResponseCreatePayload() err = %v", err)
	}
	if !warmup {
		t.Fatal("warmup = false, want true")
	}

	var got map[string]interface{}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal normalized body: %v", err)
	}
	for _, key := range []string{"type", "generate", "client_metadata"} {
		if _, ok := got[key]; ok {
			t.Fatalf("normalized body still contains %q: %s", key, string(body))
		}
	}
	if got["stream"] != true {
		t.Fatalf("stream = %v, want true", got["stream"])
	}
}

func TestResponsesWebSocketHandler_ForwardsSSEEventsAsWebSocketMessages(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstreamBody := make(chan map[string]interface{}, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read upstream request: %v", err)
		}
		var req map[string]interface{}
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("unmarshal upstream request: %v", err)
		}
		upstreamBody <- req

		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		_, _ = w.Write([]byte("event: response.created\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp-1\",\"status\":\"in_progress\",\"output\":[]}}\n\n"))
		_, _ = w.Write([]byte("event: response.output_text.delta\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"hi\"}\n\n"))
		_, _ = w.Write([]byte("event: response.completed\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp-1\",\"status\":\"completed\",\"output\":[{\"type\":\"message\",\"id\":\"msg-1\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"hi\"}]}],\"usage\":{\"input_tokens\":1,\"output_tokens\":1,\"total_tokens\":2}}}\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
	}))
	defer upstream.Close()

	router := newResponsesWebSocketTestRouter(t, config.UpstreamConfig{
		Name:        "responses-ws-test",
		BaseURL:     upstream.URL,
		APIKeys:     []string{"sk-test"},
		ServiceType: "responses",
		Status:      "active",
	})
	server := httptest.NewServer(router)
	defer server.Close()

	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/v1/responses", http.Header{
		"x-api-key": []string{"secret-key"},
	})
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	err = conn.WriteJSON(map[string]interface{}{
		"type":            "response.create",
		"model":           "gpt-5",
		"input":           "hello",
		"stream":          false,
		"generate":        true,
		"client_metadata": map[string]interface{}{"x-codex-installation-id": "install"},
	})
	if err != nil {
		t.Fatalf("write websocket request: %v", err)
	}

	created := readWebSocketJSON(t, conn)
	if created["type"] != "response.created" {
		t.Fatalf("first event type = %v, want response.created", created["type"])
	}
	delta := readWebSocketJSON(t, conn)
	if delta["type"] != "response.output_text.delta" {
		t.Fatalf("second event type = %v, want response.output_text.delta", delta["type"])
	}
	completed := readWebSocketJSON(t, conn)
	if completed["type"] != "response.completed" {
		t.Fatalf("third event type = %v, want response.completed", completed["type"])
	}

	select {
	case req := <-upstreamBody:
		for _, key := range []string{"type", "generate", "client_metadata"} {
			if _, ok := req[key]; ok {
				t.Fatalf("upstream request still contains %q: %#v", key, req)
			}
		}
		if req["stream"] != true {
			t.Fatalf("upstream stream = %v, want true", req["stream"])
		}
	case <-time.After(time.Second):
		t.Fatal("upstream did not receive request")
	}
}

func TestResponsesWebSocketHandler_WarmupDoesNotCallUpstream(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstreamCalled := make(chan struct{}, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalled <- struct{}{}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer upstream.Close()

	router := newResponsesWebSocketTestRouter(t, config.UpstreamConfig{
		Name:        "responses-ws-warmup-test",
		BaseURL:     upstream.URL,
		APIKeys:     []string{"sk-test"},
		ServiceType: "responses",
		Status:      "active",
	})
	server := httptest.NewServer(router)
	defer server.Close()

	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/v1/responses", http.Header{
		"x-api-key": []string{"secret-key"},
	})
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(map[string]interface{}{
		"type":     "response.create",
		"model":    "gpt-5",
		"input":    "hello",
		"generate": false,
	}); err != nil {
		t.Fatalf("write warmup request: %v", err)
	}

	_ = readWebSocketJSON(t, conn)
	completed := readWebSocketJSON(t, conn)
	if completed["type"] != "response.completed" {
		t.Fatalf("warmup terminal event type = %v, want response.completed", completed["type"])
	}
	response, _ := completed["response"].(map[string]interface{})
	if response["id"] != "" {
		t.Fatalf("warmup response id = %v, want empty", response["id"])
	}

	select {
	case <-upstreamCalled:
		t.Fatal("warmup unexpectedly called upstream")
	case <-time.After(100 * time.Millisecond):
	}
}

func newResponsesWebSocketTestRouter(t *testing.T, upstream config.UpstreamConfig) *gin.Engine {
	t.Helper()
	cfgManager := setupResponsesTestConfigManager(t, []config.UpstreamConfig{upstream})
	channelScheduler := scheduler.NewChannelScheduler(
		cfgManager,
		metrics.NewMetricsManager(),
		metrics.NewMetricsManager(),
		metrics.NewMetricsManager(),
		metrics.NewMetricsManager(),
		metrics.NewMetricsManager(),
		session.NewTraceAffinityManager(),
		nil,
	)
	envCfg := &config.EnvConfig{
		ProxyAccessKey:     "secret-key",
		MaxRequestBodySize: 1024 * 1024,
	}

	r := gin.New()
	r.GET("/v1/responses", WebSocketHandler(envCfg, cfgManager, session.NewSessionManager(time.Hour, 100, 100000), channelScheduler))
	return r
}

func readWebSocketJSON(t *testing.T, conn *websocket.Conn) map[string]interface{} {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var event map[string]interface{}
	if err := conn.ReadJSON(&event); err != nil {
		t.Fatalf("read websocket json: %v", err)
	}
	return event
}
