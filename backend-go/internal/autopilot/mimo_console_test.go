package autopilot

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func TestMiMoConsoleClientVerify(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.Header.Get("Cookie") != "api-platform_serviceToken=session; userId=42" {
			t.Fatalf("Cookie header=%q", r.Header.Get("Cookie"))
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/userProfile":
			_, _ = w.Write([]byte(`{"code":0,"data":{"userId":"42"}}`))
		case "/api/v1/tokenPlan/detail":
			_, _ = w.Write([]byte(`{"code":0,"data":{"planCode":"max","planName":"Max","currentPeriodEnd":"2026-07-29 23:59:59","expired":false}}`))
		case "/api/v1/tokenPlan/usage":
			_, _ = w.Write([]byte(`{"code":0,"data":{"monthUsage":{"percent":0.25,"items":[{"used":25,"limit":100}]},"usage":{"percent":0.5,"items":[{"used":40,"limit":80},{"used":10,"limit":20}]}}}`))
		case "/api/v1/tokenPlan/apiKey/raw":
			_, _ = w.Write([]byte(`{"code":0,"data":"tp-cookie-key"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	now := time.Date(2026, 7, 12, 7, 0, 0, 0, time.Local)
	client := &MiMoConsoleClient{HTTPClient: server.Client(), BaseURL: server.URL, Now: func() time.Time { return now }}
	result, err := client.Verify(context.Background(), "Cookie: api-platform_serviceToken=session; userId=42")
	if err != nil {
		t.Fatal(err)
	}
	wantPaths := []string{"/api/v1/userProfile", "/api/v1/tokenPlan/detail", "/api/v1/tokenPlan/usage", "/api/v1/tokenPlan/apiKey/raw"}
	if !reflect.DeepEqual(paths, wantPaths) {
		t.Fatalf("paths=%v", paths)
	}
	if result.APIKey != "tp-cookie-key" || result.Snapshot.PlanCode != "max" || result.Snapshot.ValidatedAt != now {
		t.Fatalf("result=%+v", result)
	}
	if result.Snapshot.MonthUsage.Used != 25 || result.Snapshot.CurrentUsage.Used != 50 || result.Snapshot.CurrentUsage.Limit != 100 {
		t.Fatalf("usage=%+v", result.Snapshot)
	}
}

func TestMiMoConsoleClientRejectsIncompleteCookie(t *testing.T) {
	for _, cookie := range []string{
		"userId=42",
		"fake_api-platform_serviceToken=session; userId=42",
		"api-platform_serviceToken=; userId=42",
	} {
		if _, err := (&MiMoConsoleClient{}).Verify(context.Background(), cookie); err == nil {
			t.Fatalf("不完整 Cookie 应被拒绝: %q", cookie)
		}
	}
}
