package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/BenedictKing/ccx/internal/handlers/responses"
	"github.com/gin-gonic/gin"
)

func TestResponsesWebSocketPlainGetReturns405(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	handler := responses.WebSocketHandler(nil, nil, nil, nil)
	router.GET("/v1/responses", handler)
	router.GET("/:routePrefix/v1/responses", handler)

	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{
			name:       "root_plain_get_returns_405",
			path:       "/v1/responses",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "prefixed_plain_get_returns_405",
			path:       "/minimax/v1/responses",
			wantStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}
