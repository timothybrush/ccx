package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestResponsesWebSocketFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/v1/responses", responsesWebSocketFallback)
	router.GET("/:routePrefix/v1/responses", responsesWebSocketFallback)

	tests := []struct {
		name       string
		path       string
		upgrade    string
		wantStatus int
	}{
		{
			name:       "websocket_upgrade_returns_426",
			path:       "/v1/responses",
			upgrade:    "websocket",
			wantStatus: http.StatusUpgradeRequired,
		},
		{
			name:       "websocket_upgrade_is_case_insensitive",
			path:       "/v1/responses",
			upgrade:    "WebSocket",
			wantStatus: http.StatusUpgradeRequired,
		},
		{
			name:       "prefixed_websocket_upgrade_returns_426",
			path:       "/minimax/v1/responses",
			upgrade:    "websocket",
			wantStatus: http.StatusUpgradeRequired,
		},
		{
			name:       "plain_get_returns_405",
			path:       "/v1/responses",
			wantStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			if tt.upgrade != "" {
				req.Header.Set("Upgrade", tt.upgrade)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}
