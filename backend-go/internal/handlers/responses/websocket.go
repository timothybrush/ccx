package responses

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/BenedictKing/ccx/internal/config"
	"github.com/BenedictKing/ccx/internal/scheduler"
	"github.com/BenedictKing/ccx/internal/session"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const (
	responsesWebSocketWriteTimeout = 15 * time.Second
	responsesWebSocketReadLimit    = 32 << 20
)

var responsesWebSocketUpgrader = websocket.Upgrader{
	CheckOrigin: func(*http.Request) bool { return true },
}

// WebSocketHandler 支持 Codex 原生 Responses WebSocket 协议。
// Codex 发送的 response.create payload 与 HTTP Responses 请求基本一致，
// 这里将其转为内部 POST /v1/responses，再把 SSE data 事件逐帧写回 WebSocket。
func WebSocketHandler(
	envCfg *config.EnvConfig,
	cfgManager *config.ConfigManager,
	sessionManager *session.SessionManager,
	channelScheduler *scheduler.ChannelScheduler,
) gin.HandlerFunc {
	httpHandler := Handler(envCfg, cfgManager, sessionManager, channelScheduler)
	bridgeRouter := gin.New()
	bridgeRouter.POST("/v1/responses", httpHandler)
	bridgeRouter.POST("/:routePrefix/v1/responses", httpHandler)

	return gin.HandlerFunc(func(c *gin.Context) {
		if !isWebSocketUpgrade(c.Request) {
			c.Status(http.StatusMethodNotAllowed)
			return
		}

		conn, err := responsesWebSocketUpgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		conn.SetReadLimit(responsesWebSocketReadLimit)

		for {
			messageType, payload, err := conn.ReadMessage()
			if err != nil {
				if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					closeWithWebSocketError(conn, http.StatusBadRequest, "websocket_read_error", err.Error())
				}
				return
			}
			if messageType != websocket.TextMessage {
				closeWithWebSocketError(conn, http.StatusBadRequest, "invalid_message_type", "only text messages are supported")
				return
			}

			requestBody, warmup, err := normalizeWebSocketResponseCreatePayload(payload)
			if err != nil {
				writeWebSocketError(conn, http.StatusBadRequest, "invalid_request", err.Error())
				continue
			}
			if warmup {
				if err := writeWebSocketWarmupResponse(conn); err != nil {
					return
				}
				continue
			}

			if err := serveResponseCreateOverWebSocket(c, conn, bridgeRouter, requestBody); err != nil {
				if isWebSocketClosed(err) {
					return
				}
				writeWebSocketError(conn, http.StatusInternalServerError, "stream_error", err.Error())
			}
		}
	})
}

func isWebSocketUpgrade(r *http.Request) bool {
	return headerContainsToken(r.Header, "Connection", "upgrade") &&
		headerContainsToken(r.Header, "Upgrade", "websocket")
}

func headerContainsToken(header http.Header, key, token string) bool {
	for _, value := range header.Values(key) {
		for _, part := range strings.Split(value, ",") {
			if strings.EqualFold(strings.TrimSpace(part), token) {
				return true
			}
		}
	}
	return false
}

func normalizeWebSocketResponseCreatePayload(payload []byte) ([]byte, bool, error) {
	var req map[string]interface{}
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, false, fmt.Errorf("解析 response.create 失败: %w", err)
	}

	msgType, _ := req["type"].(string)
	if msgType != "response.create" {
		return nil, false, fmt.Errorf("不支持的 WebSocket 消息类型: %s", msgType)
	}
	warmup := req["generate"] == false

	delete(req, "type")
	delete(req, "client_metadata")
	delete(req, "generate")
	req["stream"] = true

	body, err := json.Marshal(req)
	if err != nil {
		return nil, false, fmt.Errorf("序列化 response.create 失败: %w", err)
	}
	return body, warmup, nil
}

func serveResponseCreateOverWebSocket(
	parent *gin.Context,
	conn *websocket.Conn,
	httpHandler http.Handler,
	requestBody []byte,
) error {
	req, err := http.NewRequestWithContext(
		parent.Request.Context(),
		http.MethodPost,
		parent.Request.URL.RequestURI(),
		bytes.NewReader(requestBody),
	)
	if err != nil {
		return err
	}
	copyWebSocketRequestHeaders(parent.Request.Header, req.Header)
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = int64(len(requestBody))

	writer := newResponsesWebSocketWriter(conn)
	httpHandler.ServeHTTP(writer, req)
	return writer.finish()
}

func copyWebSocketRequestHeaders(src http.Header, dst http.Header) {
	for key, values := range src {
		switch strings.ToLower(key) {
		case "connection", "upgrade", "sec-websocket-key", "sec-websocket-version", "sec-websocket-extensions", "sec-websocket-protocol":
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

type responsesWebSocketWriter struct {
	conn        *websocket.Conn
	header      http.Header
	status      int
	size        int
	written     bool
	writeErr    error
	mu          sync.Mutex
	sseBuffer   bytes.Buffer
	currentData strings.Builder
}

func newResponsesWebSocketWriter(conn *websocket.Conn) *responsesWebSocketWriter {
	return &responsesWebSocketWriter{
		conn:   conn,
		header: make(http.Header),
		status: http.StatusOK,
		size:   -1,
	}
}

func (w *responsesWebSocketWriter) Header() http.Header {
	return w.header
}

func (w *responsesWebSocketWriter) WriteHeader(statusCode int) {
	if statusCode <= 0 {
		return
	}
	if w.Written() && w.status != statusCode {
		return
	}
	w.status = statusCode
	if statusCode >= http.StatusBadRequest {
		w.writeErr = writeWebSocketError(w.conn, statusCode, "upstream_error", http.StatusText(statusCode))
	}
}

func (w *responsesWebSocketWriter) WriteHeaderNow() {
	w.written = true
	if w.size < 0 {
		w.size = 0
	}
}

func (w *responsesWebSocketWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.writeErr != nil {
		return 0, w.writeErr
	}
	w.WriteHeaderNow()
	w.size += len(data)
	if w.status >= http.StatusBadRequest {
		return len(data), nil
	}

	w.sseBuffer.Write(data)
	w.flushSSEFrames(false)
	if w.writeErr != nil {
		return 0, w.writeErr
	}
	return len(data), nil
}

func (w *responsesWebSocketWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

func (w *responsesWebSocketWriter) Status() int {
	return w.status
}

func (w *responsesWebSocketWriter) Size() int {
	return w.size
}

func (w *responsesWebSocketWriter) Written() bool {
	return w.written
}

func (w *responsesWebSocketWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.WriteHeaderNow()
	w.flushSSEFrames(false)
}

func (w *responsesWebSocketWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, errors.New("websocket response writer does not support hijack")
}

func (w *responsesWebSocketWriter) CloseNotify() <-chan bool {
	ch := make(chan bool)
	return ch
}

func (w *responsesWebSocketWriter) Pusher() http.Pusher {
	return nil
}

func (w *responsesWebSocketWriter) finish() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.flushSSEFrames(true)
	return w.writeErr
}

func (w *responsesWebSocketWriter) flushSSEFrames(final bool) {
	for {
		line, ok := takeSSELine(&w.sseBuffer)
		if !ok {
			break
		}
		w.processSSELine(line)
		if w.writeErr != nil {
			return
		}
	}
	if final && w.sseBuffer.Len() > 0 {
		w.processSSELine(w.sseBuffer.String())
		w.sseBuffer.Reset()
	}
	if final && strings.TrimSpace(w.currentData.String()) != "" {
		w.sendCurrentData()
	}
}

func (w *responsesWebSocketWriter) processSSELine(line string) {
	line = strings.TrimRight(line, "\r")
	if line == "" {
		w.sendCurrentData()
		return
	}
	if strings.HasPrefix(line, ":") || strings.HasPrefix(line, "event:") {
		return
	}
	if !strings.HasPrefix(line, "data:") {
		return
	}

	data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
	if data == "" || data == "[DONE]" {
		return
	}
	if w.currentData.Len() > 0 {
		w.currentData.WriteByte('\n')
	}
	w.currentData.WriteString(data)
}

func (w *responsesWebSocketWriter) sendCurrentData() {
	data := strings.TrimSpace(w.currentData.String())
	w.currentData.Reset()
	if data == "" || w.writeErr != nil {
		return
	}

	_ = w.conn.SetWriteDeadline(time.Now().Add(responsesWebSocketWriteTimeout))
	w.writeErr = w.conn.WriteMessage(websocket.TextMessage, []byte(data))
}

func takeSSELine(buf *bytes.Buffer) (string, bool) {
	data := buf.Bytes()
	for i, b := range data {
		if b == '\n' {
			lineBytes := make([]byte, i)
			copy(lineBytes, data[:i])
			buf.Next(i + 1)
			return string(lineBytes), true
		}
	}
	return "", false
}

func writeWebSocketError(conn *websocket.Conn, status int, code, message string) error {
	payload, err := json.Marshal(gin.H{
		"type":        "error",
		"status_code": status,
		"error": gin.H{
			"code":    code,
			"message": message,
		},
	})
	if err != nil {
		return err
	}
	_ = conn.SetWriteDeadline(time.Now().Add(responsesWebSocketWriteTimeout))
	return conn.WriteMessage(websocket.TextMessage, payload)
}

func writeWebSocketWarmupResponse(conn *websocket.Conn) error {
	created := gin.H{
		"type": "response.created",
		"response": gin.H{
			"id":     "",
			"status": "in_progress",
			"output": []interface{}{},
		},
	}
	completed := gin.H{
		"type": "response.completed",
		"response": gin.H{
			"id":       "",
			"status":   "completed",
			"output":   []interface{}{},
			"end_turn": false,
			"usage": gin.H{
				"input_tokens":  0,
				"output_tokens": 0,
				"total_tokens":  0,
			},
		},
	}
	for _, event := range []gin.H{created, completed} {
		payload, err := json.Marshal(event)
		if err != nil {
			return err
		}
		_ = conn.SetWriteDeadline(time.Now().Add(responsesWebSocketWriteTimeout))
		if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
			return err
		}
	}
	return nil
}

func closeWithWebSocketError(conn *websocket.Conn, status int, code, message string) {
	_ = writeWebSocketError(conn, status, code, message)
	closePayload := websocket.FormatCloseMessage(websocket.CloseUnsupportedData, message)
	_ = conn.WriteControl(websocket.CloseMessage, closePayload, time.Now().Add(responsesWebSocketWriteTimeout))
}

func isWebSocketClosed(err error) bool {
	return websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) ||
		errors.Is(err, net.ErrClosed)
}
