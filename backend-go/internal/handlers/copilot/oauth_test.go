package copilot

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	corecopilot "github.com/BenedictKing/ccx/internal/copilot"
	"github.com/gin-gonic/gin"
)

func TestRequestDeviceCodeSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/device/code") {
			t.Fatalf("path = %q, want /device/code", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"device_code":"dev-1","user_code":"USER-1","verification_uri":"https://github.com/login/device","expires_in":900,"interval":5}`))
	}))
	defer upstream.Close()
	withOAuthClient(t, upstream.Client(), upstream.URL+"/device/code", "", "")

	w := performRequestDeviceCode()

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"userCode":"USER-1"`) {
		t.Fatalf("body = %s", w.Body.String())
	}
}

func TestRequestDeviceCodeTimesOut(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_, _ = w.Write([]byte(`{"device_code":"late"}`))
	}))
	defer upstream.Close()
	withOAuthClient(t, upstream.Client(), upstream.URL+"/device/code", "", "")
	oldTimeout := oauthRequestTimeout
	oauthRequestTimeout = 30 * time.Millisecond
	t.Cleanup(func() {
		oauthRequestTimeout = oldTimeout
	})

	start := time.Now()
	w := performRequestDeviceCode()

	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("request took %s, want quick timeout", elapsed)
	}
	if !strings.Contains(w.Body.String(), "GitHub OAuth") {
		t.Fatalf("body = %s", w.Body.String())
	}
}

func withOAuthClient(t *testing.T, httpClient *http.Client, deviceCodeURL, accessTokenURL, userURL string) {
	t.Helper()
	oldFactory := newOAuthClient
	newOAuthClient = func(client *http.Client) *corecopilot.OAuthClient {
		oauth := corecopilot.NewOAuthClient(httpClient)
		oauth.DeviceCodeURL = deviceCodeURL
		oauth.AccessTokenURL = accessTokenURL
		oauth.UserURL = userURL
		return oauth
	}
	t.Cleanup(func() {
		newOAuthClient = oldFactory
	})
}

func performRequestDeviceCode() *httptest.ResponseRecorder {
	router := gin.New()
	router.POST("/api/copilot/oauth/device/code", RequestDeviceCode())
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/copilot/oauth/device/code", nil)
	router.ServeHTTP(w, req)
	return w
}
