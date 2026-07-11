package autopilot

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/BenedictKing/ccx/internal/config"
)

func TestApplyVolcengineSignature(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "https://ark.cn-beijing.volces.com/?Version=2024-01-01&Action=ListArkAgentPlanModel", bytes.NewBufferString("{}"))
	applyVolcengineSignature(req, []byte("{}"), "AKIDTEST", "test-secret", "ark_stg", time.Date(2026, 4, 24, 12, 20, 3, 0, time.UTC))
	want := "HMAC-SHA256 Credential=AKIDTEST/20260424/cn-beijing/ark_stg/request, SignedHeaders=content-type;host;x-content-sha256;x-date, Signature=072882a505a7ea44d8b029876785a8a0dc4722899b95d18951b48f946e6d7589"
	if got := req.Header.Get("Authorization"); got != want {
		t.Fatalf("Authorization 签名不匹配\ngot:  %s\nwant: %s", got, want)
	}
	if got := req.Header.Get("X-Date"); got != "20260424T122003Z" {
		t.Fatalf("X-Date=%q", got)
	}
}

func TestVolcenginePlanClientDetectAndFetchModels(t *testing.T) {
	tests := []struct {
		name       string
		activePlan string
		wantAction string
	}{
		{name: "Agent Plan", activePlan: volcenginePlanAgent, wantAction: "ListArkAgentPlanModel"},
		{name: "Coding Plan", activePlan: volcenginePlanCoding, wantAction: "ListArkCodingPlanModel"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var modelAction string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !strings.Contains(r.Header.Get("Authorization"), "/cn-beijing/") {
					t.Errorf("缺少签名: %s", r.Header.Get("Authorization"))
				}
				switch action := r.URL.Query().Get("Action"); action {
				case "GetPersonalPlan":
					var body struct{ Plan string }
					_ = json.NewDecoder(r.Body).Decode(&body)
					matched := (body.Plan == "AgentPlan" && tt.activePlan == volcenginePlanAgent) ||
						(body.Plan == "CodingPlan" && tt.activePlan == volcenginePlanCoding)
					if !matched {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"ResponseMetadata":{"Error":{"Code":"ResourceNotFound.Plan","Message":"not found"}}}`))
						return
					}
					_, _ = w.Write([]byte(`{"Result":{"PlanType":"Pro","Status":"Running"}}`))
				case "ListArkAgentPlanModel", "ListArkCodingPlanModel":
					modelAction = action
					_, _ = w.Write([]byte(`{"Result":{"Datas":[{"ModelID":"model-b"},{"ModelID":"model-a"},{"ModelID":"model-a"}]}}`))
				default:
					t.Errorf("未知 action: %s", action)
					w.WriteHeader(http.StatusBadRequest)
				}
			}))
			defer server.Close()

			client := &volcenginePlanClient{Endpoint: server.URL, HTTPClient: server.Client()}
			pair := &config.VolcengineAccessKeyPair{AccessKeyID: "AKID", SecretAccessKey: "SECRET"}
			plan, err := client.DetectPlan(context.Background(), pair, "")
			if err != nil {
				t.Fatal(err)
			}
			if plan.Plan != tt.activePlan || plan.Tier != "Pro" || plan.Status != "Running" {
				t.Fatalf("plan=%+v", plan)
			}
			models, err := client.FetchModels(context.Background(), pair, plan.Plan)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Join(models, ",") != "model-a,model-b" || modelAction != tt.wantAction {
				t.Fatalf("models=%v action=%s", models, modelAction)
			}
		})
	}
}

func TestVolcenginePlanClientUsesBaseURLHintWhenBothPlansExist(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"Result":{"PlanType":"Large","Status":"Running"}}`))
	}))
	defer server.Close()
	client := &volcenginePlanClient{Endpoint: server.URL, HTTPClient: server.Client()}
	plan, err := client.DetectPlan(context.Background(), &config.VolcengineAccessKeyPair{AccessKeyID: "AK", SecretAccessKey: "SK"}, volcenginePlanCoding)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Plan != volcenginePlanCoding {
		t.Fatalf("plan=%+v", plan)
	}
}

func TestVolcengineAutoDiscoveryUsesControlPlaneModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("Action") {
		case "GetPersonalPlan":
			var body struct{ Plan string }
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.Plan != "AgentPlan" {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"ResponseMetadata":{"Error":{"Code":"ResourceNotFound.Plan","Message":"not found"}}}`))
				return
			}
			_, _ = w.Write([]byte(`{"Result":{"PlanType":"Large","Status":"Running"}}`))
		case "ListArkAgentPlanModel":
			_, _ = w.Write([]byte(`{"Result":{"Datas":[{"ModelID":"doubao-seed-code"},{"ModelID":"ark-code-latest"}]}}`))
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	data := `{
  "managedAccounts":[{"accountUid":"acct_volc","providerId":"volcengine","name":"volc","credentials":[{"credentialUid":"cred_volc","apiKey":"ark-inference","volcengineAccessKey":{"accessKeyId":"AKID","secretAccessKey":"SECRET"}}]}],
  "upstream":[{"accountUid":"acct_volc","channelUid":"ch_volc","providerId":"volcengine","name":"volc","serviceType":"claude","autoManaged":true,"baseUrl":"https://ark.cn-beijing.volces.com/api/plan","apiKeyConfigs":[{"credentialUid":"cred_volc","baseUrl":"https://ark.cn-beijing.volces.com/api/plan"}]}],
  "chatUpstream":[],"responsesUpstream":[],"geminiUpstream":[],"imagesUpstream":[],"vectorsUpstream":[]
}`
	if err := os.WriteFile(configPath, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	manager, err := config.NewConfigManager(configPath, filepath.Join(dir, "backups"))
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	channels := manager.GetAccountChannels("acct_volc")
	if len(channels) != 1 {
		t.Fatalf("channels=%d", len(channels))
	}
	runner := NewAutoDiscoveryRunner(nil, nil)
	runner.client = server.Client()
	runner.volcengineControlPlaneEndpoint = server.URL
	results := runner.discoverEndpoints(context.Background(), &channels[0].Upstream, manager)
	if len(results) != 1 || !results[0].ProtocolOk || strings.Join(results[0].Models, ",") != "ark-code-latest,doubao-seed-code" {
		t.Fatalf("results=%+v", results)
	}
	credential, ok := manager.GetManagedAccountCredential("acct_volc", "cred_volc")
	if !ok || credential.VolcengineAccessKey == nil || credential.VolcengineAccessKey.Plan != volcenginePlanAgent {
		t.Fatalf("套餐识别结果未持久化: %+v", credential)
	}
}
