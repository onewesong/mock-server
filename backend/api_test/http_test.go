package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"

	"mock-server/server"
)

type testEnv struct {
	server *httptest.Server
	dbClose func() error
}

func setupEnv(t *testing.T) testEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "mock.db")
	db, err := server.OpenDB(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	store, err := server.NewStore(db)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	router := server.NewRouter(store, server.Config{})
	server := httptest.NewServer(router)
	return testEnv{server: server, dbClose: db.Close}
}

func TestEndpointRuleFlow(t *testing.T) {
	env := setupEnv(t)
	defer env.server.Close()
	defer func() {
		_ = env.dbClose()
	}()

	endpointPayload := map[string]any{
		"name":        "用户详情",
		"method":      "GET",
		"pathPattern": "/users/:id",
		"enabled":     true,
		"tags":        []string{"user", "v1"},
		"description": "测试接口",
	}
	var createdEndpoint server.Endpoint
	postJSON(t, env.server.URL+"/__admin/api/endpoints", endpointPayload, &createdEndpoint)
	if createdEndpoint.ID == "" {
		t.Fatalf("endpoint id empty")
	}

	var endpoints []server.Endpoint
	getJSON(t, env.server.URL+"/__admin/api/endpoints", &endpoints)
	if len(endpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(endpoints))
	}

	updatePayload := map[string]any{
		"name":        "用户详情-更新",
		"method":      "GET",
		"pathPattern": "/users/:id",
		"enabled":     true,
		"tags":        []string{"user"},
		"description": "更新",
	}
	var updatedEndpoint server.Endpoint
	putJSON(t, env.server.URL+"/__admin/api/endpoints/"+createdEndpoint.ID, updatePayload, &updatedEndpoint)
	if updatedEndpoint.Name != "用户详情-更新" {
		t.Fatalf("update endpoint failed")
	}

	rulePayload := map[string]any{
		"name":     "命中测试",
		"enabled":  true,
		"priority": 0,
		"weight":   1,
		"matchers": []map[string]any{
			{
				"source":        "header",
				"key":           "X-Env",
				"op":            "eq",
				"value":         "test",
				"caseSensitive": false,
			},
		},
		"response": map[string]any{
			"status":   200,
			"headers":  map[string]string{"X-Mock": "yes"},
			"delayMs":  0,
			"bodyType": "json",
			"body":     "{\"ok\":true}",
		},
	}
	var createdRule server.Rule
	postJSON(t, env.server.URL+"/__admin/api/endpoints/"+createdEndpoint.ID+"/rules", rulePayload, &createdRule)
	if createdRule.ID == "" {
		t.Fatalf("rule id empty")
	}

	var rules []server.Rule
	getJSON(t, env.server.URL+"/__admin/api/endpoints/"+createdEndpoint.ID+"/rules", &rules)
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}

	previewPayload := map[string]any{
		"method": "GET",
		"path":   "/users/123",
		"query":  map[string]string{"debug": "1"},
		"headers": map[string]string{
			"X-Env": "test",
		},
		"body": "",
	}
	var previewResp server.PreviewResponse
	postJSON(t, env.server.URL+"/__admin/api/preview", previewPayload, &previewResp)
	if !previewResp.Matched || previewResp.RuleID == "" {
		t.Fatalf("preview not matched")
	}

	req, err := http.NewRequest("GET", env.server.URL+"/users/123", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("X-Env", "test")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("mock status %d", resp.StatusCode)
	}
	if resp.Header.Get("X-Mock") != "yes" {
		t.Fatalf("mock header missing")
	}

	deleteReq(t, env.server.URL+"/__admin/api/rules/"+createdRule.ID)
	deleteReq(t, env.server.URL+"/__admin/api/endpoints/"+createdEndpoint.ID)
}

func TestExportImport(t *testing.T) {
	env := setupEnv(t)
	defer env.server.Close()
	defer func() {
		_ = env.dbClose()
	}()

	endpointPayload := map[string]any{
		"name":        "导出测试",
		"method":      "POST",
		"pathPattern": "/orders",
		"enabled":     true,
		"tags":        []string{"order"},
		"description": "",
	}
	var createdEndpoint server.Endpoint
	postJSON(t, env.server.URL+"/__admin/api/endpoints", endpointPayload, &createdEndpoint)

	rulePayload := map[string]any{
		"name":     "默认",
		"enabled":  true,
		"priority": 0,
		"weight":   1,
		"matchers": []map[string]any{},
		"response": map[string]any{
			"status":   201,
			"headers":  map[string]string{},
			"delayMs":  0,
			"bodyType": "text",
			"body":     "created",
		},
	}
	var createdRule server.Rule
	postJSON(t, env.server.URL+"/__admin/api/endpoints/"+createdEndpoint.ID+"/rules", rulePayload, &createdRule)

	var bundle server.ExportBundle
	getJSON(t, env.server.URL+"/__admin/api/export", &bundle)
	if len(bundle.Endpoints) != 1 || len(bundle.Rules) != 1 {
		t.Fatalf("export bundle invalid")
	}

	deleteReq(t, env.server.URL+"/__admin/api/endpoints/"+createdEndpoint.ID)

	postJSON(t, env.server.URL+"/__admin/api/import", bundle, nil)
	var endpoints []server.Endpoint
	getJSON(t, env.server.URL+"/__admin/api/endpoints", &endpoints)
	if len(endpoints) != 1 {
		t.Fatalf("import endpoints missing")
	}
}

func getJSON(t *testing.T, url string, out any) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("get %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		t.Fatalf("get %s status %d", url, resp.StatusCode)
	}
	if out == nil {
		return
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		t.Fatalf("decode: %v", err)
	}
}

func postJSON(t *testing.T, url string, payload any, out any) {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		t.Fatalf("post %s status %d", url, resp.StatusCode)
	}
	if out == nil {
		return
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		t.Fatalf("decode: %v", err)
	}
}

func putJSON(t *testing.T, url string, payload any, out any) {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("put %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		t.Fatalf("put %s status %d", url, resp.StatusCode)
	}
	if out == nil {
		return
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		t.Fatalf("decode: %v", err)
	}
}

func deleteReq(t *testing.T, url string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		t.Fatalf("delete %s status %d", url, resp.StatusCode)
	}
}
