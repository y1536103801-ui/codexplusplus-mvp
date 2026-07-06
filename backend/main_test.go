package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type testClient struct {
	t           *testing.T
	handler     http.Handler
	app         *App
	store       *Store
	adminToken  string
	clientToken string
}

func newTestClient(t *testing.T) *testClient {
	t.Helper()
	t.Setenv("CODEXPPP_DATABASE_URL", "")
	installTestCodexRun(t)
	path := filepath.Join(t.TempDir(), "state.json")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	app := &App{store: store, secretKey: deriveKey("test-secret")}
	return &testClient{t: t, handler: app.routes(), app: app, store: store}
}

func installTestCodexRun(t *testing.T) {
	t.Helper()
	originalRun := codexAppServerRun
	codexAppServerRun = func(ctx context.Context, credentials codexProbeCredentials, requestBody []byte) (codexRunResult, error) {
		const prefix = "test-upstream-url:"
		if !strings.HasPrefix(credentials.AccessToken, prefix) {
			return codexRunResult{}, fmt.Errorf("codex_app_server_start_failed")
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimPrefix(credentials.AccessToken, prefix), bytes.NewReader(requestBody))
		if err != nil {
			return codexRunResult{}, fmt.Errorf("codex_app_server_request_invalid")
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer access")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return codexRunResult{}, fmt.Errorf("codex_app_server_unavailable")
		}
		defer res.Body.Close()
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			return codexRunResult{}, fmt.Errorf("codex_app_server_http_%d", res.StatusCode)
		}
		var payload any
		dec := json.NewDecoder(res.Body)
		dec.UseNumber()
		if err := dec.Decode(&payload); err != nil {
			return codexRunResult{}, fmt.Errorf("codex_app_server_invalid_json")
		}
		usage, ok := extractGatewayUsage(payload)
		if !ok {
			return codexRunResult{}, fmt.Errorf("codex_app_server_usage_missing")
		}
		return codexRunResult{
			Model:  stringField(payload, "model"),
			Text:   textFromAny(resultField(payload)),
			Usage:  usage,
			Status: res.StatusCode,
		}, nil
	}
	t.Cleanup(func() { codexAppServerRun = originalRun })
}

type fakeGatewayRateLimiter struct {
	allow bool
	err   error
	calls int
}

func (f *fakeGatewayRateLimiter) Allow(ctx context.Context, key string, limit int64, window time.Duration) (bool, error) {
	f.calls++
	if f.err != nil {
		return false, f.err
	}
	return f.allow, nil
}

func (c *testClient) request(method, path string, token string, body any, out any) int {
	return c.requestWithHeaders(method, path, token, nil, body, out)
}

func (c *testClient) requestRaw(method, path string, token string, raw string, out any) int {
	return c.requestRawWithHeaders(method, path, token, nil, raw, out)
}

func (c *testClient) requestRawWithHeaders(method, path string, token string, headers map[string]string, raw string, out any) int {
	c.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rr := httptest.NewRecorder()
	c.handler.ServeHTTP(rr, req)
	res := rr.Result()
	defer res.Body.Close()
	if out != nil {
		_ = json.NewDecoder(res.Body).Decode(out)
	}
	return res.StatusCode
}

func (c *testClient) requestWithHeaders(method, path string, token string, headers map[string]string, body any, out any) int {
	c.t.Helper()
	var payload bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&payload).Encode(body); err != nil {
			c.t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &payload)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rr := httptest.NewRecorder()
	c.handler.ServeHTTP(rr, req)
	res := rr.Result()
	defer res.Body.Close()
	if out != nil {
		_ = json.NewDecoder(res.Body).Decode(out)
	}
	return res.StatusCode
}

func (c *testClient) gatewayRun(token string, body any, out any) int {
	return c.gatewayRunWithHeaders(token, nil, body, out)
}

func (c *testClient) gatewayRunWithHeaders(token string, headers map[string]string, body any, out any) int {
	c.t.Helper()
	var payload bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&payload).Encode(body); err != nil {
			c.t.Fatal(err)
		}
	}
	return c.gatewayRunRawWithHeaders(token, headers, payload.String(), out)
}

func (c *testClient) gatewayRunRawWithHeaders(token string, headers map[string]string, raw string, out any) int {
	c.t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/gateway/runs", strings.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rr := httptest.NewRecorder()
	c.app.gatewayRun(rr, req)
	res := rr.Result()
	defer res.Body.Close()
	if out != nil {
		_ = json.NewDecoder(res.Body).Decode(out)
	}
	return res.StatusCode
}

func (c *testClient) setupAdmin() {
	var setup struct {
		Token string `json:"token"`
	}
	if code := c.request(http.MethodPost, "/api/admin/setup", "", map[string]any{"account": "root", "password": ""}, &setup); code != http.StatusOK {
		c.t.Fatalf("setup status = %d", code)
	}
	c.adminToken = setup.Token
	if code := c.request(http.MethodPost, "/api/admin/password", c.adminToken, map[string]any{"password": ""}, nil); code != http.StatusOK {
		c.t.Fatalf("password change status = %d", code)
	}
}

func (c *testClient) createUser(account string) {
	if code := c.request(http.MethodPost, "/api/admin/users", c.adminToken, map[string]any{"account": account, "password": ""}, nil); code != http.StatusCreated {
		c.t.Fatalf("create user status = %d", code)
	}
}

func (c *testClient) loginClient(account string) {
	var login struct {
		Token string         `json:"token"`
		User  map[string]any `json:"user"`
	}
	body := map[string]any{"account": account, "password": "", "deviceName": "Windows 设备", "fingerprint": account + "-device"}
	if code := c.request(http.MethodPost, "/api/client/login", "", body, &login); code != http.StatusOK {
		c.t.Fatalf("client login status = %d", code)
	}
	if login.User["account"] != account || login.User["tokenBalance"] == nil || login.User["recentRechargeStatus"] == nil {
		c.t.Fatalf("client login user = %#v", login.User)
	}
	for _, key := range []string{"id", "status", "lastLoginAt", "createdAt"} {
		if _, ok := login.User[key]; ok {
			c.t.Fatalf("client login leaked user field %q: %#v", key, login.User)
		}
	}
	c.clientToken = login.Token
}

type launchPrepareResponse struct {
	LaunchState string `json:"launchState"`
	Provider    struct {
		BearerToken string `json:"bearerToken"`
	} `json:"provider"`
	Diagnostics map[string]string `json:"diagnostics"`
}

func (c *testClient) prepareCodexProvider() launchPrepareResponse {
	c.t.Helper()
	var prepare launchPrepareResponse
	if code := c.request(http.MethodPost, "/api/client/launch/prepare", c.clientToken, map[string]any{}, &prepare); code != http.StatusOK {
		c.t.Fatalf("launch prepare status = %d", code)
	}
	if prepare.LaunchState != "ready" || prepare.Provider.BearerToken == "" {
		c.t.Fatalf("launch prepare response = %#v", prepare)
	}
	if !isSub2APIStyleKey(prepare.Provider.BearerToken) {
		c.t.Fatalf("launch prepare provider key has invalid API-key shape")
	}
	if _, ok := prepare.Diagnostics["codexAccount"]; ok {
		c.t.Fatalf("launch prepare must not expose provider account as local Codex account: %#v", prepare)
	}
	return prepare
}

func (c *testClient) prepareCodexProviderToken() string {
	c.t.Helper()
	prepare := c.prepareCodexProvider()
	return prepare.Provider.BearerToken
}

func TestAdminPasswordAllowsEmpty(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	var login struct {
		Token string `json:"token"`
	}
	if code := c.request(http.MethodPost, "/api/admin/login", "", map[string]any{"account": "root", "password": ""}, &login); code != http.StatusOK {
		t.Fatalf("empty password login status = %d", code)
	}
	if login.Token == "" {
		t.Fatal("expected admin token")
	}
}

func TestBackendSecretRequiresExplicitSecretForDatabase(t *testing.T) {
	if secret, err := backendSecret("", ""); err != nil || secret != defaultDevSecret {
		t.Fatalf("local default secret = %q err=%v", secret, err)
	}
	if _, err := backendSecret("", "postgres://example"); err == nil {
		t.Fatal("expected missing database secret error")
	}
	if _, err := backendSecret(defaultDevSecret, "postgres://example"); err == nil {
		t.Fatal("expected default database secret error")
	}
	if _, err := backendSecret("replace-with-a-long-random-secret", "postgres://example"); err == nil {
		t.Fatal("expected deployment placeholder secret error")
	}
	if _, err := backendSecret("<set-a-long-random-secret>", "postgres://example"); err == nil {
		t.Fatal("expected documented placeholder secret error")
	}
	if secret, err := backendSecret("production-secret", "postgres://example"); err != nil || secret != "production-secret" {
		t.Fatalf("database secret = %q err=%v", secret, err)
	}
}

func TestPostgresMigrationCoversOperationalSchema(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("migrations", "001_init.sql"))
	if err != nil {
		t.Fatal(err)
	}
	schema := string(body)
	for _, required := range []string{
		"CREATE TABLE IF NOT EXISTS users",
		"CREATE TABLE IF NOT EXISTS devices",
		"CREATE TABLE IF NOT EXISTS token_topups",
		"CREATE TABLE IF NOT EXISTS recharge_requests",
		"status_transitions JSONB NOT NULL DEFAULT '[]'::jsonb",
		"CREATE TABLE IF NOT EXISTS token_ledgers",
		"CREATE TABLE IF NOT EXISTS upstream_accounts",
		"access_token_cipher TEXT NOT NULL DEFAULT ''",
		"refresh_token_cipher TEXT NOT NULL DEFAULT ''",
		"credential_fingerprint TEXT NOT NULL DEFAULT ''",
		"CREATE TABLE IF NOT EXISTS api_keys",
		"last_used_at TIMESTAMPTZ",
		"CREATE TABLE IF NOT EXISTS usage_records",
		"CREATE TABLE IF NOT EXISTS audit_logs",
		"actor_role IN ('admin', 'client', 'system')",
		"CREATE TABLE IF NOT EXISTS sessions",
		"role IN ('admin', 'client', 'codex')",
		"CREATE TABLE IF NOT EXISTS idempotency_records",
		"request_id TEXT NOT NULL",
		"reserved_tokens BIGINT NOT NULL",
		"charged_tokens BIGINT NOT NULL",
		"usage_record_id TEXT NOT NULL DEFAULT ''",
		"result_text TEXT NOT NULL DEFAULT ''",
		"UNIQUE(user_id, request_id)",
	} {
		if !strings.Contains(schema, required) {
			t.Fatalf("postgres migration missing %q", required)
		}
	}
	if !strings.Contains(schema, "DROP COLUMN IF EXISTS run_url_cipher") {
		t.Fatal("postgres migration must actively remove retired run_url_cipher column")
	}
}

func TestListenAddrDefaultsToLoopback(t *testing.T) {
	if addr := listenAddrFromEnv(""); addr != defaultListenAddr {
		t.Fatalf("default listen addr = %q", addr)
	}
	if addr := listenAddrFromEnv("   "); addr != defaultListenAddr {
		t.Fatalf("blank listen addr = %q", addr)
	}
	if addr := listenAddrFromEnv(":8787"); addr != ":8787" {
		t.Fatalf("explicit listen addr = %q", addr)
	}
}

func TestListenDisplayURLUsesLocalhostForWildcard(t *testing.T) {
	tests := map[string]string{
		"127.0.0.1:8787": "http://127.0.0.1:8787",
		":8787":          "http://localhost:8787",
		"0.0.0.0:8787":   "http://localhost:8787",
		"[::]:8787":      "http://localhost:8787",
	}
	for addr, want := range tests {
		if got := listenDisplayURL(addr); got != want {
			t.Fatalf("display URL for %q = %q, want %q", addr, got, want)
		}
	}
}

func TestHealthEndpointIsLivenessOnly(t *testing.T) {
	c := newTestClient(t)
	var out map[string]any
	if code := c.request(http.MethodGet, "/api/health", "", nil, &out); code != http.StatusOK {
		t.Fatalf("health status = %d", code)
	}
	if len(out) != 1 || out["status"] != "ok" {
		t.Fatalf("health payload exposes more than liveness: %#v", out)
	}
	var errOut apiError
	if code := c.request(http.MethodPost, "/api/health", "", map[string]any{}, &errOut); code != http.StatusNotFound {
		t.Fatalf("health post status = %d", code)
	}
	if errOut.Error != "not_found" {
		t.Fatalf("health post error = %q", errOut.Error)
	}
}

func TestDockerComposeOperationalStackKeepsDockerPortsLoopbackForPortproxy(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "deploy", "docker-compose.yml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, required := range []string{
		"backend:",
		"context: ../backend",
		"CODEXPPP_DATABASE_URL: postgres://codexppp:codexppp@postgres:5432/codexppp?sslmode=disable",
		"CODEXPPP_REDIS_ADDR: redis:6379",
		"CODEXPPP_CODEX_COMMAND: ${CODEXPPP_CODEX_COMMAND:-codex}",
		"CODEXPPP_DESKTOP_LATEST_VERSION: ${CODEXPPP_DESKTOP_LATEST_VERSION:-}",
		"CODEXPPP_DESKTOP_DOWNLOAD_URL: ${CODEXPPP_DESKTOP_DOWNLOAD_URL:-}",
		"CODEXPPP_DESKTOP_DOWNLOAD_SHA256: ${CODEXPPP_DESKTOP_DOWNLOAD_SHA256:-}",
		"CODEXPPP_DESKTOP_RELEASE_NOTES: ${CODEXPPP_DESKTOP_RELEASE_NOTES:-}",
		"127.0.0.1:8787:8787",
		"curl -fsS http://127.0.0.1:8787/api/health >/dev/null",
		"127.0.0.1:54329:5432",
		"127.0.0.1:63799:6379",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("compose operational stack missing %q", required)
		}
	}
	for _, forbidden := range []string{
		`- "8787:8787"`,
		`- "54329:5432"`,
		`- "63799:6379"`,
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("compose exposes Docker port on all interfaces instead of relying on Windows portproxy: %q", forbidden)
		}
	}

	dockerfile, err := os.ReadFile("Dockerfile")
	if err != nil {
		t.Fatal(err)
	}
	image := string(dockerfile)
	for _, required := range []string{
		"ca-certificates curl git openssh-client",
		"npm install -g @openai/codex",
		"ENV CODEXPPP_CODEX_COMMAND=codex",
		`CMD ["codexppp-backend"]`,
	} {
		if !strings.Contains(image, required) {
			t.Fatalf("backend Dockerfile missing %q", required)
		}
	}
}

func TestWindowsDeployStartScriptKeepsGeneratedSecretLocal(t *testing.T) {
	ignore, err := os.ReadFile(filepath.Join("..", ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ignore), "deploy/.env") {
		t.Fatal("deploy/.env must be ignored so generated deployment secrets are never committed")
	}

	example, err := os.ReadFile(filepath.Join("..", "deploy", ".env.example"))
	if err != nil {
		t.Fatal(err)
	}
	exampleText := string(example)
	for _, required := range []string{
		"CODEXPPP_SECRET=",
		"CODEXPPP_CLIENT_ORIGINS=",
		"CODEXPPP_GATEWAY_RATE_LIMIT_PER_MINUTE=120",
		"CODEXPPP_REDIS_DB=0",
		"CODEXPPP_REDIS_PASSWORD=",
		"CODEXPPP_CODEX_COMMAND=codex",
		"CODEXPPP_DESKTOP_LATEST_VERSION=",
		"CODEXPPP_DESKTOP_DOWNLOAD_URL=",
		"CODEXPPP_DESKTOP_DOWNLOAD_SHA256=",
		"CODEXPPP_DESKTOP_RELEASE_NOTES=",
	} {
		if !strings.Contains(exampleText, required) {
			t.Fatalf("deploy env example missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"local-docker-validation-secret",
		"replace-with-a-long-random-secret",
		"CODEXPPP_SECRET=local",
	} {
		if strings.Contains(exampleText, forbidden) {
			t.Fatalf("deploy env example leaks or hardcodes secret material via %q", forbidden)
		}
	}

	body, err := os.ReadFile(filepath.Join("..", "deploy", "start.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, required := range []string{
		"RandomNumberGenerator",
		"New-CodexEnvTemplate",
		"Get-CodexEnvSecret",
		"Set-CodexEnvSecret",
		"CODEXPPP_SECRET=$Secret",
		"New-Object System.Text.UTF8Encoding -ArgumentList $false",
		"Updated deploy/.env with a generated CODEXPPP_SECRET.",
		"docker compose --env-file $EnvFile -f $ComposeFile up -d --build",
		"docker compose --env-file $EnvFile -f $ComposeFile ps",
		"Keep this file stable for this deployment.",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("deploy start script missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"replace-with-a-long-random-secret",
		"Write-Host $secret",
		"CODEXPPP_SECRET=local",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("deploy start script leaks or hardcodes secret material via %q", forbidden)
		}
	}
}

func TestBackendDoesNotServeOrdinaryUserWebClient(t *testing.T) {
	c := newTestClient(t)
	for _, path := range []string{
		"/client",
		"/client/",
		"/client/login",
		"/desktop-client/ui/index.html",
		"/desktop-client/ui/",
	} {
		if code := c.request(http.MethodGet, path, "", nil, nil); code != http.StatusNotFound {
			t.Fatalf("ordinary-user web path %s status = %d", path, code)
		}
	}
	if code := c.request(http.MethodGet, "/admin", "", nil, nil); code != http.StatusOK {
		t.Fatalf("admin console status = %d", code)
	}
}

func TestCORSAllowsDefaultTauriOrigin(t *testing.T) {
	c := newTestClient(t)
	req := httptest.NewRequest(http.MethodOptions, "/api/client/me", nil)
	req.Header.Set("Origin", "http://tauri.localhost")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rr := httptest.NewRecorder()

	c.handler.ServeHTTP(rr, req)

	res := rr.Result()
	defer res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("preflight status = %d", res.StatusCode)
	}
	if got := res.Header.Get("Access-Control-Allow-Origin"); got != "http://tauri.localhost" {
		t.Fatalf("allow origin = %q", got)
	}
	if got := res.Header.Get("Access-Control-Allow-Headers"); !strings.Contains(got, "Authorization") || !strings.Contains(got, "Idempotency-Key") {
		t.Fatalf("allow headers = %q", got)
	}
}

func TestCORSAllowsConfiguredOriginAndNormalizesPath(t *testing.T) {
	origins, err := corsOriginsFromEnv("https://client.example.com/app")
	if err != nil {
		t.Fatal(err)
	}
	handler := withCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}), origins)
	req := httptest.NewRequest(http.MethodGet, "/api/client/me", nil)
	req.Header.Set("Origin", "https://client.example.com")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	res := rr.Result()
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("configured origin status = %d", res.StatusCode)
	}
	if got := res.Header.Get("Access-Control-Allow-Origin"); got != "https://client.example.com" {
		t.Fatalf("configured allow origin = %q", got)
	}
}

func TestCORSRejectsUnconfiguredAPIOrigin(t *testing.T) {
	c := newTestClient(t)
	var out apiError
	req := httptest.NewRequest(http.MethodGet, "/api/client/me", nil)
	req.Header.Set("Origin", "https://evil.example")
	rr := httptest.NewRecorder()

	c.handler.ServeHTTP(rr, req)

	res := rr.Result()
	defer res.Body.Close()
	_ = json.NewDecoder(res.Body).Decode(&out)
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("unconfigured origin status = %d", res.StatusCode)
	}
	if out.Error != "origin_not_allowed" {
		t.Fatalf("unconfigured origin error = %q", out.Error)
	}
	if got := res.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("unexpected allow origin = %q", got)
	}
}

func TestCORSRejectsMalformedAPIOrigin(t *testing.T) {
	c := newTestClient(t)
	req := httptest.NewRequest(http.MethodGet, "/api/client/me", nil)
	req.Header.Set("Origin", "http://tauri.localhost/app")
	rr := httptest.NewRecorder()

	c.handler.ServeHTTP(rr, req)

	res := rr.Result()
	defer res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("malformed origin status = %d", res.StatusCode)
	}
	if got := res.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("unexpected allow origin = %q", got)
	}
}

func TestCORSAllowsSameOriginAdmin(t *testing.T) {
	c := newTestClient(t)
	req := httptest.NewRequest(http.MethodOptions, "http://codex.example/api/admin/bootstrap", nil)
	req.Header.Set("Origin", "http://codex.example")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rr := httptest.NewRecorder()

	c.handler.ServeHTTP(rr, req)

	res := rr.Result()
	defer res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("same-origin status = %d", res.StatusCode)
	}
	if got := res.Header.Get("Access-Control-Allow-Origin"); got != "http://codex.example" {
		t.Fatalf("same-origin allow origin = %q", got)
	}
}

func TestRedisGatewayRateLimitConfigParsing(t *testing.T) {
	if limit, err := gatewayRateLimitFromEnv(""); err != nil || limit != defaultGatewayRateLimitPerMinute {
		t.Fatalf("default rate limit = %d err=%v", limit, err)
	}
	if limit, err := gatewayRateLimitFromEnv("0"); err != nil || limit != 0 {
		t.Fatalf("disabled rate limit = %d err=%v", limit, err)
	}
	if _, err := gatewayRateLimitFromEnv("-1"); err == nil {
		t.Fatal("expected negative rate limit error")
	}
	if db, err := redisDBFromEnv(""); err != nil || db != 0 {
		t.Fatalf("default redis db = %d err=%v", db, err)
	}
	if db, err := redisDBFromEnv("2"); err != nil || db != 2 {
		t.Fatalf("redis db = %d err=%v", db, err)
	}
	if _, err := redisDBFromEnv("-1"); err == nil {
		t.Fatal("expected negative redis db error")
	}
}

func TestDesktopUpdateVersionHelpers(t *testing.T) {
	if !versionGreater("0.2.0", "0.1.9") {
		t.Fatal("expected 0.2.0 to be newer than 0.1.9")
	}
	if versionGreater("0.1.0", "0.1.0") {
		t.Fatal("same version should not be newer")
	}
	if versionGreater("0.1.0", "0.2.0") {
		t.Fatal("older version should not be newer")
	}
	if versionGreater("latest", "0.1.0") {
		t.Fatal("non-numeric version should not be treated as newer")
	}
	if got := safeHTTPURL(" file:///tmp/codex.exe "); got != "" {
		t.Fatalf("unsafe update url = %q", got)
	}
	if got := safeHTTPURL(" https://example.com/codex.exe "); got != "https://example.com/codex.exe" {
		t.Fatalf("safe update url = %q", got)
	}
}

func TestClientDesktopUpdateEndpoint(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("update-user")
	c.loginClient("update-user")
	t.Setenv("CODEXPPP_DESKTOP_LATEST_VERSION", "")
	t.Setenv("CODEXPPP_DESKTOP_DOWNLOAD_URL", "")
	t.Setenv("CODEXPPP_DESKTOP_DOWNLOAD_SHA256", "")
	t.Setenv("CODEXPPP_DESKTOP_RELEASE_NOTES", "")

	var noUpdate map[string]any
	if code := c.request(http.MethodGet, "/api/client/desktop/update?currentVersion=0.1.0", c.clientToken, nil, &noUpdate); code != http.StatusOK {
		t.Fatalf("desktop update status = %d", code)
	}
	if noUpdate["available"] != false || noUpdate["currentVersion"] != "0.1.0" || noUpdate["latestVersion"] != "0.1.0" {
		t.Fatalf("no update payload = %#v", noUpdate)
	}

	t.Setenv("CODEXPPP_DESKTOP_LATEST_VERSION", "0.2.0")
	t.Setenv("CODEXPPP_DESKTOP_DOWNLOAD_URL", "https://example.com/codexppp-0.2.0.exe")
	t.Setenv("CODEXPPP_DESKTOP_DOWNLOAD_SHA256", "abc123")
	t.Setenv("CODEXPPP_DESKTOP_RELEASE_NOTES", "修复启动流程")
	var update map[string]any
	if code := c.request(http.MethodGet, "/api/client/desktop/update?currentVersion=0.1.0", c.clientToken, nil, &update); code != http.StatusOK {
		t.Fatalf("desktop update available status = %d", code)
	}
	if update["available"] != true || update["latestVersion"] != "0.2.0" || update["downloadUrl"] != "https://example.com/codexppp-0.2.0.exe" || update["sha256"] != "abc123" || update["releaseNotes"] != "修复启动流程" {
		t.Fatalf("update payload = %#v", update)
	}

	t.Setenv("CODEXPPP_DESKTOP_DOWNLOAD_URL", "file:///tmp/codexppp.exe")
	var unsafeURL map[string]any
	if code := c.request(http.MethodGet, "/api/client/desktop/update?currentVersion=0.1.0", c.clientToken, nil, &unsafeURL); code != http.StatusOK {
		t.Fatalf("desktop update unsafe url status = %d", code)
	}
	if unsafeURL["available"] != true || unsafeURL["downloadUrl"] != "" {
		t.Fatalf("unsafe update url leaked = %#v", unsafeURL)
	}
}

func TestCodexModelsEndpoint(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("codex-models")
	c.loginClient("codex-models")
	c.approvePaidRecharge()
	c.createRoutedUpstream("")
	providerToken := c.prepareCodexProviderToken()
	var out struct {
		Object string           `json:"object"`
		Data   []map[string]any `json:"data"`
	}
	if code := c.request(http.MethodGet, "/api/codex/v1/models", providerToken, nil, &out); code != http.StatusOK {
		t.Fatalf("models status = %d", code)
	}
	if out.Object != "list" || len(out.Data) != 1 || out.Data[0]["id"] != codexDefaultModel {
		t.Fatalf("unexpected models payload: %#v", out)
	}
}

func TestClientServiceAvailableWithUsableUpstreamWithoutManualAPIKey(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("service-auto-route")
	c.loginClient("service-auto-route")
	c.approvePaidRecharge()

	var upstream struct {
		ID string `json:"id"`
	}
	body := map[string]any{"name": "auto-route-upstream", "credentialType": "oauth", "accessToken": "access", "chatgptAccountId": "account-123", "email": "pool@example.com", "subscriptionTier": "pro"}
	if code := c.request(http.MethodPost, "/api/admin/upstreams", c.adminToken, body, &upstream); code != http.StatusCreated {
		t.Fatalf("upstream status = %d", code)
	}
	if len(c.store.state.APIKeys) != 0 {
		t.Fatalf("test requires no manual api keys: %#v", c.store.state.APIKeys)
	}
	prepare := c.prepareCodexProvider()
	providerToken := prepare.Provider.BearerToken

	var me struct {
		Service map[string]any `json:"service"`
	}
	if code := c.request(http.MethodGet, "/api/client/me", c.clientToken, nil, &me); code != http.StatusOK {
		t.Fatalf("me status = %d", code)
	}
	if me.Service["status"] != "可用" {
		t.Fatalf("service status = %#v", me.Service)
	}
	for _, key := range []string{"currentAccount", "currentPlan", "accessToken", "refreshToken", "accessTokenCipher", "refreshTokenCipher", "credentialFingerprint"} {
		if _, ok := me.Service[key]; ok {
			t.Fatalf("service leaked upstream field %q: %#v", key, me.Service)
		}
	}

	var models struct {
		Data []map[string]any `json:"data"`
	}
	if code := c.request(http.MethodGet, "/api/codex/v1/models", providerToken, nil, &models); code != http.StatusOK {
		t.Fatalf("models status = %d", code)
	}
}

func TestCodexModelsRequiresClientAuth(t *testing.T) {
	c := newTestClient(t)
	var out struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if code := c.request(http.MethodGet, "/api/codex/v1/models", "", nil, &out); code != http.StatusUnauthorized {
		t.Fatalf("models auth status = %d", code)
	}
	if out.Error.Code != "login_failed" || out.Error.Type != "codexppp_error" || out.Error.Message != "登录失败，请重新登录" {
		t.Fatalf("models auth error = %#v", out.Error)
	}

	c.setupAdmin()
	c.createUser("models-client-token-denied")
	c.loginClient("models-client-token-denied")
	var clientTokenErr struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if code := c.request(http.MethodGet, "/api/codex/v1/models", c.clientToken, nil, &clientTokenErr); code != http.StatusUnauthorized {
		t.Fatalf("models ordinary client token status = %d", code)
	}
	if clientTokenErr.Error.Code != "login_failed" {
		t.Fatalf("models ordinary client token error = %#v", clientTokenErr.Error)
	}
}

func TestCodexModelsRequiresAvailableTokenAndRoute(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()

	c.createUser("models-zero-token")
	c.loginClient("models-zero-token")
	c.createRoutedUpstream("")
	c.approvePaidRecharge()
	providerToken := c.prepareCodexProviderToken()
	c.store.mu.Lock()
	for idx := range c.store.state.Users {
		if c.store.state.Users[idx].Account == "models-zero-token" {
			c.store.state.Users[idx].TokenBalance = 0
		}
	}
	c.store.mu.Unlock()
	var tokenErr struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if code := c.request(http.MethodGet, "/api/codex/v1/models", providerToken, nil, &tokenErr); code != http.StatusPaymentRequired {
		t.Fatalf("zero-token models status = %d", code)
	}
	if tokenErr.Error.Code != "token_not_available" || tokenErr.Error.Type != "codexppp_error" || tokenErr.Error.Message != "暂时无法继续使用，请刷新状态后重试" {
		t.Fatalf("zero-token models error = %#v", tokenErr)
	}
	for _, forbidden := range []string{"余额不足", "API Key", "upstream", "上游", "route", "账号不可用", "设备不可用"} {
		if strings.Contains(tokenErr.Error.Message, forbidden) {
			t.Fatalf("zero-token models message leaked forbidden wording %q: %#v", forbidden, tokenErr.Error)
		}
	}

	c.createUser("models-no-route")
	c.loginClient("models-no-route")
	c.approvePaidRecharge()
	c.createRoutedUpstream("")
	providerToken = c.prepareCodexProviderToken()
	c.store.mu.Lock()
	c.store.state.APIKeys = nil
	c.store.state.UpstreamAccounts = nil
	c.store.mu.Unlock()
	var routeErr struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if code := c.request(http.MethodGet, "/api/codex/v1/models", providerToken, nil, &routeErr); code != http.StatusServiceUnavailable {
		t.Fatalf("no-route models status = %d", code)
	}
	if routeErr.Error.Code != "route_unavailable" || routeErr.Error.Type != "codexppp_error" || routeErr.Error.Message != "服务暂时不可用，请稍后再试" {
		t.Fatalf("no-route models error = %#v", routeErr)
	}
}

func TestRootV1CompatibilityRoutesAreNotExposed(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("no-root-v1")
	c.loginClient("no-root-v1")
	routes := []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodGet, "/v1/models", nil},
		{http.MethodPost, "/v1/responses", map[string]any{"model": "codex", "input": "hello"}},
		{http.MethodPost, "/v1/chat/completions", map[string]any{"model": "codex", "messages": []any{}}},
		{http.MethodPost, "/api/gateway/runs", map[string]any{"model": "codex", "input": "hello"}},
	}
	for _, route := range routes {
		if code := c.request(route.method, route.path, c.clientToken, route.body, nil); code != http.StatusNotFound {
			t.Fatalf("%s %s status = %d", route.method, route.path, code)
		}
	}
}

func TestGatewayRunsHTTPRouteIsNotRegistered(t *testing.T) {
	raw, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, forbidden := range []string{
		`mux.HandleFunc("/api/gateway/`,
		"func (a *App) gatewayAPI",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("gateway internal runner is exposed as HTTP route via %q", forbidden)
		}
	}
	if !strings.Contains(text, `mux.HandleFunc("/api/codex/"`) {
		t.Fatal("codex provider endpoint must remain registered")
	}
}

func TestCodexResponsesRequiresClientAuth(t *testing.T) {
	c := newTestClient(t)
	var out struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	code := c.request(http.MethodPost, "/api/codex/v1/responses", "", map[string]any{"model": "codex", "input": "hello"}, &out)
	if code != http.StatusUnauthorized {
		t.Fatalf("responses status = %d", code)
	}
	if out.Error.Code != "login_failed" || out.Error.Type != "codexppp_error" || out.Error.Message != "登录失败，请重新登录" {
		t.Fatalf("responses error = %#v", out.Error)
	}
}

func TestCodexResponsesFailureUsesSanitizedChineseMessage(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("codex-zero-token")
	c.loginClient("codex-zero-token")
	c.approvePaidRecharge()
	c.createRoutedUpstream("")
	providerToken := c.prepareCodexProviderToken()
	c.store.mu.Lock()
	for idx := range c.store.state.Users {
		if c.store.state.Users[idx].Account == "codex-zero-token" {
			c.store.state.Users[idx].TokenBalance = 0
		}
	}
	c.store.mu.Unlock()

	var out struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	code := c.request(http.MethodPost, "/api/codex/v1/responses", providerToken, map[string]any{"model": "codex", "input": "hello"}, &out)
	if code != http.StatusPaymentRequired {
		t.Fatalf("responses status = %d", code)
	}
	if out.Error.Code != "token_not_available" || out.Error.Type != "codexppp_error" {
		t.Fatalf("responses error = %#v", out.Error)
	}
	if out.Error.Message != "暂时无法继续使用，请刷新状态后重试" {
		t.Fatalf("responses message = %#v", out.Error)
	}
	for _, forbidden := range []string{"余额不足", "API Key", "upstream", "上游", "route", "账号不可用", "设备不可用"} {
		if strings.Contains(out.Error.Message, forbidden) {
			t.Fatalf("responses message leaked forbidden wording %q: %#v", forbidden, out.Error)
		}
	}
}

func TestCodexResponsesIdempotencyReturnsStoredOutput(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("codex-idempotent-output")
	c.loginClient("codex-idempotent-output")
	c.approvePaidRecharge()

	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		writeJSON(w, http.StatusOK, map[string]any{
			"model": "codex",
			"result": map[string]any{
				"output_text": "stored provider output",
			},
			"usage": map[string]any{
				"input_tokens":  5,
				"output_tokens": 9,
				"total_tokens":  14,
			},
		})
	}))
	defer upstream.Close()
	c.createRoutedUpstream(upstream.URL)
	providerToken := c.prepareCodexProviderToken()

	headers := map[string]string{"Idempotency-Key": "codex-response-output-1"}
	var first, second struct {
		OutputText string `json:"output_text"`
		Usage      struct {
			TotalTokens int64 `json:"total_tokens"`
		} `json:"usage"`
	}
	body := map[string]any{"model": "codex", "input": "hello"}
	if code := c.requestWithHeaders(http.MethodPost, "/api/codex/v1/responses", providerToken, headers, body, &first); code != http.StatusOK {
		t.Fatalf("first responses status = %d", code)
	}
	if code := c.requestWithHeaders(http.MethodPost, "/api/codex/v1/responses", providerToken, headers, body, &second); code != http.StatusOK {
		t.Fatalf("second responses status = %d", code)
	}
	if upstreamCalls != 1 {
		t.Fatalf("upstream calls = %d", upstreamCalls)
	}
	if first.OutputText != "stored provider output" || second.OutputText != first.OutputText {
		t.Fatalf("idempotent provider output first=%#v second=%#v", first, second)
	}
	if first.Usage.TotalTokens != 14 || second.Usage.TotalTokens != 14 {
		t.Fatalf("idempotent provider usage first=%#v second=%#v", first.Usage, second.Usage)
	}
	var me struct {
		User map[string]any `json:"user"`
	}
	if code := c.request(http.MethodGet, "/api/client/me", c.clientToken, nil, &me); code != http.StatusOK {
		t.Fatalf("me status = %d", code)
	}
	if got := int64(me.User["tokenBalance"].(float64)); got != 99986 {
		t.Fatalf("token balance = %d", got)
	}
}

func TestCodexResponsePayloadFromGateway(t *testing.T) {
	payload := codexResponsePayloadFromGateway(map[string]any{
		"requestId": "req_1",
		"usageRecord": map[string]any{
			"model":             "codex",
			"inputTokens":       int64(11),
			"cachedInputTokens": int64(3),
			"outputTokens":      int64(7),
			"totalTokens":       int64(21),
		},
		"result": map[string]any{"output_text": "done"},
	})
	if payload["id"] != "resp_req_1" || payload["output_text"] != "done" {
		t.Fatalf("unexpected response payload: %#v", payload)
	}
	usage := payload["usage"].(map[string]any)
	if usage["total_tokens"] != int64(21) || usage["cached_input_tokens"] != int64(3) {
		t.Fatalf("unexpected usage payload: %#v", usage)
	}
}

func TestCodexResponsesSSEIncludesOutputLifecycleBeforeDelta(t *testing.T) {
	response := codexResponsePayloadFromGateway(map[string]any{
		"requestId": "req_stream",
		"usageRecord": map[string]any{
			"model":        "codex",
			"inputTokens":  int64(1),
			"outputTokens": int64(1),
			"totalTokens":  int64(2),
		},
		"result": map[string]any{"text": "ok"},
	})
	stream := codexResponsesSSE(response)
	required := []string{
		"event: response.created",
		"event: response.output_item.added",
		"event: response.content_part.added",
		"event: response.output_text.delta",
		"event: response.output_text.done",
		"event: response.content_part.done",
		"event: response.output_item.done",
		"event: response.completed",
		"data: [DONE]",
	}
	last := -1
	for _, marker := range required {
		idx := strings.Index(stream, marker)
		if idx < 0 {
			t.Fatalf("stream missing %q:\n%s", marker, stream)
		}
		if idx <= last {
			t.Fatalf("stream marker %q is out of order:\n%s", marker, stream)
		}
		last = idx
	}
	if !strings.Contains(stream, `"item_id":"msg_req_stream"`) || !strings.Contains(stream, `"delta":"ok"`) {
		t.Fatalf("stream did not include item id and text delta:\n%s", stream)
	}
}

func TestResultFieldExtractsResponsesText(t *testing.T) {
	result := resultField(map[string]any{
		"output_text": "responses result",
		"usage":       map[string]any{"total_tokens": 12},
	})
	text := textFromAny(result)
	if text != "responses result" {
		t.Fatalf("result text = %q", text)
	}
}

func TestAdminMustChangePasswordBlocksBusinessButAllowsMe(t *testing.T) {
	c := newTestClient(t)
	var setup struct {
		Token string `json:"token"`
		Admin struct {
			MustChangePassword bool `json:"mustChangePassword"`
		} `json:"admin"`
	}
	if code := c.request(http.MethodPost, "/api/admin/setup", "", map[string]any{"account": "root", "password": ""}, &setup); code != http.StatusOK {
		t.Fatalf("setup status = %d", code)
	}
	if !setup.Admin.MustChangePassword {
		t.Fatalf("expected first password change flag")
	}
	var me struct {
		Admin struct {
			MustChangePassword bool `json:"mustChangePassword"`
		} `json:"admin"`
	}
	if code := c.request(http.MethodGet, "/api/admin/me", setup.Token, nil, &me); code != http.StatusOK {
		t.Fatalf("admin me status = %d", code)
	}
	if !me.Admin.MustChangePassword {
		t.Fatalf("admin me mustChangePassword = false")
	}
	if code := c.request(http.MethodPost, "/api/admin/users", setup.Token, map[string]any{"account": "blocked", "password": ""}, nil); code != http.StatusConflict {
		t.Fatalf("business write before password change = %d", code)
	}
	if code := c.request(http.MethodPost, "/api/admin/password", setup.Token, map[string]any{"password": ""}, nil); code != http.StatusOK {
		t.Fatalf("password change status = %d", code)
	}
	if code := c.request(http.MethodPost, "/api/admin/users", setup.Token, map[string]any{"account": "allowed", "password": ""}, nil); code != http.StatusCreated {
		t.Fatalf("business write after password change = %d", code)
	}
}

func TestAdminAuthRequestsRejectUnknownFieldsButAllowEmptyPassword(t *testing.T) {
	c := newTestClient(t)
	var invalidSetup apiError
	if code := c.request(http.MethodPost, "/api/admin/setup", "", map[string]any{"account": "root", "password": "", "unexpectedField": "unexpected"}, &invalidSetup); code != http.StatusBadRequest {
		t.Fatalf("setup with unknown field status = %d", code)
	}
	if invalidSetup.Error != "invalid_admin_setup_request" {
		t.Fatalf("setup with unknown field error = %#v", invalidSetup)
	}
	c.setupAdmin()

	var invalidLogin apiError
	if code := c.request(http.MethodPost, "/api/admin/login", "", map[string]any{"account": "root", "password": "", "unexpectedField": "unexpected"}, &invalidLogin); code != http.StatusBadRequest {
		t.Fatalf("login with unknown field status = %d", code)
	}
	if invalidLogin.Error != "invalid_admin_login_request" {
		t.Fatalf("login with unknown field error = %#v", invalidLogin)
	}

	var invalidPassword apiError
	if code := c.request(http.MethodPost, "/api/admin/password", c.adminToken, map[string]any{"password": "", "unexpectedField": "unexpected"}, &invalidPassword); code != http.StatusBadRequest {
		t.Fatalf("password with unknown field status = %d", code)
	}
	if invalidPassword.Error != "invalid_admin_password_request" {
		t.Fatalf("password with unknown field error = %#v", invalidPassword)
	}
	if code := c.request(http.MethodPost, "/api/admin/password", c.adminToken, map[string]any{"password": ""}, nil); code != http.StatusOK {
		t.Fatalf("empty admin password status = %d", code)
	}
}

func TestRechargeFlowAndFreeTopupRules(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("alice")
	c.loginClient("alice")

	var topups struct {
		Items []TokenTopup `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/client/topups", c.clientToken, nil, &topups); code != http.StatusOK {
		t.Fatalf("topups status = %d", code)
	}
	var publicTopups struct {
		Items []map[string]any `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/client/topups", c.clientToken, nil, &publicTopups); code != http.StatusOK {
		t.Fatalf("public topups status = %d", code)
	}
	for _, item := range publicTopups.Items {
		for _, key := range []string{"enabled", "sort", "description", "createdAt", "updatedAt"} {
			if _, ok := item[key]; ok {
				t.Fatalf("client topup leaked admin field %q: %#v", key, item)
			}
		}
	}
	var freeID, paidID string
	var paidTopup TokenTopup
	for _, topup := range topups.Items {
		if topup.PriceCents == 0 {
			freeID = topup.ID
		} else {
			paidID = topup.ID
			paidTopup = topup
		}
	}
	if freeID == "" || paidID == "" {
		t.Fatal("expected seeded free and paid token topups")
	}
	if code := c.request(http.MethodPost, "/api/client/recharges", c.clientToken, map[string]any{"topupId": freeID}, nil); code != http.StatusBadRequest {
		t.Fatalf("free recharge status = %d", code)
	}
	var strictErr apiError
	if code := c.request(http.MethodPost, "/api/client/recharges", c.clientToken, map[string]any{"topupId": paidID, "tokens": 999999, "remark": "should not be accepted"}, &strictErr); code != http.StatusBadRequest {
		t.Fatalf("recharge with extra fields status = %d", code)
	}
	if strictErr.Error != "invalid_recharge_request" {
		t.Fatalf("recharge with extra fields error = %#v", strictErr)
	}
	var recharge map[string]any
	if code := c.request(http.MethodPost, "/api/client/recharges", c.clientToken, map[string]any{"topupId": paidID}, &recharge); code != http.StatusCreated {
		t.Fatalf("paid recharge status = %d", code)
	}
	if recharge["statusLabel"] != "等待管理员确认" {
		t.Fatalf("pending recharge label = %#v", recharge)
	}
	for _, key := range []string{"id", "userId", "userAccount", "topupId", "priceCents", "tokens", "statusTransitions"} {
		if _, ok := recharge[key]; ok {
			t.Fatalf("client recharge leaked %q: %#v", key, recharge)
		}
	}
	var pending struct {
		Items []map[string]any `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/admin/recharges?status=pending", c.adminToken, nil, &pending); code != http.StatusOK {
		t.Fatalf("pending recharges status = %d", code)
	}
	if len(pending.Items) != 1 {
		t.Fatalf("pending recharges = %#v", pending)
	}
	rechargeID, _ := pending.Items[0]["id"].(string)
	if rechargeID == "" {
		t.Fatalf("pending recharge id missing: %#v", pending.Items[0])
	}
	disableBody := map[string]any{"name": paidTopup.Name, "priceCents": paidTopup.PriceCents, "tokens": paidTopup.Tokens, "enabled": false, "sort": 10, "description": "停用新申请"}
	if code := c.request(http.MethodPatch, "/api/admin/topups/"+paidID, c.adminToken, disableBody, nil); code != http.StatusOK {
		t.Fatalf("disable topup status = %d", code)
	}
	var visibleAfterDisable struct {
		Items []map[string]any `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/client/topups", c.clientToken, nil, &visibleAfterDisable); code != http.StatusOK {
		t.Fatalf("client topups after disable status = %d", code)
	}
	for _, item := range visibleAfterDisable.Items {
		if item["id"] == paidID {
			t.Fatalf("disabled topup still visible as new option: %#v", visibleAfterDisable.Items)
		}
	}
	var disabledTopupErr apiError
	if code := c.request(http.MethodPost, "/api/client/recharges", c.clientToken, map[string]any{"topupId": paidID}, &disabledTopupErr); code != http.StatusBadRequest {
		t.Fatalf("disabled topup recharge status = %d", code)
	}
	if disabledTopupErr.Error != "topup_unavailable" {
		t.Fatalf("disabled topup recharge error = %#v", disabledTopupErr)
	}
	var approved struct {
		Status            string                     `json:"status"`
		StatusTransitions []RechargeStatusTransition `json:"statusTransitions"`
	}
	if code := c.request(http.MethodPost, "/api/admin/recharges/"+rechargeID+"/approve", c.adminToken, nil, &approved); code != http.StatusOK {
		t.Fatalf("approve status = %d", code)
	}
	if approved.Status != rechargeApproved || len(approved.StatusTransitions) != 2 {
		t.Fatalf("approved transitions = %#v", approved)
	}
	if approved.StatusTransitions[0].Status != rechargePending || approved.StatusTransitions[0].ActorRole != "client" || approved.StatusTransitions[0].At.IsZero() {
		t.Fatalf("pending transition = %#v", approved.StatusTransitions)
	}
	if approved.StatusTransitions[1].Status != rechargeApproved || approved.StatusTransitions[1].ActorRole != "admin" || approved.StatusTransitions[1].At.IsZero() {
		t.Fatalf("approved transition = %#v", approved.StatusTransitions)
	}
	var me struct {
		User map[string]any `json:"user"`
	}
	if code := c.request(http.MethodGet, "/api/client/me", c.clientToken, nil, &me); code != http.StatusOK {
		t.Fatalf("me status = %d", code)
	}
	if got := int64(me.User["tokenBalance"].(float64)); got != 100000 {
		t.Fatalf("token balance = %d", got)
	}
	if me.User["recentRechargeStatus"] != "已确认" {
		t.Fatalf("recent recharge status = %#v", me.User)
	}
	var rejectApprovedErr apiError
	if code := c.request(http.MethodPost, "/api/admin/recharges/"+rechargeID+"/reject", c.adminToken, nil, &rejectApprovedErr); code != http.StatusConflict {
		t.Fatalf("reject approved recharge status = %d", code)
	}
	if rejectApprovedErr.Error != "recharge_not_pending" {
		t.Fatalf("reject approved recharge error = %#v", rejectApprovedErr)
	}

	enableBody := map[string]any{"name": paidTopup.Name, "priceCents": paidTopup.PriceCents, "tokens": paidTopup.Tokens, "enabled": true, "sort": 10, "description": "重新开放新申请"}
	if code := c.request(http.MethodPatch, "/api/admin/topups/"+paidID, c.adminToken, enableBody, nil); code != http.StatusOK {
		t.Fatalf("enable topup status = %d", code)
	}
	if code := c.request(http.MethodPost, "/api/client/recharges", c.clientToken, map[string]any{"topupId": paidID}, nil); code != http.StatusCreated {
		t.Fatalf("second paid recharge status = %d", code)
	}
	var secondPending struct {
		Items []map[string]any `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/admin/recharges?status=pending", c.adminToken, nil, &secondPending); code != http.StatusOK {
		t.Fatalf("second pending recharges status = %d", code)
	}
	if len(secondPending.Items) != 1 {
		t.Fatalf("second pending recharges = %#v", secondPending)
	}
	rejectedRechargeID, _ := secondPending.Items[0]["id"].(string)
	if rejectedRechargeID == "" {
		t.Fatalf("second pending recharge id missing: %#v", secondPending.Items[0])
	}
	var cancelErr apiError
	if code := c.request(http.MethodPost, "/api/admin/recharges/"+rejectedRechargeID+"/cancel", c.adminToken, nil, &cancelErr); code != http.StatusNotFound {
		t.Fatalf("cancel recharge status = %d", code)
	}
	if cancelErr.Error != "not_found" {
		t.Fatalf("cancel recharge error = %#v", cancelErr)
	}
	var rejected struct {
		Status            string                     `json:"status"`
		ConfirmedAt       *time.Time                 `json:"confirmedAt"`
		StatusTransitions []RechargeStatusTransition `json:"statusTransitions"`
	}
	if code := c.request(http.MethodPost, "/api/admin/recharges/"+rejectedRechargeID+"/reject", c.adminToken, nil, &rejected); code != http.StatusOK {
		t.Fatalf("reject status = %d", code)
	}
	if rejected.Status != rechargeRejected || rejected.ConfirmedAt == nil || rejected.ConfirmedAt.IsZero() || len(rejected.StatusTransitions) != 2 {
		t.Fatalf("rejected recharge = %#v", rejected)
	}
	if rejected.StatusTransitions[1].Status != rechargeRejected || rejected.StatusTransitions[1].ActorRole != "admin" || rejected.StatusTransitions[1].At.IsZero() {
		t.Fatalf("rejected transition = %#v", rejected.StatusTransitions)
	}
	var approveRejectedErr apiError
	if code := c.request(http.MethodPost, "/api/admin/recharges/"+rejectedRechargeID+"/approve", c.adminToken, nil, &approveRejectedErr); code != http.StatusConflict {
		t.Fatalf("approve rejected recharge status = %d", code)
	}
	if approveRejectedErr.Error != "recharge_not_pending" {
		t.Fatalf("approve rejected recharge error = %#v", approveRejectedErr)
	}
	var meAfterReject struct {
		User map[string]any `json:"user"`
	}
	if code := c.request(http.MethodGet, "/api/client/me", c.clientToken, nil, &meAfterReject); code != http.StatusOK {
		t.Fatalf("me after reject status = %d", code)
	}
	if got := int64(meAfterReject.User["tokenBalance"].(float64)); got != 100000 {
		t.Fatalf("token balance after rejected recharge = %d", got)
	}
	if meAfterReject.User["recentRechargeStatus"] != "已拒绝" {
		t.Fatalf("recent recharge status after reject = %#v", meAfterReject.User)
	}
	var clientRecharges struct {
		Items []map[string]any `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/client/recharges", c.clientToken, nil, &clientRecharges); code != http.StatusOK {
		t.Fatalf("client recharge list status = %d", code)
	}
	if len(clientRecharges.Items) != 2 {
		t.Fatalf("client recharge list = %#v", clientRecharges)
	}
	sawApproved, sawRejected := false, false
	for _, item := range clientRecharges.Items {
		for _, key := range []string{"id", "userId", "userAccount", "topupId", "priceCents", "tokens", "statusTransitions"} {
			if _, ok := item[key]; ok {
				t.Fatalf("client recharge list leaked %q: %#v", key, item)
			}
		}
		if item["topupName"] != paidTopup.Name {
			t.Fatalf("client recharge topup name = %#v", item)
		}
		switch item["status"] {
		case rechargeApproved:
			sawApproved = true
			if item["statusLabel"] != "已确认" {
				t.Fatalf("approved client recharge history = %#v", item)
			}
		case rechargeRejected:
			sawRejected = true
			if item["statusLabel"] != "已拒绝" || item["confirmedAt"] == nil || item["confirmedAt"] == "" {
				t.Fatalf("rejected client recharge history = %#v", item)
			}
		default:
			t.Fatalf("unexpected client recharge history = %#v", item)
		}
	}
	if !sawApproved || !sawRejected {
		t.Fatalf("client recharge history missing approved/rejected states: %#v", clientRecharges.Items)
	}
}

func TestAdminTopupValidationKeepsCatalogOperational(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()

	var emptyNameErr apiError
	if code := c.request(http.MethodPost, "/api/admin/topups", c.adminToken, map[string]any{"name": "   ", "priceCents": 0, "tokens": 0, "enabled": true, "sort": 20, "description": ""}, &emptyNameErr); code != http.StatusBadRequest {
		t.Fatalf("empty topup name status = %d", code)
	}
	if emptyNameErr.Error != "invalid_topup_name" {
		t.Fatalf("empty topup name error = %#v", emptyNameErr)
	}

	var paidZeroTokenErr apiError
	if code := c.request(http.MethodPost, "/api/admin/topups", c.adminToken, map[string]any{"name": "Paid empty", "priceCents": 100, "tokens": 0, "enabled": true, "sort": 20, "description": ""}, &paidZeroTokenErr); code != http.StatusBadRequest {
		t.Fatalf("paid zero-token topup status = %d", code)
	}
	if paidZeroTokenErr.Error != "invalid_topup_tokens" {
		t.Fatalf("paid zero-token topup error = %#v", paidZeroTokenErr)
	}

	var freeDisplay TokenTopup
	if code := c.request(http.MethodPost, "/api/admin/topups", c.adminToken, map[string]any{"name": "  free display  ", "priceCents": 0, "tokens": 0, "enabled": true, "sort": 20, "description": "  view only  "}, &freeDisplay); code != http.StatusCreated {
		t.Fatalf("free display topup status = %d", code)
	}
	if freeDisplay.Name != "free display" || freeDisplay.Description != "view only" || freeDisplay.PriceCents != 0 || freeDisplay.Tokens != 0 {
		t.Fatalf("free display topup = %#v", freeDisplay)
	}
}

func TestAdminUserListSearchAndPagination(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	for i := 0; i < 12; i++ {
		c.createUser(fmt.Sprintf("user-%02d", i))
	}

	var page2 struct {
		Items []map[string]any `json:"items"`
		Page  int              `json:"page"`
		Size  int              `json:"size"`
		Total int              `json:"total"`
	}
	if code := c.request(http.MethodGet, "/api/admin/users?page=2&size=10", c.adminToken, nil, &page2); code != http.StatusOK {
		t.Fatalf("users status = %d", code)
	}
	if page2.Page != 2 || page2.Size != 10 || page2.Total != 12 || len(page2.Items) != 2 {
		t.Fatalf("pagination = %#v", page2)
	}

	var searched struct {
		Items []map[string]any `json:"items"`
		Total int              `json:"total"`
	}
	if code := c.request(http.MethodGet, "/api/admin/users?q=user-01", c.adminToken, nil, &searched); code != http.StatusOK {
		t.Fatalf("search status = %d", code)
	}
	if searched.Total != 1 || searched.Items[0]["account"] != "user-01" {
		t.Fatalf("searched users = %#v", searched)
	}
}

func TestDisabledUserCannotLoginOrUseExistingSessionButHistoryRemains(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("disabled-user")
	c.loginClient("disabled-user")
	previousToken := c.clientToken
	c.approvePaidRecharge()

	c.store.mu.Lock()
	userID := c.store.state.Users[0].ID
	c.store.mu.Unlock()

	var updated map[string]any
	if code := c.request(http.MethodPost, "/api/admin/users/"+userID+"/status", c.adminToken, map[string]any{"status": "disabled"}, &updated); code != http.StatusOK {
		t.Fatalf("disable user status = %d", code)
	}
	if updated["status"] != statusDisabled {
		t.Fatalf("disabled user = %#v", updated)
	}

	var loginErr apiError
	loginBody := map[string]any{"account": "disabled-user", "password": "", "deviceName": "Windows 设备", "fingerprint": "disabled-user-device"}
	if code := c.request(http.MethodPost, "/api/client/login", "", loginBody, &loginErr); code != http.StatusUnauthorized {
		t.Fatalf("disabled user login status = %d", code)
	}
	if loginErr.Error != "login_failed" {
		t.Fatalf("disabled user login error = %#v", loginErr)
	}

	var sessionErr apiError
	if code := c.request(http.MethodGet, "/api/client/me", previousToken, nil, &sessionErr); code != http.StatusUnauthorized {
		t.Fatalf("disabled user existing session status = %d", code)
	}
	if sessionErr.Error != "login_failed" {
		t.Fatalf("disabled user existing session error = %#v", sessionErr)
	}

	var recharges struct {
		Items []map[string]any `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/admin/users/"+userID+"/recharges", c.adminToken, nil, &recharges); code != http.StatusOK {
		t.Fatalf("disabled user recharge history status = %d", code)
	}
	if len(recharges.Items) != 1 || recharges.Items[0]["status"] != rechargeApproved {
		t.Fatalf("disabled user recharge history = %#v", recharges.Items)
	}
}

func TestManagementActionsRejectUnknownFields(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()

	var invalidUser apiError
	if code := c.request(http.MethodPost, "/api/admin/users", c.adminToken, map[string]any{"account": "unknown-field-user", "password": "", "unexpectedField": "unexpected"}, &invalidUser); code != http.StatusBadRequest {
		t.Fatalf("create user with unknown field status = %d", code)
	}
	if invalidUser.Error != "invalid_user_request" {
		t.Fatalf("create user with unknown field error = %#v", invalidUser)
	}

	c.createUser("strict-user")
	c.loginClient("strict-user")
	c.store.mu.Lock()
	userID := c.store.state.Users[0].ID
	deviceID := c.store.state.Devices[0].ID
	c.store.mu.Unlock()

	var invalidUserStatus apiError
	if code := c.request(http.MethodPost, "/api/admin/users/"+userID+"/status", c.adminToken, map[string]any{"status": "disabled", "unexpectedField": "unexpected"}, &invalidUserStatus); code != http.StatusBadRequest {
		t.Fatalf("user status with unknown field status = %d", code)
	}
	if invalidUserStatus.Error != "invalid_user_status_request" {
		t.Fatalf("user status with unknown field error = %#v", invalidUserStatus)
	}

	var invalidUserPassword apiError
	if code := c.request(http.MethodPost, "/api/admin/users/"+userID+"/password", c.adminToken, map[string]any{"password": "", "unexpectedField": "unexpected"}, &invalidUserPassword); code != http.StatusBadRequest {
		t.Fatalf("user password with unknown field status = %d", code)
	}
	if invalidUserPassword.Error != "invalid_user_password_request" {
		t.Fatalf("user password with unknown field error = %#v", invalidUserPassword)
	}

	var invalidAdjustment apiError
	if code := c.request(http.MethodPost, "/api/admin/users/"+userID+"/token-adjustments", c.adminToken, map[string]any{"deltaTokens": 10, "remark": "", "unexpectedField": "unexpected"}, &invalidAdjustment); code != http.StatusBadRequest {
		t.Fatalf("token adjustment with unknown field status = %d", code)
	}
	if invalidAdjustment.Error != "invalid_token_adjustment_request" {
		t.Fatalf("token adjustment with unknown field error = %#v", invalidAdjustment)
	}

	var invalidDeviceStatus apiError
	if code := c.request(http.MethodPost, "/api/admin/devices/"+deviceID+"/status", c.adminToken, map[string]any{"status": "disabled", "unexpectedField": "unexpected"}, &invalidDeviceStatus); code != http.StatusBadRequest {
		t.Fatalf("device status with unknown field status = %d", code)
	}
	if invalidDeviceStatus.Error != "invalid_device_status_request" {
		t.Fatalf("device status with unknown field error = %#v", invalidDeviceStatus)
	}

	var invalidClientLogin apiError
	if code := c.request(http.MethodPost, "/api/client/login", "", map[string]any{"account": "strict-user", "password": "", "deviceName": "Windows 设备", "fingerprint": "strict-user-device-2", "unexpectedField": "unexpected"}, &invalidClientLogin); code != http.StatusBadRequest {
		t.Fatalf("client login with unknown field status = %d", code)
	}
	if invalidClientLogin.Error != "invalid_client_login_request" {
		t.Fatalf("client login with unknown field error = %#v", invalidClientLogin)
	}

	var upstream map[string]any
	upstreamBody := map[string]any{"name": "strict-upstream", "group": "default", "credentialType": "oauth", "accessToken": "access", "refreshToken": "refresh"}
	if code := c.request(http.MethodPost, "/api/admin/upstreams", c.adminToken, upstreamBody, &upstream); code != http.StatusCreated {
		t.Fatalf("create upstream status = %d", code)
	}
	upstreamID, _ := upstream["id"].(string)
	if upstreamID == "" {
		t.Fatalf("upstream id missing: %#v", upstream)
	}
	var invalidUpstreamStatus apiError
	if code := c.request(http.MethodPost, "/api/admin/upstreams/"+upstreamID+"/status", c.adminToken, map[string]any{"status": "disabled", "unexpectedField": "unexpected"}, &invalidUpstreamStatus); code != http.StatusBadRequest {
		t.Fatalf("upstream status with unknown field status = %d", code)
	}
	if invalidUpstreamStatus.Error != "invalid_upstream_status_request" {
		t.Fatalf("upstream status with unknown field error = %#v", invalidUpstreamStatus)
	}

	c.createUser("strict-recharge")
	c.loginClient("strict-recharge")
	var topups struct {
		Items []TokenTopup `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/client/topups", c.clientToken, nil, &topups); code != http.StatusOK {
		t.Fatalf("strict topups status = %d", code)
	}
	var paidID string
	for _, topup := range topups.Items {
		if topup.PriceCents > 0 {
			paidID = topup.ID
			break
		}
	}
	if paidID == "" {
		t.Fatal("strict recharge paid topup missing")
	}
	if code := c.request(http.MethodPost, "/api/client/recharges", c.clientToken, map[string]any{"topupId": paidID}, nil); code != http.StatusCreated {
		t.Fatalf("strict recharge create status = %d", code)
	}
	var pending struct {
		Items []map[string]any `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/admin/recharges?status=pending", c.adminToken, nil, &pending); code != http.StatusOK {
		t.Fatalf("strict pending recharges status = %d", code)
	}
	if len(pending.Items) != 1 {
		t.Fatalf("strict pending recharges = %#v", pending.Items)
	}
	rechargeID, _ := pending.Items[0]["id"].(string)
	var invalidRechargeApprove apiError
	if code := c.request(http.MethodPost, "/api/admin/recharges/"+rechargeID+"/approve", c.adminToken, map[string]any{"tokens": 999999, "remark": "override"}, &invalidRechargeApprove); code != http.StatusBadRequest {
		t.Fatalf("recharge approve with body status = %d", code)
	}
	if invalidRechargeApprove.Error != "invalid_recharge_action_request" {
		t.Fatalf("recharge approve with body error = %#v", invalidRechargeApprove)
	}
	var stillPending struct {
		Status string `json:"status"`
	}
	if code := c.request(http.MethodPost, "/api/admin/recharges/"+rechargeID+"/reject", c.adminToken, nil, &stillPending); code != http.StatusOK {
		t.Fatalf("recharge reject after invalid approve status = %d", code)
	}
	if stillPending.Status != rechargeRejected {
		t.Fatalf("recharge status after invalid approve = %#v", stillPending)
	}

	var invalidUpstreamCheck apiError
	if code := c.request(http.MethodPost, "/api/admin/upstreams/"+upstreamID+"/check", c.adminToken, map[string]any{"refreshToken": "override", "route": "old-route"}, &invalidUpstreamCheck); code != http.StatusBadRequest {
		t.Fatalf("upstream check with body status = %d", code)
	}
	if invalidUpstreamCheck.Error != "invalid_upstream_check_request" {
		t.Fatalf("upstream check with body error = %#v", invalidUpstreamCheck)
	}
}

func TestJSONRequestsRejectTrailingContent(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()

	var createUserErr apiError
	if code := c.requestRaw(http.MethodPost, "/api/admin/users", c.adminToken, `{"account":"trailing-user","password":""} {"extra":true}`, &createUserErr); code != http.StatusBadRequest {
		t.Fatalf("create user trailing json status = %d", code)
	}
	if createUserErr.Error != "invalid_json" {
		t.Fatalf("create user trailing json error = %#v", createUserErr)
	}

	var topupErr apiError
	if code := c.requestRaw(http.MethodPost, "/api/admin/topups", c.adminToken, `{"name":"Trailing","priceCents":0,"tokens":0,"enabled":true,"sort":10,"description":""} {"extra":true}`, &topupErr); code != http.StatusBadRequest {
		t.Fatalf("topup trailing json status = %d", code)
	}
	if topupErr.Error != "invalid_json" {
		t.Fatalf("topup trailing json error = %#v", topupErr)
	}

	c.createUser("trailing-client")
	c.loginClient("trailing-client")

	var launchErr apiError
	if code := c.requestRaw(http.MethodPost, "/api/client/launch/prepare", c.clientToken, `{} {}`, &launchErr); code != http.StatusBadRequest {
		t.Fatalf("launch prepare trailing json status = %d", code)
	}
	if launchErr.Error != "invalid_json" {
		t.Fatalf("launch prepare trailing json error = %#v", launchErr)
	}

	var gatewayErr apiError
	if code := c.gatewayRunRawWithHeaders(c.clientToken, map[string]string{"Idempotency-Key": "trailing-json"}, `{"model":"gpt-5.5","input":"hello"} {}`, &gatewayErr); code != http.StatusBadRequest {
		t.Fatalf("gateway trailing json status = %d", code)
	}
	if gatewayErr.Error != "invalid_json" {
		t.Fatalf("gateway trailing json error = %#v", gatewayErr)
	}

	var topups struct {
		Items []TokenTopup `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/client/topups", c.clientToken, nil, &topups); code != http.StatusOK {
		t.Fatalf("trailing topups status = %d", code)
	}
	var paidID string
	for _, topup := range topups.Items {
		if topup.PriceCents > 0 {
			paidID = topup.ID
			break
		}
	}
	if paidID == "" {
		t.Fatal("trailing paid topup missing")
	}
	if code := c.request(http.MethodPost, "/api/client/recharges", c.clientToken, map[string]any{"topupId": paidID}, nil); code != http.StatusCreated {
		t.Fatalf("trailing recharge create status = %d", code)
	}
	var pending struct {
		Items []map[string]any `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/admin/recharges?status=pending", c.adminToken, nil, &pending); code != http.StatusOK {
		t.Fatalf("trailing pending recharges status = %d", code)
	}
	if len(pending.Items) != 1 {
		t.Fatalf("trailing pending recharges = %#v", pending.Items)
	}
	rechargeID, _ := pending.Items[0]["id"].(string)
	var actionErr apiError
	if code := c.requestRaw(http.MethodPost, "/api/admin/recharges/"+rechargeID+"/approve", c.adminToken, `{} {}`, &actionErr); code != http.StatusBadRequest {
		t.Fatalf("recharge action trailing json status = %d", code)
	}
	if actionErr.Error != "invalid_json" {
		t.Fatalf("recharge action trailing json error = %#v", actionErr)
	}
}

func TestJSONRequestsRejectOversizedBodies(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()

	oversizedAdmin := `{"account":"oversized-user","password":"` + strings.Repeat("x", int(maxJSONBodyBytes)+1) + `"}`
	var adminErr apiError
	if code := c.requestRaw(http.MethodPost, "/api/admin/users", c.adminToken, oversizedAdmin, &adminErr); code != http.StatusBadRequest {
		t.Fatalf("oversized admin request status = %d", code)
	}
	if adminErr.Error != "invalid_json" {
		t.Fatalf("oversized admin request error = %#v", adminErr)
	}

	c.createUser("oversized-client")
	c.loginClient("oversized-client")
	oversizedGateway := `{"model":"gpt-5.5","input":"` + strings.Repeat("x", int(maxGatewayBodyBytes)+1) + `"}`
	var gatewayErr apiError
	if code := c.gatewayRunRawWithHeaders(c.clientToken, map[string]string{"Idempotency-Key": "oversized-body"}, oversizedGateway, &gatewayErr); code != http.StatusBadRequest {
		t.Fatalf("oversized gateway request status = %d", code)
	}
	if gatewayErr.Error != "invalid_json" {
		t.Fatalf("oversized gateway request error = %#v", gatewayErr)
	}
}

func TestAdminUserDetailResourcesAndManualAdjustment(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("detail-user")
	c.loginClient("detail-user")
	c.approvePaidRecharge()

	c.store.mu.Lock()
	userID := c.store.state.Users[0].ID
	now := time.Now().UTC()
	for i := 0; i < 12; i++ {
		c.store.state.UsageRecords = append(c.store.state.UsageRecords, UsageRecord{ID: c.store.nextID("use"), UserID: userID, Model: "gpt-5.5", InputTokens: int64(i + 1), OutputTokens: 1, TotalTokens: int64(i + 2), CreatedAt: now.Add(time.Duration(i) * time.Minute)})
	}
	c.store.mu.Unlock()

	if code := c.request(http.MethodPost, "/api/admin/users/"+userID+"/token-adjustments", c.adminToken, map[string]any{"deltaTokens": 123, "remark": "人工校正"}, nil); code != http.StatusOK {
		t.Fatalf("manual adjustment status = %d", code)
	}
	if code := c.request(http.MethodPost, "/api/admin/users/"+userID+"/password", c.adminToken, map[string]any{"password": ""}, nil); code != http.StatusOK {
		t.Fatalf("password reset status = %d", code)
	}

	var ledger struct {
		Items []struct {
			DeltaTokens int64  `json:"deltaTokens"`
			Source      string `json:"source"`
			TypeLabel   string `json:"typeLabel"`
		} `json:"items"`
		Total int `json:"total"`
	}
	if code := c.request(http.MethodGet, "/api/admin/users/"+userID+"/ledger?page=1&size=10", c.adminToken, nil, &ledger); code != http.StatusOK {
		t.Fatalf("ledger status = %d", code)
	}
	if ledger.Total < 2 || ledger.Items[0].Source != "管理员调整：人工校正" || ledger.Items[0].DeltaTokens != 123 {
		t.Fatalf("ledger = %#v", ledger)
	}
	if ledger.Items[0].TypeLabel != "收入" {
		t.Fatalf("ledger type label = %#v", ledger.Items[0])
	}

	var recharges struct {
		Items []map[string]any `json:"items"`
		Total int              `json:"total"`
	}
	if code := c.request(http.MethodGet, "/api/admin/users/"+userID+"/recharges", c.adminToken, nil, &recharges); code != http.StatusOK {
		t.Fatalf("recharges status = %d", code)
	}
	if recharges.Total != 1 || recharges.Items[0]["userAccount"] != "detail-user" {
		t.Fatalf("recharges = %#v", recharges)
	}

	var usage struct {
		Items []UsageRecord `json:"items"`
		Page  int           `json:"page"`
		Total int           `json:"total"`
	}
	if code := c.request(http.MethodGet, "/api/admin/users/"+userID+"/usage?page=2&size=10", c.adminToken, nil, &usage); code != http.StatusOK {
		t.Fatalf("usage status = %d", code)
	}
	if usage.Page != 2 || usage.Total != 12 || len(usage.Items) != 2 {
		t.Fatalf("usage = %#v", usage)
	}
}

func TestAdminTopupSearchFilterAndPagination(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	if code := c.request(http.MethodPost, "/api/admin/topups", c.adminToken, map[string]any{"name": "bad-price", "priceCents": -1, "tokens": 1000, "enabled": true}, nil); code != http.StatusBadRequest {
		t.Fatalf("negative price topup status = %d", code)
	}
	if code := c.request(http.MethodPost, "/api/admin/topups", c.adminToken, map[string]any{"name": "bad-tokens", "priceCents": 100, "tokens": -1, "enabled": true}, nil); code != http.StatusBadRequest {
		t.Fatalf("negative tokens topup status = %d", code)
	}
	var extraErr apiError
	if code := c.request(http.MethodPost, "/api/admin/topups", c.adminToken, map[string]any{"name": "unknown-field-topup", "priceCents": 100, "tokens": 1000, "enabled": true, "unexpectedField": "unexpected"}, &extraErr); code != http.StatusBadRequest {
		t.Fatalf("topup with extra fields status = %d", code)
	}
	if extraErr.Error != "invalid_topup_request" {
		t.Fatalf("topup with extra fields error = %#v", extraErr)
	}
	for i := 0; i < 11; i++ {
		body := map[string]any{"name": fmt.Sprintf("alpha-topup-%02d", i), "priceCents": 100 + i, "tokens": 1000 + i, "enabled": i%2 == 0, "sort": i}
		if code := c.request(http.MethodPost, "/api/admin/topups", c.adminToken, body, nil); code != http.StatusCreated {
			t.Fatalf("create topup %d status = %d", i, code)
		}
	}
	var filtered struct {
		Items []TokenTopup `json:"items"`
		Total int          `json:"total"`
	}
	if code := c.request(http.MethodGet, "/api/admin/topups?q=alpha-topup&enabled=false&page=1&size=3", c.adminToken, nil, &filtered); code != http.StatusOK {
		t.Fatalf("topups status = %d", code)
	}
	if filtered.Total != 5 || len(filtered.Items) != 3 {
		t.Fatalf("filtered topups = %#v", filtered)
	}
	for _, item := range filtered.Items {
		if item.Enabled || !strings.Contains(item.Name, "alpha-topup") {
			t.Fatalf("unexpected topup item = %#v", item)
		}
		if item.CreatedAt.IsZero() || item.UpdatedAt.IsZero() {
			t.Fatalf("topup timestamps missing = %#v", item)
		}
	}
	invalidPatch := filtered.Items[0]
	patchBody := map[string]any{"name": invalidPatch.Name, "priceCents": invalidPatch.PriceCents, "tokens": -1, "enabled": invalidPatch.Enabled, "sort": invalidPatch.Sort, "description": invalidPatch.Description}
	if code := c.request(http.MethodPatch, "/api/admin/topups/"+invalidPatch.ID, c.adminToken, patchBody, nil); code != http.StatusBadRequest {
		t.Fatalf("negative tokens patch status = %d", code)
	}
	patchBody["id"] = invalidPatch.ID
	if code := c.request(http.MethodPatch, "/api/admin/topups/"+invalidPatch.ID, c.adminToken, patchBody, &extraErr); code != http.StatusBadRequest {
		t.Fatalf("topup patch with extra fields status = %d", code)
	}
	if extraErr.Error != "invalid_topup_request" {
		t.Fatalf("topup patch with extra fields error = %#v", extraErr)
	}
}

func TestClientMeReturnsSanitizedDeviceAndSecurityStatus(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("device-user")
	c.loginClient("device-user")

	var me struct {
		User     map[string]any `json:"user"`
		Device   map[string]any `json:"device"`
		Security map[string]any `json:"security"`
		Service  map[string]any `json:"service"`
	}
	if code := c.request(http.MethodGet, "/api/client/me", c.clientToken, nil, &me); code != http.StatusOK {
		t.Fatalf("me status = %d", code)
	}
	if me.User["account"] != "device-user" || me.User["tokenBalance"] == nil || me.User["recentRechargeStatus"] == nil {
		t.Fatalf("user = %#v", me.User)
	}
	for _, key := range []string{"id", "status", "lastLoginAt", "createdAt"} {
		if _, ok := me.User[key]; ok {
			t.Fatalf("client user leaked %q: %#v", key, me.User)
		}
	}
	if me.Device["status"] != "可用" {
		t.Fatalf("device = %#v", me.Device)
	}
	for _, key := range []string{"id", "userId", "name", "fingerprint", "lastSeenAt", "createdAt"} {
		if _, ok := me.Device[key]; ok {
			t.Fatalf("client device leaked %q: %#v", key, me.Device)
		}
	}
	if me.Security["accountStatus"] != "可用" || me.Security["deviceStatus"] != "可用" || me.Security["sessionStatus"] != "可用" {
		t.Fatalf("security = %#v", me.Security)
	}
	if me.Service["status"] != "不可用" {
		t.Fatalf("service = %#v", me.Service)
	}
	for _, key := range []string{"routeId", "upstreamId", "apiKeyId", "reason"} {
		if _, ok := me.Service[key]; ok {
			t.Fatalf("client service leaked %q: %#v", key, me.Service)
		}
	}
}

func TestAuditRecordsOperationalEvents(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()

	if code := c.request(http.MethodPost, "/api/admin/login", "", map[string]any{"account": "root", "password": "wrong"}, nil); code != http.StatusUnauthorized {
		t.Fatalf("failed admin login status = %d", code)
	}
	if code := c.request(http.MethodPost, "/api/admin/login", "", map[string]any{"account": "root", "password": ""}, nil); code != http.StatusOK {
		t.Fatalf("admin login status = %d", code)
	}

	if code := c.request(http.MethodPost, "/api/client/login", "", map[string]any{"account": "missing", "password": "", "deviceName": "Windows 设备", "fingerprint": "missing-device"}, nil); code != http.StatusUnauthorized {
		t.Fatalf("failed client login status = %d", code)
	}
	c.createUser("audit-user")
	c.loginClient("audit-user")
	c.approvePaidRecharge()

	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			writeJSON(w, http.StatusOK, map[string]any{"result": map[string]any{"ok": true}})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"model": "gpt-5.5", "usage": map[string]any{"input_tokens": 2, "output_tokens": 3, "total_tokens": 5}})
	}))
	defer upstream.Close()
	c.createRoutedUpstream(upstream.URL)
	if code := c.gatewayRunWithHeaders(c.clientToken, map[string]string{"Idempotency-Key": "audit-fail"}, map[string]any{"model": "gpt-5.5", "input": "fail"}, nil); code != http.StatusBadGateway {
		t.Fatalf("failed gateway status = %d", code)
	}
	if code := c.gatewayRunWithHeaders(c.clientToken, map[string]string{"Idempotency-Key": "audit-success"}, map[string]any{"model": "gpt-5.5", "input": "success"}, nil); code != http.StatusOK {
		t.Fatalf("successful gateway status = %d", code)
	}
	if calls != 2 {
		t.Fatalf("upstream calls = %d", calls)
	}

	actions := c.auditActionCounts()
	for _, action := range []string{
		"admin.setup",
		"admin.password.change",
		"admin.login.failed",
		"admin.login.success",
		"client.login.failed",
		"client.login.success",
		"recharge.request",
		"recharge.approve",
		"token.recharge.add",
		"gateway.request.release",
		"gateway.usage.debit",
	} {
		if actions[action] == 0 {
			t.Fatalf("missing audit action %q in %#v", action, actions)
		}
	}
	var audit struct {
		Items []struct {
			ActorRole    string `json:"actorRole"`
			ActorAccount string `json:"actorAccount"`
			Action       string `json:"action"`
		} `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/admin/audit?size=100", c.adminToken, nil, &audit); code != http.StatusOK {
		t.Fatalf("audit status = %d", code)
	}
	adminActorFound := false
	clientActorFound := false
	for _, item := range audit.Items {
		if item.ActorRole == "admin" && item.ActorAccount == "root" && item.Action == "admin.login.success" {
			adminActorFound = true
		}
		if item.ActorRole == "client" && item.ActorAccount == "audit-user" && item.Action == "gateway.usage.debit" {
			clientActorFound = true
		}
	}
	if !adminActorFound || !clientActorFound {
		t.Fatalf("audit actor accounts admin=%v client=%v items=%#v", adminActorFound, clientActorFound, audit.Items)
	}
}

func TestAuditRedactsCredentialLikeTargetAndDetail(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()

	c.store.mu.Lock()
	c.app.auditLocked("system", "system", "upstream.check", `C:\Users\1\.codex\config.toml token=abc`, "Authorization: Bearer secret")
	c.app.auditLocked("system", "system", "gateway.route.switch", "up_1", "from_key=key_1 to_key=key_2 reason=upstream_balance_unavailable")
	c.store.mu.Unlock()

	var audit struct {
		Items []AuditLog `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/admin/audit?size=100", c.adminToken, nil, &audit); code != http.StatusOK {
		t.Fatalf("audit status = %d", code)
	}
	redactedFound := false
	switchFound := false
	for _, item := range audit.Items {
		if item.Action == "upstream.check" && item.ActorRole == "system" {
			redactedFound = true
			if item.TargetID != "redacted" || item.Detail != "redacted" {
				t.Fatalf("sensitive audit text was not redacted: %#v", item)
			}
		}
		if item.Action == "gateway.route.switch" && item.TargetID == "up_1" {
			switchFound = true
			if !strings.Contains(item.Detail, "reason=upstream_balance_unavailable") {
				t.Fatalf("non-secret route switch audit lost reason: %#v", item)
			}
			if item.Detail == "redacted" {
				t.Fatalf("route switch ids should remain auditable: %#v", item)
			}
		}
	}
	if !redactedFound || !switchFound {
		t.Fatalf("expected audit entries missing redacted=%v switch=%v items=%#v", redactedFound, switchFound, audit.Items)
	}
}

func TestClientAdvancedListsPaginateTenItems(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("paged")
	c.loginClient("paged")

	var topups struct {
		Items []TokenTopup `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/client/topups", c.clientToken, nil, &topups); code != http.StatusOK {
		t.Fatalf("topups status = %d", code)
	}
	var paidID string
	for _, topup := range topups.Items {
		if topup.PriceCents > 0 {
			paidID = topup.ID
			break
		}
	}
	for i := 0; i < 12; i++ {
		if code := c.request(http.MethodPost, "/api/client/recharges", c.clientToken, map[string]any{"topupId": paidID}, nil); code != http.StatusCreated {
			t.Fatalf("create recharge %d status = %d", i, code)
		}
	}

	c.store.mu.Lock()
	userID := c.store.state.Users[0].ID
	now := time.Now().UTC()
	for i := 0; i < 12; i++ {
		c.store.state.UsageRecords = append(c.store.state.UsageRecords, UsageRecord{ID: c.store.nextID("use"), UserID: userID, Model: "gpt-5.5", InputTokens: int64(i + 1), OutputTokens: 1, TotalTokens: int64(i + 2), CreatedAt: now.Add(time.Duration(i) * time.Minute)})
	}
	c.store.mu.Unlock()

	var usage struct {
		Items []UsageRecord `json:"items"`
		Page  int           `json:"page"`
		Total int           `json:"total"`
	}
	if code := c.request(http.MethodGet, "/api/client/usage?page=2&size=10", c.clientToken, nil, &usage); code != http.StatusOK {
		t.Fatalf("usage status = %d", code)
	}
	if usage.Page != 2 || usage.Total != 12 || len(usage.Items) != 2 {
		t.Fatalf("usage pagination = %#v", usage)
	}
	var publicUsage struct {
		Items []map[string]any `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/client/usage?page=1&size=1", c.clientToken, nil, &publicUsage); code != http.StatusOK {
		t.Fatalf("public usage status = %d", code)
	}
	if len(publicUsage.Items) != 1 {
		t.Fatalf("public usage items = %#v", publicUsage.Items)
	}
	for _, key := range []string{"id", "userId"} {
		if _, ok := publicUsage.Items[0][key]; ok {
			t.Fatalf("client usage leaked %q: %#v", key, publicUsage.Items[0])
		}
	}
	for _, key := range []string{"createdAt", "model", "inputTokens", "cachedInputTokens", "outputTokens", "totalTokens"} {
		if _, ok := publicUsage.Items[0][key]; !ok {
			t.Fatalf("client usage missing %q: %#v", key, publicUsage.Items[0])
		}
	}

	var recharges struct {
		Items []map[string]any `json:"items"`
		Page  int              `json:"page"`
		Total int              `json:"total"`
	}
	if code := c.request(http.MethodGet, "/api/client/recharges?page=2&size=10", c.clientToken, nil, &recharges); code != http.StatusOK {
		t.Fatalf("recharges status = %d", code)
	}
	if recharges.Page != 2 || recharges.Total != 12 || len(recharges.Items) != 2 {
		t.Fatalf("recharge pagination = %#v", recharges)
	}
	for _, item := range recharges.Items {
		for _, key := range []string{"id", "userId", "userAccount", "topupId", "priceCents", "tokens", "statusTransitions"} {
			if _, ok := item[key]; ok {
				t.Fatalf("client recharge leaked %q: %#v", key, item)
			}
		}
		for _, key := range []string{"topupName", "status", "statusLabel", "submittedAt", "confirmedAt"} {
			if _, ok := item[key]; !ok {
				t.Fatalf("client recharge missing %q: %#v", key, item)
			}
		}
	}
}

func TestClientLedgerSummaryGroupsDailyIncomeAndExpense(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("summary")
	c.loginClient("summary")

	c.store.mu.Lock()
	userID := c.store.state.Users[0].ID
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	c.store.state.TokenLedgers = []TokenLedger{
		{ID: c.store.nextID("led"), UserID: userID, Type: "recharge", DeltaTokens: 100, BalanceAfter: 100, Source: "manual", CreatedAt: today.Add(9 * time.Hour)},
		{ID: c.store.nextID("led"), UserID: userID, Type: "debit", DeltaTokens: -30, BalanceAfter: 70, Source: "usage", CreatedAt: today.Add(10 * time.Hour)},
		{ID: c.store.nextID("led"), UserID: userID, Type: "recharge", DeltaTokens: 50, BalanceAfter: 120, Source: "manual", CreatedAt: today.AddDate(0, 0, -1).Add(11 * time.Hour)},
		{ID: c.store.nextID("led"), UserID: userID, Type: "recharge", DeltaTokens: 999, BalanceAfter: 999, Source: "old", CreatedAt: today.AddDate(0, 0, -30)},
	}
	c.store.mu.Unlock()

	var summary struct {
		Items []struct {
			Date          string `json:"date"`
			IncomeTokens  int64  `json:"incomeTokens"`
			ExpenseTokens int64  `json:"expenseTokens"`
			NetTokens     int64  `json:"netTokens"`
		} `json:"items"`
		Days int `json:"days"`
	}
	if code := c.request(http.MethodGet, "/api/client/ledger/summary?days=30", c.clientToken, nil, &summary); code != http.StatusOK {
		t.Fatalf("ledger summary status = %d", code)
	}
	if summary.Days != 30 || len(summary.Items) != 30 {
		t.Fatalf("summary shape = %#v", summary)
	}
	var ledger struct {
		Items []struct {
			DeltaTokens int64  `json:"deltaTokens"`
			TypeLabel   string `json:"typeLabel"`
		} `json:"items"`
		Total int `json:"total"`
	}
	if code := c.request(http.MethodGet, "/api/client/ledger?page=1&size=10", c.clientToken, nil, &ledger); code != http.StatusOK {
		t.Fatalf("client ledger status = %d", code)
	}
	if ledger.Total != 4 || ledger.Items[0].DeltaTokens != -30 || ledger.Items[0].TypeLabel != "消耗" || ledger.Items[1].TypeLabel != "收入" {
		t.Fatalf("client ledger labels = %#v", ledger)
	}
	var publicLedger struct {
		Items []map[string]any `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/client/ledger?page=1&size=1", c.clientToken, nil, &publicLedger); code != http.StatusOK {
		t.Fatalf("public ledger status = %d", code)
	}
	if len(publicLedger.Items) != 1 {
		t.Fatalf("public ledger items = %#v", publicLedger.Items)
	}
	for _, key := range []string{"id", "userId"} {
		if _, ok := publicLedger.Items[0][key]; ok {
			t.Fatalf("client ledger leaked %q: %#v", key, publicLedger.Items[0])
		}
	}
	for _, key := range []string{"createdAt", "type", "typeLabel", "deltaTokens", "balanceAfter", "source"} {
		if _, ok := publicLedger.Items[0][key]; !ok {
			t.Fatalf("client ledger missing %q: %#v", key, publicLedger.Items[0])
		}
	}
	todayItem := summary.Items[len(summary.Items)-1]
	if todayItem.Date != today.Format("2006-01-02") || todayItem.IncomeTokens != 100 || todayItem.ExpenseTokens != 30 || todayItem.NetTokens != 70 {
		t.Fatalf("today summary = %#v", todayItem)
	}
	yesterdayItem := summary.Items[len(summary.Items)-2]
	if yesterdayItem.IncomeTokens != 50 || yesterdayItem.ExpenseTokens != 0 || yesterdayItem.NetTokens != 50 {
		t.Fatalf("yesterday summary = %#v", yesterdayItem)
	}
}

func TestClientLedgerSourceShowsSafeAdminRemarksAndHidesInternalText(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("ledger-source")
	c.loginClient("ledger-source")

	c.store.mu.Lock()
	userID := c.store.state.Users[0].ID
	now := time.Now().UTC()
	c.store.state.TokenLedgers = []TokenLedger{
		{ID: c.store.nextID("led"), UserID: userID, Type: "adjustment", DeltaTokens: 100, BalanceAfter: 100, Source: "管理员调整：人工校正", CreatedAt: now.Add(4 * time.Minute)},
		{ID: c.store.nextID("led"), UserID: userID, Type: "adjustment", DeltaTokens: 100, BalanceAfter: 200, Source: `管理员调整：internal route C:\Users\1\.codex token=abc`, CreatedAt: now.Add(3 * time.Minute)},
		{ID: c.store.nextID("led"), UserID: userID, Type: "recharge", DeltaTokens: 1000, BalanceAfter: 1200, Source: "Token 充值项：secret topup detail", CreatedAt: now.Add(2 * time.Minute)},
		{ID: c.store.nextID("led"), UserID: userID, Type: "debit", DeltaTokens: -50, BalanceAfter: 1150, Source: "Codex token 用量（upstream route key=abc）", CreatedAt: now.Add(time.Minute)},
		{ID: c.store.nextID("led"), UserID: userID, Type: "unexpected", DeltaTokens: 1, BalanceAfter: 1051, Source: "proxy endpoint base_url", CreatedAt: now},
	}
	c.store.mu.Unlock()

	var ledger struct {
		Items []map[string]any `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/client/ledger?page=1&size=10", c.clientToken, nil, &ledger); code != http.StatusOK {
		t.Fatalf("client ledger status = %d", code)
	}
	if len(ledger.Items) != 5 {
		t.Fatalf("client ledger items = %#v", ledger.Items)
	}
	allowed := map[string]bool{"管理员调整：人工校正": true, "管理员调整": true, "Token 充值项": true, "Codex token 用量": true, "系统记录": true}
	for _, item := range ledger.Items {
		source, _ := item["source"].(string)
		if !allowed[source] {
			t.Fatalf("unexpected public source %q in %#v", source, item)
		}
		for _, forbidden := range []string{"internal route", "C:\\Users", "token=abc", "secret", "upstream", "key=abc", "proxy", "endpoint", "base_url"} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("client ledger source leaked %q: %#v", forbidden, item)
			}
		}
	}
}

func TestAdminUsageLedgerAndAuditPaginateTenItems(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("usage-user")

	c.store.mu.Lock()
	userID := c.store.state.Users[0].ID
	now := time.Now().UTC()
	c.store.state.UsageRecords = nil
	c.store.state.TokenLedgers = nil
	c.store.state.AuditLogs = nil
	for i := 0; i < 12; i++ {
		c.store.state.UsageRecords = append(c.store.state.UsageRecords, UsageRecord{ID: c.store.nextID("use"), UserID: userID, Model: "gpt-5.5", InputTokens: int64(i + 1), OutputTokens: 1, TotalTokens: int64(i + 2), CreatedAt: now.Add(time.Duration(i) * time.Minute)})
		source := "管理员调整：人工校正"
		if i == 10 {
			source = `管理员调整：internal route C:\Users\1\.codex token=abc api key=secret proxy endpoint base_url`
		}
		c.store.state.TokenLedgers = append(c.store.state.TokenLedgers, TokenLedger{ID: c.store.nextID("led"), UserID: userID, Type: "adjustment", DeltaTokens: int64(i + 1), BalanceAfter: int64(i + 1), Source: source, CreatedAt: now.Add(time.Duration(i) * time.Minute)})
		c.store.state.AuditLogs = append(c.store.state.AuditLogs, AuditLog{ID: c.store.nextID("aud"), ActorID: "admin", ActorRole: "admin", Action: "test.action", TargetID: fmt.Sprintf("target-%d", i), CreatedAt: now.Add(time.Duration(i) * time.Minute)})
	}
	c.store.mu.Unlock()

	var usage struct {
		Items []struct {
			UserID      string `json:"userId"`
			UserAccount string `json:"userAccount"`
		} `json:"items"`
		Page  int `json:"page"`
		Total int `json:"total"`
	}
	if code := c.request(http.MethodGet, "/api/admin/usage?page=2&size=10", c.adminToken, nil, &usage); code != http.StatusOK {
		t.Fatalf("admin usage status = %d", code)
	}
	if usage.Page != 2 || usage.Total != 12 || len(usage.Items) != 2 {
		t.Fatalf("admin usage pagination = %#v", usage)
	}
	if usage.Items[0].UserID != userID || usage.Items[0].UserAccount != "usage-user" {
		t.Fatalf("admin usage user display = %#v", usage.Items[0])
	}

	var ledger struct {
		Items []struct {
			UserID      string `json:"userId"`
			UserAccount string `json:"userAccount"`
			TypeLabel   string `json:"typeLabel"`
			Source      string `json:"source"`
		} `json:"items"`
		Page  int `json:"page"`
		Total int `json:"total"`
	}
	if code := c.request(http.MethodGet, "/api/admin/ledger?page=2&size=10", c.adminToken, nil, &ledger); code != http.StatusOK {
		t.Fatalf("admin ledger status = %d", code)
	}
	if ledger.Page != 2 || ledger.Total != 12 || len(ledger.Items) != 2 {
		t.Fatalf("admin ledger pagination = %#v", ledger)
	}
	if ledger.Items[0].UserID != userID || ledger.Items[0].UserAccount != "usage-user" || ledger.Items[0].TypeLabel != "收入" {
		t.Fatalf("admin ledger user display = %#v", ledger.Items[0])
	}
	for _, item := range ledger.Items {
		for _, forbidden := range []string{"internal route", "C:\\Users", "token=abc", "api key", "secret", "proxy", "endpoint", "base_url"} {
			if strings.Contains(item.Source, forbidden) {
				t.Fatalf("admin ledger source leaked %q: %#v", forbidden, item)
			}
		}
	}

	var audit struct {
		Items []struct {
			ActorID      string `json:"actorId"`
			ActorRole    string `json:"actorRole"`
			ActorAccount string `json:"actorAccount"`
		} `json:"items"`
		Page  int `json:"page"`
		Total int `json:"total"`
	}
	if code := c.request(http.MethodGet, "/api/admin/audit?page=2&size=10", c.adminToken, nil, &audit); code != http.StatusOK {
		t.Fatalf("admin audit status = %d", code)
	}
	if audit.Page != 2 || audit.Total != 12 || len(audit.Items) != 2 {
		t.Fatalf("admin audit pagination = %#v", audit)
	}
	if audit.Items[0].ActorID != "admin" || audit.Items[0].ActorRole != "admin" {
		t.Fatalf("admin audit actor = %#v", audit.Items[0])
	}
}

func TestAdminOverviewUsesUTCDayForTodayTokens(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()

	c.store.mu.Lock()
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	c.store.state.UsageRecords = []UsageRecord{
		{ID: c.store.nextID("use"), UserID: "usr", Model: "gpt-5.5", TotalTokens: 70, CreatedAt: today.Add(time.Hour)},
		{ID: c.store.nextID("use"), UserID: "usr", Model: "gpt-5.5", TotalTokens: 900, CreatedAt: today.Add(-time.Nanosecond)},
		{ID: c.store.nextID("use"), UserID: "usr", Model: "gpt-5.5", TotalTokens: 800, CreatedAt: today.Add(24 * time.Hour)},
	}
	c.store.mu.Unlock()

	var overview struct {
		TodayTokens int64 `json:"todayTokens"`
	}
	if code := c.request(http.MethodGet, "/api/admin/overview", c.adminToken, nil, &overview); code != http.StatusOK {
		t.Fatalf("overview status = %d", code)
	}
	if overview.TodayTokens != 70 {
		t.Fatalf("today tokens = %d", overview.TodayTokens)
	}
	var usage struct {
		Items []map[string]any `json:"items"`
		Total int              `json:"total"`
	}
	if code := c.request(http.MethodGet, "/api/admin/usage?today=true", c.adminToken, nil, &usage); code != http.StatusOK {
		t.Fatalf("today usage status = %d", code)
	}
	if usage.Total != 1 || len(usage.Items) != 1 || int64(usage.Items[0]["totalTokens"].(float64)) != 70 {
		t.Fatalf("today usage = %#v", usage)
	}
}

func TestAdminOverviewMetricJumpsResetOperationalFilters(t *testing.T) {
	raw, err := adminFiles.ReadFile("web/admin/index.html")
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, required := range []string{
		`data-jump="recharges"`,
		`data-jump="upstreams" data-available="true"`,
		`data-jump="usage" data-today="true"`,
		"state.rechargePage = 1;",
		"state.upstreamAvailable = btn.dataset.available;",
		"state.upstreamPage = 1;",
		"state.usageToday = btn.dataset.today;",
		"state.usagePage = 1;",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("admin overview jump behavior missing %q", required)
		}
	}
	if strings.Contains(text, "异常数量") {
		t.Fatal("admin overview must not show undefined anomaly count")
	}
}

func TestZeroTokenBalanceRejectsLaunchPrepareAndGateway(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("bob")
	c.loginClient("bob")

	var upstream struct {
		ID string `json:"id"`
	}
	upstreamBody := map[string]any{"name": "codex-upstream", "group": "default", "credentialType": "oauth", "accessToken": "access", "refreshToken": "refresh"}
	if code := c.request(http.MethodPost, "/api/admin/upstreams", c.adminToken, upstreamBody, &upstream); code != http.StatusCreated {
		t.Fatalf("upstream status = %d", code)
	}
	if code := c.request(http.MethodPost, "/api/admin/api-keys", c.adminToken, map[string]any{"upstreamAccountId": upstream.ID}, nil); code != http.StatusCreated {
		t.Fatalf("api key status = %d", code)
	}
	var prepare map[string]any
	if code := c.request(http.MethodPost, "/api/client/launch/prepare", c.clientToken, map[string]any{}, &prepare); code != http.StatusPaymentRequired {
		t.Fatalf("launch prepare status with zero token balance = %d", code)
	}
	if prepare["error"] != "token_not_available" {
		t.Fatalf("launch prepare response = %#v", prepare)
	}
	for _, key := range []string{"route", "routeId", "apiKey", "upstream", "gatewayPath", "provider"} {
		if _, ok := prepare[key]; ok {
			t.Fatalf("launch prepare leaked route supply %q: %#v", key, prepare)
		}
	}
	var gatewayErr apiError
	if code := c.gatewayRunWithHeaders(c.clientToken, map[string]string{"Idempotency-Key": "zero-token-request"}, map[string]any{"model": "gpt-5.5", "input": "blocked by zero balance"}, &gatewayErr); code != http.StatusPaymentRequired {
		t.Fatalf("gateway status with zero token balance = %d", code)
	}
	if gatewayErr.Error != "token_not_available" {
		t.Fatalf("gateway zero-token error = %#v", gatewayErr)
	}
}

func TestClientLaunchPrepareRejectsRouteSupplyFields(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("prepare-clean")
	c.loginClient("prepare-clean")

	var invalid apiError
	body := map[string]any{"route": "old-route", "apiKey": "old-key", "upstream": "old-upstream"}
	if code := c.request(http.MethodPost, "/api/client/launch/prepare", c.clientToken, body, &invalid); code != http.StatusBadRequest {
		t.Fatalf("launch prepare with old route fields status = %d", code)
	}
	if invalid.Error != "invalid_launch_prepare_request" {
		t.Fatalf("launch prepare with old route fields error = %#v", invalid)
	}
}

func TestGatewayReservedTokensReduceAvailableBalance(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("reserved")
	c.loginClient("reserved")
	c.approvePaidRecharge()
	c.createRoutedUpstream("")

	c.store.mu.Lock()
	userID := c.store.state.Users[0].ID
	now := time.Now().UTC()
	c.store.state.GatewayRequests = append(c.store.state.GatewayRequests, GatewayRequest{
		ID:             c.store.nextID("gw"),
		UserID:         userID,
		RequestID:      "held-request",
		Status:         gatewayReserved,
		ReservedTokens: c.store.state.Users[0].TokenBalance,
		CreatedAt:      now,
		UpdatedAt:      now,
	})
	c.store.mu.Unlock()

	var adjustmentErr apiError
	if code := c.request(http.MethodPost, "/api/admin/users/"+userID+"/token-adjustments", c.adminToken, map[string]any{"deltaTokens": -1}, &adjustmentErr); code != http.StatusBadRequest {
		t.Fatalf("reserved balance adjustment status = %d", code)
	}
	if adjustmentErr.Error != "token_not_available" {
		t.Fatalf("reserved balance adjustment error = %#v", adjustmentErr)
	}
	var me struct {
		User map[string]any `json:"user"`
	}
	if code := c.request(http.MethodGet, "/api/client/me", c.clientToken, nil, &me); code != http.StatusOK {
		t.Fatalf("me status after adjustment = %d", code)
	}
	if got := int64(me.User["tokenBalance"].(float64)); got != 100000 {
		t.Fatalf("token balance after rejected adjustment = %d", got)
	}
	var prepare map[string]any
	if code := c.request(http.MethodPost, "/api/client/launch/prepare", c.clientToken, map[string]any{}, &prepare); code != http.StatusPaymentRequired {
		t.Fatalf("launch prepare status with reserved balance = %d", code)
	}
	if prepare["error"] != "token_not_available" {
		t.Fatalf("launch prepare with reserved balance = %#v", prepare)
	}
	if code := c.gatewayRunWithHeaders(c.clientToken, map[string]string{"Idempotency-Key": "second-request"}, map[string]any{"model": "gpt-5.5", "input": "blocked by reservation"}, nil); code != http.StatusPaymentRequired {
		t.Fatalf("gateway status with reserved balance = %d", code)
	}
}

func TestClientRoutesNextIsNotExposed(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("route-user")
	c.loginClient("route-user")
	c.approvePaidRecharge()
	c.createRoutedUpstream("http://127.0.0.1:1/run")

	var out apiError
	if code := c.request(http.MethodPost, "/api/client/routes/next", c.clientToken, map[string]any{}, &out); code != http.StatusNotFound {
		t.Fatalf("route endpoint status = %d", code)
	}
	if out.Error != "not_found" {
		t.Fatalf("route endpoint error = %#v", out)
	}
	var me struct {
		Service map[string]any `json:"service"`
	}
	if code := c.request(http.MethodGet, "/api/client/me", c.clientToken, nil, &me); code != http.StatusOK {
		t.Fatalf("me status = %d", code)
	}
	if me.Service["status"] != "可用" {
		t.Fatalf("service status = %#v", me.Service)
	}
}

func TestAdminCheckUpstreamUsesCodexAppServerProbe(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	originalProbe := codexAppServerProbe
	codexAppServerProbe = func(ctx context.Context, credentials codexProbeCredentials) (codexProbeResult, error) {
		if credentials.AccessToken != "access" || credentials.ChatGPTAccountID != "account-123" {
			t.Fatalf("probe credentials = %#v", credentials)
		}
		usedPercent := 25.5
		creditBalance := 12.75
		resetsAt := time.Date(2026, 7, 5, 1, 2, 3, 0, time.UTC)
		return codexProbeResult{AccountType: "chatgpt", Email: "codex@example.com", PlanType: "pro", UsageTokens: 12345, RateLimitUsedPercent: &usedPercent, RateLimitResetsAt: &resetsAt, CreditBalance: &creditBalance, CreditBalanceLabel: "12.75"}, nil
	}
	t.Cleanup(func() { codexAppServerProbe = originalProbe })

	var upstream struct {
		ID string `json:"id"`
	}
	upstreamBody := map[string]any{"name": "codex-upstream", "group": "default", "credentialType": "oauth", "accessToken": "access", "refreshToken": "refresh", "chatgptAccountId": "account-123"}
	if code := c.request(http.MethodPost, "/api/admin/upstreams", c.adminToken, upstreamBody, &upstream); code != http.StatusCreated {
		t.Fatalf("upstream status = %d", code)
	}
	var checked struct {
		ID                    string   `json:"id"`
		Email                 string   `json:"email"`
		SubscriptionTier      string   `json:"subscriptionTier"`
		EntitlementStatus     string   `json:"entitlementStatus"`
		AvailabilityStatus    string   `json:"availabilityStatus"`
		UsageTokens           int64    `json:"usageTokens"`
		AccountType           string   `json:"accountType"`
		CredentialFingerprint string   `json:"credentialFingerprint"`
		RateLimitUsedPercent  *float64 `json:"rateLimitUsedPercent"`
		RemainingPercent      *float64 `json:"rateLimitRemainingPercent"`
		CreditBalance         *float64 `json:"creditBalance"`
	}
	if code := c.request(http.MethodPost, "/api/admin/upstreams/"+upstream.ID+"/check", c.adminToken, nil, &checked); code != http.StatusOK {
		t.Fatalf("check status = %d", code)
	}
	if checked.Email != "codex@example.com" || checked.SubscriptionTier != "pro" || checked.EntitlementStatus != "available" || checked.AvailabilityStatus != "available" {
		t.Fatalf("checked upstream = %#v", checked)
	}
	if checked.UsageTokens != 12345 || checked.AccountType != "chatgpt" {
		t.Fatalf("probe fields = %#v", checked)
	}
	if checked.RateLimitUsedPercent == nil || *checked.RateLimitUsedPercent != 25.5 || checked.RemainingPercent == nil || *checked.RemainingPercent != 74.5 || checked.CreditBalance == nil || *checked.CreditBalance != 12.75 {
		t.Fatalf("probe balance fields = %#v", checked)
	}
	if checked.CredentialFingerprint != "" {
		t.Fatalf("credential fingerprint leaked: %#v", checked)
	}
	var upstreams struct {
		Items []map[string]any `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/admin/upstreams", c.adminToken, nil, &upstreams); code != http.StatusOK {
		t.Fatalf("list checked upstream status = %d", code)
	}
	if len(upstreams.Items) != 1 || upstreams.Items[0]["usageTokens"] != float64(12345) || upstreams.Items[0]["rateLimitRemainingPercent"] != 74.5 || upstreams.Items[0]["creditBalance"] != 12.75 || upstreams.Items[0]["rateLimitResetsAt"] == nil {
		t.Fatalf("checked upstream list fields = %#v", upstreams.Items)
	}
}

func TestAdminCheckUpstreamAuditHidesRawProbeFailure(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	rawFailure := "raw secret path C:\\Users\\1\\.codex\\config.toml token=abc"
	originalProbe := codexAppServerProbe
	codexAppServerProbe = func(ctx context.Context, credentials codexProbeCredentials) (codexProbeResult, error) {
		return codexProbeResult{}, errors.New(rawFailure)
	}
	t.Cleanup(func() { codexAppServerProbe = originalProbe })

	var upstream struct {
		ID string `json:"id"`
	}
	upstreamBody := map[string]any{"name": "failing-upstream", "group": "default", "credentialType": "oauth", "accessToken": "access", "chatgptAccountId": "account-123"}
	if code := c.request(http.MethodPost, "/api/admin/upstreams", c.adminToken, upstreamBody, &upstream); code != http.StatusCreated {
		t.Fatalf("upstream status = %d", code)
	}

	var checked struct {
		EntitlementStatus  string `json:"entitlementStatus"`
		AvailabilityStatus string `json:"availabilityStatus"`
	}
	if code := c.request(http.MethodPost, "/api/admin/upstreams/"+upstream.ID+"/check", c.adminToken, nil, &checked); code != http.StatusOK {
		t.Fatalf("check status = %d", code)
	}
	if checked.EntitlementStatus != "check_failed" || checked.AvailabilityStatus != "unavailable" {
		t.Fatalf("checked failure status = %#v", checked)
	}

	var audit struct {
		Items []AuditLog `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/admin/audit?size=100", c.adminToken, nil, &audit); code != http.StatusOK {
		t.Fatalf("audit status = %d", code)
	}
	found := false
	for _, item := range audit.Items {
		if item.Action != "upstream.check" || item.TargetID != upstream.ID {
			continue
		}
		found = true
		if item.Detail != "check_failed" {
			t.Fatalf("probe failure detail leaked raw error: %#v", item)
		}
		if strings.Contains(item.Detail, "raw secret path") || strings.Contains(item.Detail, "token=abc") || strings.Contains(item.Detail, "C:\\Users") {
			t.Fatalf("probe failure detail contains sensitive raw text: %#v", item)
		}
	}
	if !found {
		t.Fatalf("upstream.check audit entry not found: %#v", audit.Items)
	}
}

func TestAdminCheckUpstreamClassifiesSafeProbeFailure(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	originalProbe := codexAppServerProbe
	codexAppServerProbe = func(ctx context.Context, credentials codexProbeCredentials) (codexProbeResult, error) {
		return codexProbeResult{}, errors.New("codex_app_server_http_401")
	}
	t.Cleanup(func() { codexAppServerProbe = originalProbe })

	var upstream struct {
		ID string `json:"id"`
	}
	if code := c.request(http.MethodPost, "/api/admin/upstreams", c.adminToken, map[string]any{"name": "auth-failing", "credentialType": "oauth", "accessToken": "access", "chatgptAccountId": "account-123"}, &upstream); code != http.StatusCreated {
		t.Fatalf("upstream status = %d", code)
	}
	var checked struct {
		EntitlementStatus  string `json:"entitlementStatus"`
		AvailabilityStatus string `json:"availabilityStatus"`
	}
	if code := c.request(http.MethodPost, "/api/admin/upstreams/"+upstream.ID+"/check", c.adminToken, nil, &checked); code != http.StatusOK {
		t.Fatalf("check status = %d", code)
	}
	if checked.EntitlementStatus != "auth_failed" || checked.AvailabilityStatus != "unavailable" {
		t.Fatalf("checked auth failure = %#v", checked)
	}
}

func TestAdminCanReplaceUpstreamCredentialsWithoutLeakingSecrets(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()

	var upstream map[string]any
	createBody := map[string]any{
		"name":              "old-upstream",
		"group":             "old-group",
		"credentialType":    "oauth",
		"accessToken":       "old-access",
		"refreshToken":      "old-refresh",
		"tokenType":         "Bearer",
		"chatgptAccountId":  "old-account",
		"email":             "old@example.com",
		"subscriptionTier":  "plus",
		"entitlementStatus": "available",
	}
	if code := c.request(http.MethodPost, "/api/admin/upstreams", c.adminToken, createBody, &upstream); code != http.StatusCreated {
		t.Fatalf("create upstream status = %d", code)
	}
	upstreamID, _ := upstream["id"].(string)
	if upstreamID == "" {
		t.Fatalf("upstream id missing: %#v", upstream)
	}
	if code := c.request(http.MethodPost, "/api/admin/upstreams/"+upstreamID+"/status", c.adminToken, map[string]any{"enabled": false}, nil); code != http.StatusOK {
		t.Fatalf("disable upstream status = %d", code)
	}

	replaceBody := map[string]any{
		"name":              "new-upstream",
		"group":             "new-group",
		"credentialType":    "oauth",
		"accessToken":       "new-access",
		"refreshToken":      "new-refresh",
		"tokenType":         "Bearer",
		"chatgptAccountId":  "new-account",
		"email":             "new@example.com",
		"subscriptionTier":  "pro",
		"entitlementStatus": "unchecked",
	}
	var replaced map[string]any
	if code := c.request(http.MethodPost, "/api/admin/upstreams/"+upstreamID+"/credentials", c.adminToken, replaceBody, &replaced); code != http.StatusOK {
		t.Fatalf("replace upstream credentials status = %d", code)
	}
	if replaced["id"] != upstreamID || replaced["name"] != "new-upstream" || replaced["group"] != "new-group" || replaced["chatgptAccountId"] != "new-account" || replaced["email"] != "new@example.com" || replaced["subscriptionTier"] != "pro" || replaced["entitlementStatus"] != "unchecked" {
		t.Fatalf("replaced upstream metadata = %#v", replaced)
	}
	if replaced["availabilityStatus"] != "available" || replaced["enabled"] != false {
		t.Fatalf("disabled scheduling must survive credential replacement: %#v", replaced)
	}
	for _, key := range []string{"accessToken", "refreshToken", "credentialFingerprint", "accessTokenCipher", "refreshTokenCipher", "status", "balanceStatus", "riskStatus"} {
		if _, ok := replaced[key]; ok {
			t.Fatalf("credential replacement response leaked %q: %#v", key, replaced)
		}
	}
	if replaced["lastCheckedAt"] != nil {
		t.Fatalf("credential replacement must clear stale check time: %#v", replaced)
	}

	var invalid apiError
	if code := c.request(http.MethodPost, "/api/admin/upstreams/"+upstreamID+"/credentials", c.adminToken, map[string]any{"accessToken": "another", "unexpectedField": "unexpected"}, &invalid); code != http.StatusBadRequest {
		t.Fatalf("replace upstream credentials unknown field status = %d", code)
	}
	if invalid.Error != "invalid_upstream_request" {
		t.Fatalf("replace upstream credentials unknown field error = %#v", invalid)
	}

	originalProbe := codexAppServerProbe
	codexAppServerProbe = func(ctx context.Context, credentials codexProbeCredentials) (codexProbeResult, error) {
		if credentials.AccessToken != "new-access" || credentials.ChatGPTAccountID != "new-account" {
			t.Fatalf("probe used stale credentials %#v", credentials)
		}
		return codexProbeResult{AccountType: "chatgpt", Email: "checked@example.com", PlanType: "team", UsageTokens: 9}, nil
	}
	t.Cleanup(func() { codexAppServerProbe = originalProbe })
	if code := c.request(http.MethodPost, "/api/admin/upstreams/"+upstreamID+"/check", c.adminToken, nil, nil); code != http.StatusOK {
		t.Fatalf("check replaced upstream status = %d", code)
	}

	var audit struct {
		Items []AuditLog `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/admin/audit?size=100", c.adminToken, nil, &audit); code != http.StatusOK {
		t.Fatalf("audit status = %d", code)
	}
	found := false
	for _, item := range audit.Items {
		if item.Action != "upstream.credentials.replace" {
			continue
		}
		found = true
		if item.TargetID != upstreamID || item.Detail != "credentials_replaced" {
			t.Fatalf("replace credentials audit entry = %#v", item)
		}
		for _, secret := range []string{"old-access", "old-refresh", "new-access", "new-refresh"} {
			if strings.Contains(item.TargetID, secret) || strings.Contains(item.Detail, secret) {
				t.Fatalf("replace credentials audit leaked secret %q: %#v", secret, item)
			}
		}
	}
	if !found {
		t.Fatalf("upstream.credentials.replace audit entry not found: %#v", audit.Items)
	}
}

func TestAdminUpstreamAndAPIKeyStatusAreCleanAndSanitized(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	var upstream map[string]any
	expiresAt := time.Date(2027, 2, 3, 4, 5, 6, 0, time.UTC).Format(time.RFC3339)
	body := map[string]any{
		"name":              "clean-upstream",
		"group":             "ops",
		"credentialType":    "oauth",
		"accessToken":       "access",
		"refreshToken":      "refresh",
		"tokenType":         "Bearer",
		"expiresAt":         expiresAt,
		"unexpectedField":   "unexpected",
		"email":             "codex@example.com",
		"subscriptionTier":  "pro",
		"entitlementStatus": "available",
	}
	var invalid apiError
	if code := c.request(http.MethodPost, "/api/admin/upstreams", c.adminToken, body, &invalid); code != http.StatusBadRequest {
		t.Fatalf("unknown upstream field status = %d", code)
	}
	if invalid.Error != "invalid_upstream_request" {
		t.Fatalf("unknown upstream field error = %#v", invalid)
	}
	delete(body, "unexpectedField")
	blankAccess := map[string]any{
		"name":           "blank-access",
		"group":          "ops",
		"credentialType": "oauth",
		"accessToken":    "   ",
		"refreshToken":   "refresh",
	}
	var blankAccessErr apiError
	if code := c.request(http.MethodPost, "/api/admin/upstreams", c.adminToken, blankAccess, &blankAccessErr); code != http.StatusBadRequest {
		t.Fatalf("blank access token upstream status = %d", code)
	}
	if blankAccessErr.Error != "invalid_upstream_request" {
		t.Fatalf("blank access token upstream error = %#v", blankAccessErr)
	}
	unsupportedCredential := map[string]any{
		"name":           "unsupported-credential",
		"group":          "ops",
		"credentialType": "api_key",
		"accessToken":    "access",
		"refreshToken":   "refresh",
	}
	var unsupportedCredentialErr apiError
	if code := c.request(http.MethodPost, "/api/admin/upstreams", c.adminToken, unsupportedCredential, &unsupportedCredentialErr); code != http.StatusBadRequest {
		t.Fatalf("unsupported credential upstream status = %d", code)
	}
	if unsupportedCredentialErr.Error != "invalid_upstream_request" {
		t.Fatalf("unsupported credential upstream error = %#v", unsupportedCredentialErr)
	}
	if code := c.request(http.MethodPost, "/api/admin/upstreams", c.adminToken, body, &upstream); code != http.StatusCreated {
		t.Fatalf("upstream status = %d", code)
	}
	upstreamID, _ := upstream["id"].(string)
	if upstreamID == "" {
		t.Fatalf("upstream id missing: %#v", upstream)
	}
	for _, key := range []string{"accessToken", "refreshToken", "credentialFingerprint", "status", "balanceStatus", "riskStatus"} {
		if _, ok := upstream[key]; ok {
			t.Fatalf("upstream field %q leaked in create response: %#v", key, upstream)
		}
	}
	if upstream["credentialType"] != "oauth" || upstream["tokenType"] != "Bearer" || upstream["email"] != "codex@example.com" || upstream["subscriptionTier"] != "pro" || upstream["entitlementStatus"] != "available" {
		t.Fatalf("upstream metadata = %#v", upstream)
	}
	if upstream["availabilityStatus"] != "available" {
		t.Fatalf("upstream availability = %#v", upstream)
	}
	for _, key := range []string{"unexpectedField", "unexpectedConfigured", "unexpectedCipher"} {
		if _, ok := upstream[key]; ok {
			t.Fatalf("unknown upstream field %q leaked in create response: %#v", key, upstream)
		}
	}
	var legacyAvailability apiError
	if code := c.request(http.MethodPost, "/api/admin/upstreams/"+upstreamID+"/status", c.adminToken, map[string]any{"availabilityStatus": "paused"}, &legacyAvailability); code != http.StatusBadRequest {
		t.Fatalf("legacy upstream availability code = %d", code)
	}
	if legacyAvailability.Error != "invalid_upstream_status_request" {
		t.Fatalf("legacy upstream availability error = %#v", legacyAvailability)
	}
	var splitStatusErr apiError
	if code := c.request(http.MethodPost, "/api/admin/upstreams/"+upstreamID+"/status", c.adminToken, map[string]any{"balanceStatus": "empty"}, &splitStatusErr); code != http.StatusBadRequest {
		t.Fatalf("split upstream status field code = %d", code)
	}
	if splitStatusErr.Error != "invalid_upstream_status_request" {
		t.Fatalf("split upstream status field error = %#v", splitStatusErr)
	}
	if code := c.request(http.MethodPost, "/api/admin/upstreams/"+upstreamID+"/status", c.adminToken, map[string]any{"enabled": false}, nil); code != http.StatusOK {
		t.Fatalf("update upstream status = %d", code)
	}
	var upstreams struct {
		Items []map[string]any `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/admin/upstreams", c.adminToken, nil, &upstreams); code != http.StatusOK {
		t.Fatalf("list upstreams = %d", code)
	}
	for _, key := range []string{"credentialFingerprint", "status", "balanceStatus", "riskStatus", "unexpectedField", "unexpectedConfigured", "unexpectedCipher"} {
		if _, ok := upstreams.Items[0][key]; ok {
			t.Fatalf("upstream field %q leaked in list: %#v", key, upstreams.Items[0])
		}
	}
	if upstreams.Items[0]["tokenType"] != "Bearer" || upstreams.Items[0]["expiresAt"] == "" || upstreams.Items[0]["email"] != "codex@example.com" || upstreams.Items[0]["subscriptionTier"] != "pro" || upstreams.Items[0]["entitlementStatus"] != "available" {
		t.Fatalf("upstream list metadata = %#v", upstreams.Items[0])
	}
	if upstreams.Items[0]["availabilityStatus"] != "available" || upstreams.Items[0]["enabled"] != false {
		t.Fatalf("upstream list availability/enabled = %#v", upstreams.Items[0])
	}
	var disabledAvailableUpstreams struct {
		Items []map[string]any `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/admin/upstreams?available=true", c.adminToken, nil, &disabledAvailableUpstreams); code != http.StatusOK {
		t.Fatalf("disabled available upstreams status = %d", code)
	}
	if len(disabledAvailableUpstreams.Items) != 0 {
		t.Fatalf("disabled upstream must not be schedulable = %#v", disabledAvailableUpstreams.Items)
	}
	if code := c.request(http.MethodPost, "/api/admin/upstreams/"+upstreamID+"/status", c.adminToken, map[string]any{"enabled": true}, nil); code != http.StatusOK {
		t.Fatalf("restore upstream availability status = %d", code)
	}
	var availableUpstreams struct {
		Items []map[string]any `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/admin/upstreams?available=true", c.adminToken, nil, &availableUpstreams); code != http.StatusOK {
		t.Fatalf("available upstreams status = %d", code)
	}
	if len(availableUpstreams.Items) != 1 || availableUpstreams.Items[0]["availabilityStatus"] != "available" || availableUpstreams.Items[0]["enabled"] != true {
		t.Fatalf("available upstreams = %#v", availableUpstreams.Items)
	}

	var key map[string]any
	var invalidKeyCreate apiError
	if code := c.request(http.MethodPost, "/api/admin/api-keys", c.adminToken, map[string]any{"upstreamAccountId": upstreamID, "unexpectedField": "unexpected"}, &invalidKeyCreate); code != http.StatusBadRequest {
		t.Fatalf("api key create unknown field status = %d", code)
	}
	if invalidKeyCreate.Error != "invalid_api_key_request" {
		t.Fatalf("api key create unknown field error = %#v", invalidKeyCreate)
	}
	if code := c.request(http.MethodPost, "/api/admin/api-keys", c.adminToken, map[string]any{"upstreamAccountId": upstreamID}, &key); code != http.StatusCreated {
		t.Fatalf("create api key = %d", code)
	}
	if _, ok := key["secretOnce"]; ok {
		t.Fatalf("api key secret leaked: %#v", key)
	}
	if _, ok := key["keyHash"]; ok {
		t.Fatalf("api key hash leaked: %#v", key)
	}
	if _, ok := key["publicPrefix"]; ok {
		t.Fatalf("api key stable prefix leaked: %#v", key)
	}
	for _, leaked := range []string{"upstreamBalanceStatus", "upstreamRiskStatus", "upstreamStatus"} {
		if _, ok := key[leaked]; ok {
			t.Fatalf("api key upstream split status leaked %q: %#v", leaked, key)
		}
	}
	if key["routeAvailable"] != true {
		t.Fatalf("api key route availability = %#v", key)
	}
	keyID, _ := key["id"].(string)
	if keyID == "" {
		t.Fatalf("api key id missing: %#v", key)
	}
	var keys struct {
		Items []map[string]any `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/admin/api-keys", c.adminToken, nil, &keys); code != http.StatusOK {
		t.Fatalf("list api keys = %d", code)
	}
	if len(keys.Items) != 1 || keys.Items[0]["routeAvailable"] != true {
		t.Fatalf("api key list route availability = %#v", keys)
	}
	for _, leaked := range []string{"upstreamBalanceStatus", "upstreamRiskStatus", "upstreamStatus"} {
		if _, ok := keys.Items[0][leaked]; ok {
			t.Fatalf("api key list upstream split status leaked %q: %#v", leaked, keys.Items[0])
		}
	}
	var invalidKeyStatus apiError
	if code := c.request(http.MethodPost, "/api/admin/api-keys/"+keyID+"/status", c.adminToken, map[string]any{"status": "disabled", "quota": 0}, &invalidKeyStatus); code != http.StatusBadRequest {
		t.Fatalf("api key status unknown field status = %d", code)
	}
	if invalidKeyStatus.Error != "invalid_api_key_status_request" {
		t.Fatalf("api key status unknown field error = %#v", invalidKeyStatus)
	}
	if code := c.request(http.MethodPost, "/api/admin/api-keys/"+keyID+"/status", c.adminToken, map[string]any{"status": "paused"}, nil); code != http.StatusBadRequest {
		t.Fatalf("invalid api key status code = %d", code)
	}
	if code := c.request(http.MethodPost, "/api/admin/api-keys/"+keyID+"/status", c.adminToken, map[string]any{"status": "disabled"}, nil); code != http.StatusOK {
		t.Fatalf("disable api key = %d", code)
	}
}

func TestAdminImportUpstreamsRecognizesAccountFlowExport(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	body := map[string]any{
		"accounts": []any{
			map[string]any{
				"name":     "codex@example.com",
				"platform": "openai",
				"type":     "codex",
				"credentials": map[string]any{
					"access_token":       "access-secret",
					"refresh_token":      "refresh-secret",
					"chatgpt_account_id": "chatgpt-account",
					"email":              "codex@example.com",
					"expires_at":         1783910187,
					"plan_type":          "plus",
					"organization_id":    "org_unused",
				},
			},
		},
		"proxies": []any{},
	}
	var imported struct {
		Imported int              `json:"imported"`
		Items    []map[string]any `json:"items"`
	}
	if code := c.request(http.MethodPost, "/api/admin/upstreams/import", c.adminToken, body, &imported); code != http.StatusCreated {
		t.Fatalf("import upstream status = %d", code)
	}
	if imported.Imported != 1 || len(imported.Items) != 1 {
		t.Fatalf("imported upstreams = %#v", imported)
	}
	item := imported.Items[0]
	if item["name"] != "codex@example.com" || item["credentialType"] != "oauth" || item["tokenType"] != "Bearer" || item["chatgptAccountId"] != "chatgpt-account" || item["email"] != "codex@example.com" || item["subscriptionTier"] != "plus" {
		t.Fatalf("imported upstream item = %#v", item)
	}
	for _, key := range []string{"accessToken", "refreshToken", "credentialFingerprint", "accessTokenCipher", "refreshTokenCipher"} {
		if _, ok := item[key]; ok {
			t.Fatalf("imported upstream leaked %q: %#v", key, item)
		}
	}
	rawResp, err := json.Marshal(imported)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"access-secret", "refresh-secret"} {
		if strings.Contains(string(rawResp), secret) {
			t.Fatalf("import response leaked secret %q: %s", secret, rawResp)
		}
	}
	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	if len(c.store.state.UpstreamAccounts) != 1 {
		t.Fatalf("stored upstreams = %#v", c.store.state.UpstreamAccounts)
	}
	up := c.store.state.UpstreamAccounts[0]
	if up.AccessTokenCipher == "access-secret" || up.RefreshTokenCipher == "refresh-secret" {
		t.Fatalf("stored upstream credential is plaintext: %#v", up)
	}
	access, err := c.app.decrypt(up.AccessTokenCipher)
	if err != nil {
		t.Fatal(err)
	}
	refresh, err := c.app.decrypt(up.RefreshTokenCipher)
	if err != nil {
		t.Fatal(err)
	}
	if access != "access-secret" || refresh != "refresh-secret" {
		t.Fatalf("stored upstream secrets mismatch: access=%q refresh=%q", access, refresh)
	}
	if up.ChatGPTAccountID != "chatgpt-account" {
		t.Fatalf("stored chatgpt account id = %q", up.ChatGPTAccountID)
	}
	expectedExpiry := time.Unix(1783910187, 0).UTC()
	if up.ExpiresAt == nil || !up.ExpiresAt.Equal(expectedExpiry) {
		t.Fatalf("stored upstream expiry = %#v, want %s", up.ExpiresAt, expectedExpiry.Format(time.RFC3339))
	}
}

func TestChatGPTAccountIDFromAccessTokenClaims(t *testing.T) {
	payload, err := json.Marshal(map[string]any{
		"https://api.openai.com/auth": map[string]any{
			"user_id": "user-from-claims",
			"poid":    "org-from-claims",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	token := "header." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"
	if got := chatGPTAccountIDFromAccessToken(token); got != "user-from-claims" {
		t.Fatalf("chatgpt account id = %q", got)
	}
}

func TestAdminUpstreamUIShowsSingleAvailabilityState(t *testing.T) {
	raw, err := adminFiles.ReadFile("web/admin/index.html")
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, required := range []string{
		"availabilityStatus",
		"enabled",
		"data-up-enabled",
		"switch-toggle",
		"账号状态",
		"可调度账号",
		"upstreamCredentialEditId",
		"替换 Codex 上游账号凭据",
		"data-up-credentials",
		"data-cancel-upstream-credentials",
		"/credentials",
		"已替换 Codex 上游账号凭据",
		"自动识别导入",
		`id="upstreamImportJson"`,
		`id="upstreamImportFile"`,
		"/admin/upstreams/import",
		"JSON.parse(raw)",
		"已导入 ${data.imported || 0} 个 Codex 上游账号",
		"<th>时效</th>",
		"<th>账号状态</th>",
		"<th>启用</th>",
		"检查中",
		"正在检查账号",
		"无额度数据",
		"未过期",
		"检查失败",
		"认证失败",
		"缺少 chatgptAccountId",
		"switch-slot",
		"grid-template-columns: 1fr",
		"upstreamLimitLines(up)",
		"upstreamValidity(u)",
		"upstreamAvailabilityCell(u)",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("admin upstream UI missing %q", required)
		}
	}
	if !strings.Contains(text, `<label>access_token<input name="accessToken" required></label>`) {
		t.Fatal("admin upstream UI must require access_token before importing a routable account")
	}
	for _, forbidden := range []string{"<th>剩余</th>", "<th>余额</th>", "<th>风控</th>", "上游余额", "风控不可用", "余额不可用", "min-width: 1180px", "flex-wrap: nowrap", "生成 Key</button>", "data-up-balance", "data-up-risk", "data-up-availability", "balanceStatus", "riskStatus", "credentialFingerprint", "accessTokenCipher", "refreshTokenCipher", "prompt(", "alert(", "confirm("} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("admin upstream UI still exposes split availability control %q", forbidden)
		}
	}
}

func TestAdminAPIKeyUIUsesInlineStatusAndHidesSecrets(t *testing.T) {
	raw, err := adminFiles.ReadFile("web/admin/index.html")
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, required := range []string{
		`id="upstreamMsg"`,
		"已生成 API Key 记录",
		"已更新 API Key 状态",
		"routeAvailable",
		"最近调度",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("admin api key UI missing %q", required)
		}
	}
	for _, forbidden := range []string{"alert(", "secretOnce", "keyHash", "publicPrefix", "accessTokenCipher", "refreshTokenCipher"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("admin api key UI exposes forbidden pattern %q", forbidden)
		}
	}
}

func TestAdminTopupUIUsesInlineEditForm(t *testing.T) {
	raw, err := adminFiles.ReadFile("web/admin/index.html")
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, required := range []string{
		"topupEditId",
		"编辑 Token 充值项",
		"保存修改",
		"data-cancel-topup-edit",
		"state.topupEditId = item.id",
		"method: 'PATCH'",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("admin topup inline edit UI missing %q", required)
		}
	}
	for _, forbidden := range []string{"prompt(", "alert(", "confirm("} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("admin topup UI must not use native dialog %q", forbidden)
		}
	}
}

func TestAdminUsageUIIncludesGlobalTokenLedger(t *testing.T) {
	raw, err := adminFiles.ReadFile("web/admin/index.html")
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, required := range []string{
		"ledgerPage",
		"request('/admin/ledger?'",
		"<h2>token 流水</h2>",
		"<th>token 变动</th>",
		"<th>变动后余额</th>",
		"pager(ledger, 'ledger')",
		"bindPager('ledger'",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("admin usage UI missing global token ledger %q", required)
		}
	}
}

func TestClientAdvancedDiagnosticsStaysSanitized(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "desktop-client", "ui", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, required := range []string{
		"<h2>脱敏诊断</h2>",
		"本机 Codex 是否检测到",
		"本机 Codex 是否正在运行",
		"最近一次启动失败原因",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("client diagnostics UI missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"恢复启动前配置",
		"配置是否写入成功",
		"restoreConfigBtn",
		"restore_codex_config",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("client diagnostics UI must not expose Codex config mutation control %q", forbidden)
		}
	}

	start := strings.Index(text, "$('diagnosticsTable').innerHTML = table(['项目', '状态'], [")
	if start < 0 {
		t.Fatal("client diagnostics table rendering block missing")
	}
	end := strings.Index(text[start:], "]);")
	if end < 0 {
		t.Fatal("client diagnostics table rendering block is not closed")
	}
	block := text[start : start+end]
	if rows := strings.Count(block, "        ['"); rows != 3 {
		t.Fatalf("client diagnostics must show exactly three rows, got %d in %s", rows, block)
	}
	for _, forbidden := range []string{"API Key", "proxy", "endpoint", "base_url", "gateway", "Authorization", "凭据", "上游", "provider", "模型选择"} {
		if strings.Contains(block, forbidden) {
			t.Fatalf("client diagnostics exposes internal detail %q in %s", forbidden, block)
		}
	}
}

func TestClientAdvancedModeIncludesSoftwareUpdatePanel(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "desktop-client", "ui", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, required := range []string{
		"<h2>客户端更新</h2>",
		`id="checkUpdateBtn"`,
		`id="openUpdateBtn"`,
		"async function checkUpdate()",
		"/client/desktop/update?currentVersion=",
		"open_update_download",
		"当前版本",
		"最新版本",
		"更新状态",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("client update UI missing %q", required)
		}
	}
	start := strings.Index(text, "function renderUpdateInfo()")
	if start < 0 {
		t.Fatal("client update rendering block missing")
	}
	end := strings.Index(text[start:], "\n    async function openUpdateDownload")
	if end < 0 {
		t.Fatal("client update rendering block boundary missing")
	}
	block := text[start : start+end]
	if strings.Contains(block, "下载地址") {
		t.Fatalf("client update panel should not render raw download url label: %s", block)
	}

	loginStart := strings.Index(text, `<div id="login" class="login grid">`)
	loginEnd := strings.Index(text, `<div id="app" class="shell hidden">`)
	if loginStart < 0 || loginEnd < 0 || loginEnd <= loginStart {
		t.Fatal("client login block boundary missing")
	}
	loginBlock := text[loginStart:loginEnd]
	for _, forbidden := range []string{"客户端更新", "检查更新", "获取更新"} {
		if strings.Contains(loginBlock, forbidden) {
			t.Fatalf("client login block exposes update entry %q in %s", forbidden, loginBlock)
		}
	}
}

func TestClientSimpleModeRefreshesBalanceSilently(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "desktop-client", "ui", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, required := range []string{
		"async function silentRefreshBalance()",
		"async function refreshLocalDiagnostics()",
		"function startBalanceAutoRefresh()",
		"function startDiagnosticsAutoRefresh()",
		"const me = await request('/client/me');",
		"state.balanceRefreshTimer = setInterval(silentRefreshBalance, 30000);",
		"if (state.diagnosticsRefreshPending) return;",
		"state.diagnosticsRefreshPending = true;",
		"state.diagnosticsRefreshPending = false;",
		"state.diagnosticsRefreshTimer = setInterval(refreshLocalDiagnostics, 6000);",
		"window.addEventListener('focus', () => { silentRefreshBalance(); refreshLocalDiagnostics(); });",
		"document.addEventListener('visibilitychange'",
		"startBalanceAutoRefresh();",
		"startDiagnosticsAutoRefresh();",
		"refresh().then(() => { startBalanceAutoRefresh(); startDiagnosticsAutoRefresh(); })",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("client silent balance refresh missing %q", required)
		}
	}
	for _, forbidden := range []string{
		`id="refreshBalanceBtn"`,
		">刷新余额</button>",
		"$('refreshBalanceBtn').onclick",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("client simple mode must not expose manual balance refresh control %q", forbidden)
		}
	}
	start := strings.Index(text, "async function silentRefreshBalance()")
	if start < 0 {
		t.Fatal("silentRefreshBalance function missing")
	}
	end := strings.Index(text[start:], "\n    async function loadTopups")
	if end < 0 {
		t.Fatal("silentRefreshBalance function block boundary missing")
	}
	block := text[start : start+end]
	for _, forbidden := range []string{"充值成功", "已到账", "已确认到账", "alert(", "刷新失败", "topupMsg", "launchMsg", "lastFailure"} {
		if strings.Contains(block, forbidden) {
			t.Fatalf("silent balance refresh should not show foreground popup/text %q in %s", forbidden, block)
		}
	}
}

func TestClientLaunchFailureShowsSanitizedNextAction(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "desktop-client", "ui", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, required := range []string{
		`id="launchMsg"`,
		"const failure = launchFailureMessage(err.message, launchStep);",
		"$('launchMsg').textContent = failure;",
		"$('launchMsg').className = 'msg err';",
		"未检测到本机 Codex，请确认已安装后重试",
		"本机 Codex 主程序启动失败，请确认 Codex 可从开始菜单正常打开",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("client launch failure UI missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"$('launchMsg').textContent = err.message",
		"$('launchMsg').innerHTML = err.message",
		"$('launchMsg').textContent = code",
		"本机 Codex 配置无法读取",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("client launch failure exposes raw error via %q", forbidden)
		}
	}
}

func TestClientLaunchChecksServiceStateBeforePrepare(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "desktop-client", "ui", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, required := range []string{
		"function renderCodexLaunchState()",
		"$('launchBtn').textContent = '停止 Codex';",
		"$('launchBtn').textContent = '正在启动';",
		"$('launchBtn').textContent = '安装 Codex';",
		"$('launchBtn').textContent = '启动 Codex';",
		"$('launchBtn').disabled = true;",
		"$('launchBtn').disabled = false;",
		"async function waitForCodexRunning()",
		"async function stopCodex()",
		"const stopped = await invokeTauri('stop_codex');",
		"async function installCodex()",
		"await invokeTauri('install_codex');",
		"async function launchButtonAction()",
		"if (status === '未检测到') return installCodex();",
		`<div id="installProgress" class="install-progress hidden" aria-live="polite">`,
		`id="installProgressFill"`,
		"function startInstallProgress()",
		"function finishInstallProgress(ok)",
		"if (state.codexLaunchPhase === 'installing') return '正在安装';",
		"state.codexLaunchPhase = 'installing';",
		"$('launchBtn').textContent = '正在安装';",
		"$('spark').classList.add('hidden');",
		"$('spark').classList.remove('hidden');",
		"setInstallProgress(100, 'Codex 已安装');",
		"Codex 已安装",
		"function startDiagnosticsAutoRefresh()",
		"setInterval(refreshLocalDiagnostics, 6000)",
		`<p>Codex 账号 <span id="currentAccount" class="status"></span></p>`,
		"$('currentAccount').textContent = state.diagnostics.codexAccount || '未识别';",
		"if (state.diagnostics.codexRunning === '可用') return;",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("client launch state UI missing %q", required)
		}
	}
	for _, forbidden := range []string{"启动 Codex+++", "请使用 Windows 客户端启动 Codex+++", `id="launchState"`, "$('launchState')", "Codex 运行中", "previousAccount", "providerAccount", "prep.provider && prep.provider.account"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("client launch UI still uses Codex+++ as the launched app via %q", forbidden)
		}
	}
	if strings.Contains(text, "$('currentAccount').textContent = me.user.account") {
		t.Fatal("client Codex account display must not use launcher login account")
	}
	start := strings.Index(text, "async function launchCodex()")
	if start < 0 {
		t.Fatal("launchCodex function missing")
	}
	end := strings.Index(text[start:], "\n    async function waitForCodexRunning")
	if end < 0 {
		t.Fatal("launchCodex function boundary missing")
	}
	block := text[start : start+end]
	meIdx := strings.Index(block, "const me = await request('/client/me');")
	applyIdx := strings.Index(block, "applyClientMe(me);")
	backendPrepareIdx := strings.Index(block, "const prep = await request('/client/launch/prepare'")
	tokenIdx := strings.Index(block, "providerToken = prep.provider && prep.provider.bearerToken;")
	localPrepareIdx := strings.Index(block, "const localPrep = await invokeTauri('prepare_codex', { backendUrl: state.api, providerToken });")
	if meIdx < 0 || applyIdx < 0 || backendPrepareIdx < 0 || tokenIdx < 0 || localPrepareIdx < 0 {
		t.Fatalf("launchCodex must refresh client state before local prepare: %s", block)
	}
	if !(meIdx < applyIdx && applyIdx < backendPrepareIdx && backendPrepareIdx < tokenIdx && tokenIdx < localPrepareIdx) {
		t.Fatalf("launchCodex check order invalid me=%d apply=%d backendPrepare=%d token=%d localPrepare=%d block=%s", meIdx, applyIdx, backendPrepareIdx, tokenIdx, localPrepareIdx, block)
	}
	for _, required := range []string{"state.diagnostics.codexAuthMode !== 'chatgpt'", "if (!providerToken) throw new Error('codex_provider_unavailable');"} {
		if !strings.Contains(block, required) {
			t.Fatalf("launchCodex missing API key login guard %q in %s", required, block)
		}
	}
	for _, forbidden := range []string{"余额不足", "token_not_available", "申请充值", "联系管理员"} {
		if strings.Contains(block, forbidden) {
			t.Fatalf("launchCodex must not show token-shortage prompt/action %q in %s", forbidden, block)
		}
	}
}

func TestClientLoginPageStaysMinimalAndActionable(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "desktop-client", "ui", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, required := range []string{
		`<div id="login" class="login grid">`,
		`<label>账号<input id="account" autocomplete="username"></label>`,
		`<label>密码<input id="password" type="password" autocomplete="current-password"></label>`,
		`<button id="loginBtn" class="primary">登录</button>`,
		"$('loginMsg').textContent = userMessage(err.message, '登录失败，请重新输入后再试');",
		"desktop_client_required: '请使用 Windows 客户端登录'",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("client login UI missing %q", required)
		}
	}
	start := strings.Index(text, `<div id="login" class="login grid">`)
	end := strings.Index(text, `<div id="app" class="shell hidden">`)
	if start < 0 || end < 0 || end <= start {
		t.Fatal("client login block boundary missing")
	}
	loginBlock := text[start:end]
	for _, forbidden := range []string{"高级模式", "注册", "忘记密码", "找回密码", "API Key", "proxy", "endpoint", "base_url", "诊断", "安装维护"} {
		if strings.Contains(loginBlock, forbidden) {
			t.Fatalf("client login block exposes forbidden entry %q in %s", forbidden, loginBlock)
		}
	}
}

func TestClientLoginRequiresDesktopShellDeviceIdentity(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "desktop-client", "ui", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	start := strings.Index(text, "async function deviceIdentity()")
	if start < 0 {
		t.Fatal("deviceIdentity function missing")
	}
	end := strings.Index(text[start:], "\n    async function login")
	if end < 0 {
		t.Fatal("deviceIdentity function boundary missing")
	}
	block := text[start : start+end]
	for _, required := range []string{
		"const native = await invokeTauri('device_identity');",
		"if (native && native.fingerprint) return native;",
		"throw new Error('desktop_client_required');",
	} {
		if !strings.Contains(block, required) {
			t.Fatalf("client device identity must require desktop shell %q in %s", required, block)
		}
	}
	for _, forbidden := range []string{"deviceFingerprint", "crypto.randomUUID", "navigator.userAgent", "localStorage.setItem('deviceFingerprint'"} {
		if strings.Contains(block, forbidden) {
			t.Fatalf("client device identity still has browser fallback %q in %s", forbidden, block)
		}
	}
}

func TestClientUserFacingErrorsStayChineseAndMapped(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "desktop-client", "ui", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, required := range []string{
		"topup_unavailable: 'Token 充值项不可用，请刷新后重试'",
		"free_topup_view_only: 'free Token 充值项仅供查看，请选择其他 Token 充值项'",
		"invalid_recharge_request: '申请失败，请刷新后重试'",
		"invalid_client_login_request: '登录失败'",
		"route_unavailable: '服务暂时不可用，请稍后再试'",
		"network_unavailable: '网络不可用，请稍后重试'",
		"desktop_client_required: '请使用 Windows 客户端登录'",
		"codex_stop_failed: 'Codex 停止失败，请稍后重试'",
		"codex_install_unavailable: 'Codex 安装失败，请联系管理员'",
		"codex_install_component_missing: 'Codex 安装失败，请联系管理员'",
		"codex_install_failed: 'Codex 安装失败，请联系管理员'",
		"登录失败，请重新输入后再试",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("client error mapping missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"$('loginMsg').textContent = err.message",
		"$('topupMsg').textContent = err.message",
		"$('launchState').textContent = err.message",
		"$('startState').textContent = err.message",
		"$('launchMsg').textContent = err.message",
		"token_not_available: '暂时无法继续启动'",
		"暂时无法继续启动",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("client UI directly exposes error message via %q", forbidden)
		}
	}
}

func TestAdminRechargeQueueUIOnlyOffersSingleApproveReject(t *testing.T) {
	raw, err := adminFiles.ReadFile("web/admin/index.html")
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	start := strings.Index(text, "async function renderRecharges()")
	if start < 0 {
		t.Fatal("admin pending recharge renderer missing")
	}
	end := strings.Index(text[start:], "\n    async function renderUpstreams")
	if end < 0 {
		t.Fatal("admin pending recharge renderer boundary missing")
	}
	block := text[start : start+end]
	for _, required := range []string{
		"status: 'pending'",
		"<th>用户账号</th>",
		"<th>Token 充值项</th>",
		"<th>充值项 token</th>",
		"<th>提交时间</th>",
		"<th>状态</th>",
		"<th>操作</th>",
		"data-approve",
		"data-reject",
		"/approve",
		"/reject",
	} {
		if !strings.Contains(block, required) {
			t.Fatalf("admin pending recharge UI missing %q in %s", required, block)
		}
	}
	for _, forbidden := range []string{"批量", "全选", "撤销", "取消", "token-adjustments", "deltaTokens", "remark", "data-cancel", "data-batch"} {
		if strings.Contains(block, forbidden) {
			t.Fatalf("admin pending recharge UI exposes forbidden operation %q in %s", forbidden, block)
		}
	}
}

func TestAdminDevicesPaginate(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("devices")
	if code := c.request(http.MethodPost, "/api/client/login", "", map[string]any{"account": "devices", "password": "", "deviceName": "Windows 设备", "fingerprint": ""}, nil); code != http.StatusUnauthorized {
		t.Fatalf("empty fingerprint login status = %d", code)
	}
	var emptyDevices struct {
		Total int `json:"total"`
	}
	if code := c.request(http.MethodGet, "/api/admin/devices", c.adminToken, nil, &emptyDevices); code != http.StatusOK {
		t.Fatalf("devices after empty fingerprint status = %d", code)
	}
	if emptyDevices.Total != 0 {
		t.Fatalf("empty fingerprint created device: %#v", emptyDevices)
	}
	var firstDeviceToken string
	for i := 0; i < 12; i++ {
		var login struct {
			Token string `json:"token"`
		}
		body := map[string]any{"account": "devices", "password": "", "deviceName": fmt.Sprintf("Windows 设备 %02d", i), "fingerprint": fmt.Sprintf("device-%02d", i)}
		if code := c.request(http.MethodPost, "/api/client/login", "", body, &login); code != http.StatusOK {
			t.Fatalf("client login %d status = %d", i, code)
		}
		if i == 0 {
			firstDeviceToken = login.Token
		}
	}
	var devices struct {
		Items []map[string]any `json:"items"`
		Page  int              `json:"page"`
		Total int              `json:"total"`
	}
	if code := c.request(http.MethodGet, "/api/admin/devices?page=2&size=10", c.adminToken, nil, &devices); code != http.StatusOK {
		t.Fatalf("devices status = %d", code)
	}
	if devices.Page != 2 || devices.Total != 12 || len(devices.Items) != 2 {
		t.Fatalf("devices pagination = %#v", devices)
	}
	if _, ok := devices.Items[0]["fingerprint"]; ok {
		t.Fatalf("device fingerprint leaked in list: %#v", devices.Items[0])
	}
	var firstPage struct {
		Items []map[string]any `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/admin/devices?page=1&size=10", c.adminToken, nil, &firstPage); code != http.StatusOK {
		t.Fatalf("devices first page status = %d", code)
	}
	var firstID string
	for _, item := range firstPage.Items {
		if item["name"] == "Windows 设备 00" {
			firstID, _ = item["id"].(string)
			break
		}
	}
	if firstID == "" {
		t.Fatalf("first device missing: %#v", firstPage.Items)
	}
	var updated map[string]any
	if code := c.request(http.MethodPost, "/api/admin/devices/"+firstID+"/status", c.adminToken, map[string]any{"status": "disabled"}, &updated); code != http.StatusOK {
		t.Fatalf("disable device status = %d", code)
	}
	if _, ok := updated["fingerprint"]; ok {
		t.Fatalf("device fingerprint leaked in status response: %#v", updated)
	}
	var sessionErr apiError
	if code := c.request(http.MethodGet, "/api/client/me", firstDeviceToken, nil, &sessionErr); code != http.StatusUnauthorized {
		t.Fatalf("disabled device existing session status = %d", code)
	}
	if sessionErr.Error != "login_failed" {
		t.Fatalf("disabled device existing session error = %#v", sessionErr)
	}
	loginBody := map[string]any{"account": "devices", "password": "", "deviceName": "Windows 设备 00", "fingerprint": "device-00"}
	var loginErr apiError
	if code := c.request(http.MethodPost, "/api/client/login", "", loginBody, &loginErr); code != http.StatusUnauthorized {
		t.Fatalf("disabled device login status = %d", code)
	}
	if loginErr.Error != "login_failed" {
		t.Fatalf("disabled device login error = %#v", loginErr)
	}
	changedFingerprint := map[string]any{"account": "devices", "password": "", "deviceId": firstID, "deviceName": "Windows 设备 00", "fingerprint": "device-00-changed"}
	var changedErr apiError
	if code := c.request(http.MethodPost, "/api/client/login", "", changedFingerprint, &changedErr); code != http.StatusUnauthorized {
		t.Fatalf("disabled device id with changed fingerprint login status = %d", code)
	}
	if changedErr.Error != "login_failed" {
		t.Fatalf("disabled device id with changed fingerprint error = %#v", changedErr)
	}
	actions := c.auditActionCounts()
	if actions["device.status"] == 0 {
		t.Fatalf("missing device.status audit in %#v", actions)
	}
}

func TestParseCodexProbeResult(t *testing.T) {
	result := parseCodexProbeResult(
		map[string]any{"account": map[string]any{"type": "chatgpt", "email": "codex@example.com", "planType": "plus"}},
		map[string]any{"rateLimitsByLimitId": map[string]any{"codex": map[string]any{
			"rateLimitReachedType": "usage_limit",
			"primary":              map[string]any{"usedPercent": float64(64), "resetsAt": float64(1783213200)},
			"credits":              map[string]any{"remaining": float64(8.5), "formatted": "8.5"},
		}}},
		map[string]any{"dailyUsageBuckets": []any{map[string]any{"startDate": "2026-07-01", "tokens": float64(100)}, map[string]any{"startDate": "2026-07-02", "tokens": float64(250)}}},
	)
	if result.AccountType != "chatgpt" || result.Email != "codex@example.com" || result.PlanType != "plus" {
		t.Fatalf("account result = %#v", result)
	}
	if result.RateLimitReachedType != "usage_limit" || result.UsageTokens != 250 {
		t.Fatalf("probe result = %#v", result)
	}
	if result.RateLimitUsedPercent == nil || *result.RateLimitUsedPercent != 64 || result.RateLimitResetsAt == nil || !result.RateLimitResetsAt.Equal(time.Unix(1783213200, 0).UTC()) {
		t.Fatalf("rate limit result = %#v", result)
	}
	if result.CreditBalance == nil || *result.CreditBalance != 8.5 || result.CreditBalanceLabel != "8.5" {
		t.Fatalf("credit balance result = %#v", result)
	}
}

func TestObserveCodexTurnMessageUsesV2LastTokenUsage(t *testing.T) {
	state := &codexTurnState{}
	observeCodexTurnMessage(map[string]any{
		"method": "thread/tokenUsage/updated",
		"params": map[string]any{
			"tokenUsage": map[string]any{
				"last": map[string]any{
					"inputTokens":           11,
					"cachedInputTokens":     2,
					"outputTokens":          7,
					"reasoningOutputTokens": 3,
					"totalTokens":           20,
				},
				"total": map[string]any{
					"inputTokens":           111,
					"cachedInputTokens":     22,
					"outputTokens":          77,
					"reasoningOutputTokens": 33,
					"totalTokens":           200,
				},
			},
		},
	}, state)
	if state.Usage.InputTokens != 11 || state.Usage.CachedInputTokens != 2 || state.Usage.OutputTokens != 7 || state.Usage.TotalTokens != 20 {
		t.Fatalf("usage = %#v", state.Usage)
	}
}

func TestObserveCodexTurnMessageCapturesV2RawResponseText(t *testing.T) {
	state := &codexTurnState{}
	observeCodexTurnMessage(map[string]any{
		"method": "rawResponseItem/completed",
		"params": map[string]any{
			"item": map[string]any{
				"type": "message",
				"content": []any{
					map[string]any{"type": "output_text", "text": "hello from app-server"},
				},
			},
		},
	}, state)
	if got := state.Text.String(); got != "hello from app-server" {
		t.Fatalf("raw response text = %q", got)
	}
}

func TestObserveCodexTurnMessageIgnoresCompletedInputItems(t *testing.T) {
	state := &codexTurnState{}
	observeCodexTurnMessage(map[string]any{
		"method": "item/completed",
		"params": map[string]any{
			"item": map[string]any{
				"type": "message",
				"role": "user",
				"content": []any{
					map[string]any{"type": "input_text", "text": "<permissions instructions>raw prompt</permissions instructions>"},
				},
			},
		},
	}, state)
	if got := state.Text.String(); got != "" {
		t.Fatalf("input item leaked into response text: %q", got)
	}
}

func TestCodexAppServerErrorMapsV2CodexErrorInfo(t *testing.T) {
	tests := []struct {
		name string
		err  any
		want string
	}{
		{
			name: "usage limit",
			err:  map[string]any{"message": "limit", "codexErrorInfo": "usageLimitExceeded"},
			want: "codex_app_server_http_402",
		},
		{
			name: "nested http status",
			err: map[string]any{
				"data": map[string]any{
					"codexErrorInfo": map[string]any{
						"responseStreamConnectionFailed": map[string]any{"httpStatusCode": 401},
					},
				},
			},
			want: "codex_app_server_http_401",
		},
		{
			name: "request side error",
			err:  map[string]any{"message": "too large", "codexErrorInfo": "contextWindowExceeded"},
			want: "codex_app_server_http_400",
		},
		{
			name: "openai insufficient quota",
			err: map[string]any{
				"error": map[string]any{
					"type":    "insufficient_quota",
					"code":    "insufficient_quota",
					"message": "You exceeded your current quota, please check your plan and billing details.",
				},
			},
			want: "codex_app_server_http_402",
		},
		{
			name: "sub2api quota exhausted status",
			err:  map[string]any{"data": []any{map[string]any{"status": "quota_exhausted"}}},
			want: "codex_app_server_http_402",
		},
		{
			name: "rate limit reached message",
			err:  map[string]any{"message": "API rate limit reached. Please try again later."},
			want: "codex_app_server_http_429",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := codexAppServerError(tt.err).Error(); got != tt.want {
				t.Fatalf("error = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCodexTurnCompletionErrorPrefersV2TurnErrorInfo(t *testing.T) {
	err := codexTurnCompletionError(map[string]any{
		"method": "turn/completed",
		"params": map[string]any{
			"turn": map[string]any{
				"status": "failed",
				"error": map[string]any{
					"message":        "limit",
					"codexErrorInfo": "usageLimitExceeded",
				},
			},
		},
	})
	if err == nil || err.Error() != "codex_app_server_http_402" {
		t.Fatalf("turn completion error = %v", err)
	}

	err = codexTurnCompletionError(map[string]any{
		"method": "turn/completed",
		"params": map[string]any{
			"turn": map[string]any{"status": "failed"},
		},
	})
	if err == nil || err.Error() != "codex_app_server_turn_failed" {
		t.Fatalf("turn completion fallback error = %v", err)
	}
}

func TestMigrationAllowsClientAuditRole(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("migrations", "001_init.sql"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.Contains(text, "actor_role IN ('admin', 'client', 'system')") {
		t.Fatalf("migration must allow client audit records")
	}
	if !strings.Contains(text, "role IN ('admin', 'client', 'codex')") {
		t.Fatalf("migration must allow codex provider sessions")
	}
	if !strings.Contains(text, "type IN ('recharge', 'adjustment', 'debit')") {
		t.Fatalf("migration must keep token ledger types clean")
	}
	if strings.Contains(text, "'release', 'refund', 'correction'") {
		t.Fatalf("migration contains unused token ledger types")
	}
}

func TestGatewayUsesCodexAppServer(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("cora")
	c.loginClient("cora")
	c.approvePaidRecharge()

	originalRun := codexAppServerRun
	codexAppServerRun = func(ctx context.Context, credentials codexProbeCredentials, requestBody []byte) (codexRunResult, error) {
		if credentials.AccessToken != "access" || credentials.ChatGPTAccountID != "account-123" || credentials.ChatGPTPlanType != "pro" {
			t.Fatalf("run credentials = %#v", credentials)
		}
		var payload map[string]any
		if err := json.NewDecoder(bytes.NewReader(requestBody)).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if _, ok := payload["usage"]; ok {
			t.Fatalf("client usage was forwarded to app-server: %#v", payload)
		}
		for _, key := range []string{"route", "routeId", "apiKey", "apiKeyId", "upstream", "upstreamId", "gatewayPath", "proxy", "endpoint", "base_url", "baseUrl"} {
			if _, ok := payload[key]; ok {
				t.Fatalf("old route field %q was forwarded to app-server: %#v", key, payload)
			}
		}
		return codexRunResult{
			Model: "gpt-5.5",
			Text:  "done",
			Usage: gatewayUsage{InputTokens: 8, CachedInputTokens: 3, OutputTokens: 13, TotalTokens: 21},
		}, nil
	}
	t.Cleanup(func() { codexAppServerRun = originalRun })
	c.createRoutedUpstream("")

	var resp struct {
		UsageRecord   UsageRecord    `json:"usageRecord"`
		ChargedTokens int64          `json:"chargedTokens"`
		Result        map[string]any `json:"result"`
	}
	status := c.gatewayRun(c.clientToken, map[string]any{
		"model":       "gpt-5.5",
		"input":       "run through app-server",
		"usage":       map[string]any{"total_tokens": 999999},
		"route":       "old-route",
		"apiKey":      "old-key",
		"upstream":    "old-upstream",
		"gatewayPath": "/v1/chat/completions",
		"base_url":    "http://legacy.example",
	}, &resp)
	if status != http.StatusOK {
		t.Fatalf("gateway status = %d", status)
	}
	if resp.UsageRecord.TotalTokens != 21 || resp.UsageRecord.CachedInputTokens != 3 || resp.ChargedTokens != 21 {
		t.Fatalf("gateway response = %#v", resp)
	}
	if resp.Result["text"] != "done" {
		t.Fatalf("result = %#v", resp.Result)
	}
	var me struct {
		User map[string]any `json:"user"`
	}
	if code := c.request(http.MethodGet, "/api/client/me", c.clientToken, nil, &me); code != http.StatusOK {
		t.Fatalf("me status = %d", code)
	}
	if got := int64(me.User["tokenBalance"].(float64)); got != 99979 {
		t.Fatalf("token balance = %d", got)
	}
}

func TestGatewayChargesOnlyUpstreamUsage(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("carol")
	c.loginClient("carol")
	c.approvePaidRecharge()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer access" {
			t.Fatalf("authorization = %q", got)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"usage", "inputTokens", "outputTokens", "totalTokens", "route", "routeId", "apiKey", "apiKeyId", "upstream", "upstreamId", "gatewayPath", "proxy", "endpoint", "base_url", "baseUrl"} {
			if _, ok := payload[key]; ok {
				t.Fatalf("client-controlled field %q was forwarded: %#v", key, payload)
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"model": "gpt-5.5",
			"usage": map[string]any{
				"input_tokens":         5,
				"input_tokens_details": map[string]any{"cached_tokens": 2},
				"output_tokens":        7,
				"total_tokens":         12,
			},
			"result": map[string]any{"ok": true},
		})
	}))
	defer upstream.Close()
	c.createRoutedUpstream(upstream.URL)

	var resp struct {
		UsageRecord    map[string]any `json:"usageRecord"`
		UpstreamStatus any            `json:"upstreamStatus"`
	}
	status := c.gatewayRun(c.clientToken, map[string]any{
		"model":        "gpt-5.5",
		"input":        "hello",
		"inputTokens":  999999,
		"outputTokens": 999999,
		"usage":        map[string]any{"total_tokens": 999999},
		"route":        "old-route",
		"apiKey":       "old-key",
		"upstream":     "old-upstream",
		"gatewayPath":  "/v1/messages",
		"proxy":        "http://legacy-proxy",
		"endpoint":     "http://legacy-endpoint",
		"baseUrl":      "http://legacy-base",
	}, &resp)
	if status != http.StatusOK {
		t.Fatalf("gateway status = %d", status)
	}
	if intField(resp.UsageRecord, "totalTokens") != 12 || intField(resp.UsageRecord, "inputTokens") != 5 || intField(resp.UsageRecord, "cachedInputTokens") != 2 || intField(resp.UsageRecord, "outputTokens") != 7 {
		t.Fatalf("usage record = %#v", resp.UsageRecord)
	}
	for _, key := range []string{"id", "userId"} {
		if _, ok := resp.UsageRecord[key]; ok {
			t.Fatalf("gateway usage record leaked %q: %#v", key, resp.UsageRecord)
		}
	}
	if resp.UpstreamStatus != nil {
		t.Fatalf("gateway success leaked upstream status: %#v", resp)
	}
	var me struct {
		User map[string]any `json:"user"`
	}
	if code := c.request(http.MethodGet, "/api/client/me", c.clientToken, nil, &me); code != http.StatusOK {
		t.Fatalf("me status = %d", code)
	}
	if got := int64(me.User["tokenBalance"].(float64)); got != 99988 {
		t.Fatalf("token balance = %d", got)
	}
}

func TestGatewayRedisRateLimitBlocksBeforeUpstreamAndBilling(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("riley")
	c.loginClient("riley")
	c.approvePaidRecharge()

	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		writeJSON(w, http.StatusOK, map[string]any{
			"model": "gpt-5.5",
			"usage": map[string]any{
				"input_tokens":  1,
				"output_tokens": 2,
				"total_tokens":  3,
			},
		})
	}))
	defer upstream.Close()
	c.createRoutedUpstream(upstream.URL)

	limiter := &fakeGatewayRateLimiter{allow: false}
	c.app.gatewayLimiter = limiter
	c.app.gatewayRateLimit = 1
	c.app.gatewayRateWindow = time.Minute

	var failure apiError
	if code := c.gatewayRun(c.clientToken, map[string]any{"model": "gpt-5.5", "input": "blocked"}, &failure); code != http.StatusTooManyRequests {
		t.Fatalf("gateway status = %d error=%#v", code, failure)
	}
	if failure.Error != "rate_limited" {
		t.Fatalf("gateway error = %#v", failure)
	}
	if limiter.calls != 1 || upstreamCalls != 0 {
		t.Fatalf("limiter calls=%d upstream calls=%d", limiter.calls, upstreamCalls)
	}
	var me struct {
		User map[string]any `json:"user"`
	}
	if code := c.request(http.MethodGet, "/api/client/me", c.clientToken, nil, &me); code != http.StatusOK {
		t.Fatalf("me status = %d", code)
	}
	if got := int64(me.User["tokenBalance"].(float64)); got != 100000 {
		t.Fatalf("token balance = %d", got)
	}
	var usage struct {
		Items []UsageRecord `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/client/usage", c.clientToken, nil, &usage); code != http.StatusOK {
		t.Fatalf("usage status = %d", code)
	}
	if len(usage.Items) != 0 {
		t.Fatalf("usage records = %#v", usage.Items)
	}
}

func TestGatewayCompletedIdempotencyBypassesRedisRateLimit(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("ivy")
	c.loginClient("ivy")
	c.approvePaidRecharge()

	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		writeJSON(w, http.StatusOK, map[string]any{
			"model": "gpt-5.5",
			"usage": map[string]any{
				"input_tokens":  2,
				"output_tokens": 3,
				"total_tokens":  5,
			},
		})
	}))
	defer upstream.Close()
	c.createRoutedUpstream(upstream.URL)

	limiter := &fakeGatewayRateLimiter{allow: true}
	c.app.gatewayLimiter = limiter
	c.app.gatewayRateLimit = 1
	c.app.gatewayRateWindow = time.Minute

	headers := map[string]string{"Idempotency-Key": "idem-redis-bypass"}
	var first, second struct {
		Idempotent bool `json:"idempotent"`
	}
	if code := c.gatewayRunWithHeaders(c.clientToken, headers, map[string]any{"model": "gpt-5.5", "input": "first"}, &first); code != http.StatusOK {
		t.Fatalf("first gateway status = %d", code)
	}
	limiter.allow = false
	if code := c.gatewayRunWithHeaders(c.clientToken, headers, map[string]any{"model": "gpt-5.5", "input": "again"}, &second); code != http.StatusOK {
		t.Fatalf("second gateway status = %d", code)
	}
	if upstreamCalls != 1 || limiter.calls != 1 || !second.Idempotent {
		t.Fatalf("upstream=%d limiter=%d first=%#v second=%#v", upstreamCalls, limiter.calls, first, second)
	}
}

func TestGatewayDoesNotChargeWhenUpstreamUsageMissing(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("dana")
	c.loginClient("dana")
	c.approvePaidRecharge()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"result": map[string]any{"ok": true}})
	}))
	defer upstream.Close()
	c.createRoutedUpstream(upstream.URL)

	var failure map[string]any
	if status := c.gatewayRun(c.clientToken, map[string]any{"model": "gpt-5.5", "input": "hello"}, &failure); status != http.StatusBadGateway {
		t.Fatalf("gateway status = %d", status)
	}
	if failure["error"] != "codex_app_server_usage_missing" {
		t.Fatalf("gateway failure = %#v", failure)
	}
	if _, ok := failure["upstreamStatus"]; ok {
		t.Fatalf("gateway failure leaked upstream status: %#v", failure)
	}
	var me struct {
		User map[string]any `json:"user"`
	}
	if code := c.request(http.MethodGet, "/api/client/me", c.clientToken, nil, &me); code != http.StatusOK {
		t.Fatalf("me status = %d", code)
	}
	if got := int64(me.User["tokenBalance"].(float64)); got != 100000 {
		t.Fatalf("token balance = %d", got)
	}
	var usage struct {
		Items []UsageRecord `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/client/usage", c.clientToken, nil, &usage); code != http.StatusOK {
		t.Fatalf("usage status = %d", code)
	}
	if len(usage.Items) != 0 {
		t.Fatalf("usage records = %#v", usage.Items)
	}
}

func TestGatewayDoesNotSwitchWhenUsageIsUnverifiable(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("unverifiable")
	c.loginClient("unverifiable")
	c.approvePaidRecharge()

	firstCalls := 0
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		firstCalls++
		writeJSON(w, http.StatusOK, map[string]any{"result": map[string]any{"ok": true}})
	}))
	defer first.Close()
	c.createRoutedUpstream(first.URL)

	secondCalls := 0
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondCalls++
		writeJSON(w, http.StatusOK, map[string]any{
			"model": "gpt-5.5",
			"usage": map[string]any{
				"input_tokens":  1,
				"output_tokens": 2,
				"total_tokens":  3,
			},
		})
	}))
	defer second.Close()
	c.createRoutedUpstream(second.URL)

	var failure map[string]any
	if status := c.gatewayRunWithHeaders(c.clientToken, map[string]string{"Idempotency-Key": "unverifiable-usage"}, map[string]any{"model": "gpt-5.5", "input": "hello"}, &failure); status != http.StatusBadGateway {
		t.Fatalf("gateway status = %d failure=%#v", status, failure)
	}
	if failure["error"] != "codex_app_server_usage_missing" {
		t.Fatalf("gateway failure = %#v", failure)
	}
	if firstCalls != 1 || secondCalls != 0 {
		t.Fatalf("unverifiable usage must fail closed without switching, calls first=%d second=%d", firstCalls, secondCalls)
	}
	var me struct {
		User map[string]any `json:"user"`
	}
	if code := c.request(http.MethodGet, "/api/client/me", c.clientToken, nil, &me); code != http.StatusOK {
		t.Fatalf("me status = %d", code)
	}
	if got := int64(me.User["tokenBalance"].(float64)); got != 100000 {
		t.Fatalf("token balance = %d", got)
	}
	actions := c.auditActionCounts()
	if actions["gateway.route.switch"] != 0 {
		t.Fatalf("unverifiable usage should not switch route: %#v", actions)
	}
	if actions["gateway.request.release"] == 0 {
		t.Fatalf("missing release audit after unverifiable usage: %#v", actions)
	}
}

func TestGatewayRejectedUpstreamRequestDoesNotLeakRawBodyOrDisableRoute(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("rejected")
	c.loginClient("rejected")
	c.approvePaidRecharge()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "raw upstream secret C:\\Users\\1\\.codex\\config.toml token=abc access_token=secret",
		})
	}))
	defer upstream.Close()
	upstreamID := c.createRoutedUpstream(upstream.URL)

	var failure apiError
	if status := c.gatewayRun(c.clientToken, map[string]any{"model": "gpt-5.5", "input": "bad shape"}, &failure); status != http.StatusBadGateway {
		t.Fatalf("gateway status = %d failure=%#v", status, failure)
	}
	if failure.Error != "upstream_rejected_request" {
		t.Fatalf("gateway failure = %#v", failure)
	}
	encodedFailure, _ := json.Marshal(failure)
	for _, forbidden := range []string{"raw upstream", "C:\\Users", "token=abc", "access_token", "secret"} {
		if strings.Contains(string(encodedFailure), forbidden) {
			t.Fatalf("gateway failure leaked %q: %s", forbidden, encodedFailure)
		}
	}

	var me struct {
		User map[string]any `json:"user"`
	}
	if code := c.request(http.MethodGet, "/api/client/me", c.clientToken, nil, &me); code != http.StatusOK {
		t.Fatalf("me status = %d", code)
	}
	if got := int64(me.User["tokenBalance"].(float64)); got != 100000 {
		t.Fatalf("token balance = %d", got)
	}
	var upstreams struct {
		Items []struct {
			ID                 string `json:"id"`
			AvailabilityStatus string `json:"availabilityStatus"`
		} `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/admin/upstreams", c.adminToken, nil, &upstreams); code != http.StatusOK {
		t.Fatalf("upstreams status = %d", code)
	}
	for _, item := range upstreams.Items {
		if item.ID == upstreamID && item.AvailabilityStatus != "available" {
			t.Fatalf("rejected request disabled route: %#v", item)
		}
	}
}

func TestGatewaySwitchesToNextRouteWhenUpstreamBalanceUnavailable(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("gail")
	c.loginClient("gail")
	c.approvePaidRecharge()

	firstCalls := 0
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		firstCalls++
		writeJSON(w, http.StatusPaymentRequired, map[string]any{"error": "insufficient_quota"})
	}))
	defer first.Close()
	firstID := c.createRoutedUpstream(first.URL)

	secondCalls := 0
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondCalls++
		writeJSON(w, http.StatusOK, map[string]any{
			"model": "gpt-5.5",
			"usage": map[string]any{
				"input_tokens":  9,
				"output_tokens": 4,
				"total_tokens":  13,
			},
		})
	}))
	defer second.Close()
	secondID := c.createRoutedUpstream(second.URL)

	var resp struct {
		UsageRecord   UsageRecord `json:"usageRecord"`
		ChargedTokens int64       `json:"chargedTokens"`
	}
	if code := c.gatewayRun(c.clientToken, map[string]any{"model": "gpt-5.5", "input": "switch"}, &resp); code != http.StatusOK {
		t.Fatalf("gateway status = %d", code)
	}
	if firstCalls != 1 || secondCalls != 1 {
		t.Fatalf("calls first=%d second=%d", firstCalls, secondCalls)
	}
	if resp.UsageRecord.TotalTokens != 13 || resp.ChargedTokens != 13 {
		t.Fatalf("gateway response = %#v", resp)
	}
	var upstreams struct {
		Items []struct {
			ID                 string `json:"id"`
			AvailabilityStatus string `json:"availabilityStatus"`
		} `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/admin/upstreams", c.adminToken, nil, &upstreams); code != http.StatusOK {
		t.Fatalf("upstreams status = %d", code)
	}
	var firstAvailability string
	for _, item := range upstreams.Items {
		if item.ID == firstID {
			firstAvailability = item.AvailabilityStatus
		}
	}
	if firstAvailability != "unavailable" {
		t.Fatalf("first upstream availability=%q", firstAvailability)
	}
	c.store.mu.Lock()
	firstUpstream := c.app.upstreamByID(firstID)
	c.store.mu.Unlock()
	if firstUpstream.BalanceStatus != "unavailable" || firstUpstream.RiskStatus != "available" {
		t.Fatalf("first internal upstream status balance=%q risk=%q", firstUpstream.BalanceStatus, firstUpstream.RiskStatus)
	}
	var audit struct {
		Items []AuditLog `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/admin/audit?size=100", c.adminToken, nil, &audit); code != http.StatusOK {
		t.Fatalf("audit status = %d", code)
	}
	switchAuditFound := false
	for _, item := range audit.Items {
		if item.Action != "gateway.route.switch" {
			continue
		}
		switchAuditFound = true
		if item.ActorRole != "system" || item.TargetID != secondID || !strings.Contains(item.Detail, "reason=upstream_balance_unavailable") {
			t.Fatalf("switch audit = %#v", item)
		}
		if strings.Contains(item.Detail, "insufficient_quota") || strings.Contains(item.Detail, "access") {
			t.Fatalf("switch audit leaked raw details: %#v", item)
		}
	}
	if !switchAuditFound {
		t.Fatalf("switch audit not found: %#v", audit.Items)
	}
	var me struct {
		User map[string]any `json:"user"`
	}
	if code := c.request(http.MethodGet, "/api/client/me", c.clientToken, nil, &me); code != http.StatusOK {
		t.Fatalf("me status = %d", code)
	}
	if got := int64(me.User["tokenBalance"].(float64)); got != 99987 {
		t.Fatalf("token balance = %d", got)
	}
}

func TestGatewaySwitchesOnV2CodexUsageLimitError(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("v2-limit")
	c.loginClient("v2-limit")
	c.approvePaidRecharge()

	firstID := c.createRoutedUpstream("v2-first")
	secondID := c.createRoutedUpstream("v2-second")

	firstToken := "test-upstream-url:v2-first"
	secondToken := "test-upstream-url:v2-second"
	calls := map[string]int{}
	originalRun := codexAppServerRun
	codexAppServerRun = func(ctx context.Context, credentials codexProbeCredentials, requestBody []byte) (codexRunResult, error) {
		calls[credentials.AccessToken]++
		switch credentials.AccessToken {
		case firstToken:
			return codexRunResult{}, codexAppServerError(map[string]any{
				"message":        "limit",
				"codexErrorInfo": "usageLimitExceeded",
			})
		case secondToken:
			return codexRunResult{
				Model: "gpt-5.5",
				Text:  "ok",
				Usage: gatewayUsage{
					InputTokens:  5,
					OutputTokens: 3,
					TotalTokens:  8,
				},
				Status: http.StatusOK,
			}, nil
		default:
			return codexRunResult{}, fmt.Errorf("codex_app_server_start_failed")
		}
	}
	t.Cleanup(func() { codexAppServerRun = originalRun })

	var resp struct {
		UsageRecord   UsageRecord `json:"usageRecord"`
		ChargedTokens int64       `json:"chargedTokens"`
	}
	if code := c.gatewayRun(c.clientToken, map[string]any{"model": "gpt-5.5", "input": "switch v2"}, &resp); code != http.StatusOK {
		t.Fatalf("gateway status = %d", code)
	}
	if calls[firstToken] != 1 || calls[secondToken] != 1 {
		t.Fatalf("calls = %#v", calls)
	}
	if resp.UsageRecord.TotalTokens != 8 || resp.ChargedTokens != 8 {
		t.Fatalf("gateway response = %#v", resp)
	}

	var upstreams struct {
		Items []struct {
			ID                 string `json:"id"`
			AvailabilityStatus string `json:"availabilityStatus"`
		} `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/admin/upstreams", c.adminToken, nil, &upstreams); code != http.StatusOK {
		t.Fatalf("upstreams status = %d", code)
	}
	statuses := map[string]string{}
	for _, item := range upstreams.Items {
		statuses[item.ID] = item.AvailabilityStatus
	}
	if statuses[firstID] != "unavailable" || statuses[secondID] != "available" {
		t.Fatalf("upstream statuses = %#v", statuses)
	}

	var audit struct {
		Items []AuditLog `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/admin/audit?size=100", c.adminToken, nil, &audit); code != http.StatusOK {
		t.Fatalf("audit status = %d", code)
	}
	switchAuditFound := false
	for _, item := range audit.Items {
		if item.Action != "gateway.route.switch" {
			continue
		}
		switchAuditFound = true
		if item.ActorRole != "system" || item.TargetID != secondID || !strings.Contains(item.Detail, "reason=upstream_balance_unavailable") {
			t.Fatalf("switch audit = %#v", item)
		}
		if strings.Contains(item.Detail, "limit") || strings.Contains(item.Detail, "usageLimitExceeded") {
			t.Fatalf("switch audit leaked raw app-server details: %#v", item)
		}
	}
	if !switchAuditFound {
		t.Fatalf("switch audit not found: %#v", audit.Items)
	}
}

func TestGatewaySkipsDisabledAPIKey(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("disabled-key")
	c.loginClient("disabled-key")
	c.approvePaidRecharge()

	firstCalls := 0
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		firstCalls++
		writeJSON(w, http.StatusOK, map[string]any{
			"model": "gpt-5.5",
			"usage": map[string]any{
				"input_tokens":  1,
				"output_tokens": 1,
				"total_tokens":  2,
			},
		})
	}))
	defer first.Close()
	firstID := c.createRoutedUpstream(first.URL)

	secondCalls := 0
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondCalls++
		writeJSON(w, http.StatusOK, map[string]any{
			"model": "gpt-5.5",
			"usage": map[string]any{
				"input_tokens":  4,
				"output_tokens": 3,
				"total_tokens":  7,
			},
		})
	}))
	defer second.Close()
	c.createRoutedUpstream(second.URL)

	var keys struct {
		Items []map[string]any `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/admin/api-keys", c.adminToken, nil, &keys); code != http.StatusOK {
		t.Fatalf("api keys status = %d", code)
	}
	var firstKeyID string
	for _, item := range keys.Items {
		if item["upstreamAccountId"] == firstID {
			firstKeyID, _ = item["id"].(string)
			break
		}
	}
	if firstKeyID == "" {
		t.Fatalf("first api key not found in %#v", keys.Items)
	}
	if code := c.request(http.MethodPost, "/api/admin/api-keys/"+firstKeyID+"/status", c.adminToken, map[string]any{"status": "disabled"}, nil); code != http.StatusOK {
		t.Fatalf("disable api key status = %d", code)
	}
	if code := c.request(http.MethodGet, "/api/admin/api-keys", c.adminToken, nil, &keys); code != http.StatusOK {
		t.Fatalf("api keys after disable status = %d", code)
	}
	for _, item := range keys.Items {
		if item["id"] == firstKeyID && item["routeAvailable"] != false {
			t.Fatalf("disabled api key still route available: %#v", item)
		}
	}

	var resp struct {
		ChargedTokens int64 `json:"chargedTokens"`
	}
	if code := c.gatewayRunWithHeaders(c.clientToken, map[string]string{"Idempotency-Key": "disabled-key"}, map[string]any{"model": "gpt-5.5", "input": "skip disabled key"}, &resp); code != http.StatusOK {
		t.Fatalf("gateway status = %d", code)
	}
	if firstCalls != 0 || secondCalls != 1 || resp.ChargedTokens != 7 {
		t.Fatalf("disabled key dispatch calls first=%d second=%d resp=%#v", firstCalls, secondCalls, resp)
	}
}

func TestGatewayUsesLeastRecentlyUsedAPIKey(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("lena")
	c.loginClient("lena")
	c.approvePaidRecharge()

	firstCalls := 0
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		firstCalls++
		writeJSON(w, http.StatusOK, map[string]any{
			"model": "gpt-5.5",
			"usage": map[string]any{
				"input_tokens":  2,
				"output_tokens": 2,
				"total_tokens":  4,
			},
		})
	}))
	defer first.Close()
	c.createRoutedUpstream(first.URL)

	secondCalls := 0
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondCalls++
		writeJSON(w, http.StatusOK, map[string]any{
			"model": "gpt-5.5",
			"usage": map[string]any{
				"input_tokens":  3,
				"output_tokens": 3,
				"total_tokens":  6,
			},
		})
	}))
	defer second.Close()
	c.createRoutedUpstream(second.URL)

	var firstResp struct {
		ChargedTokens int64 `json:"chargedTokens"`
	}
	if code := c.gatewayRunWithHeaders(c.clientToken, map[string]string{"Idempotency-Key": "lru-1"}, map[string]any{"model": "gpt-5.5", "input": "first"}, &firstResp); code != http.StatusOK {
		t.Fatalf("first gateway status = %d", code)
	}
	if firstCalls != 1 || secondCalls != 0 || firstResp.ChargedTokens != 4 {
		t.Fatalf("first dispatch calls first=%d second=%d resp=%#v", firstCalls, secondCalls, firstResp)
	}

	var secondResp struct {
		ChargedTokens int64 `json:"chargedTokens"`
	}
	if code := c.gatewayRunWithHeaders(c.clientToken, map[string]string{"Idempotency-Key": "lru-2"}, map[string]any{"model": "gpt-5.5", "input": "second"}, &secondResp); code != http.StatusOK {
		t.Fatalf("second gateway status = %d", code)
	}
	if firstCalls != 1 || secondCalls != 1 || secondResp.ChargedTokens != 6 {
		t.Fatalf("least recently used dispatch calls first=%d second=%d resp=%#v", firstCalls, secondCalls, secondResp)
	}

	var keys struct {
		Items []map[string]any `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/admin/api-keys", c.adminToken, nil, &keys); code != http.StatusOK {
		t.Fatalf("api keys status = %d", code)
	}
	if len(keys.Items) != 2 {
		t.Fatalf("api keys = %#v", keys.Items)
	}
	for _, item := range keys.Items {
		if item["lastUsedAt"] == nil {
			t.Fatalf("last used missing after dispatch: %#v", item)
		}
	}
}

func TestGatewayIdempotencyDoesNotDoubleCharge(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("erin")
	c.loginClient("erin")
	c.approvePaidRecharge()

	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		writeJSON(w, http.StatusOK, map[string]any{
			"model": "gpt-5.5",
			"usage": map[string]any{
				"input_tokens":  5,
				"output_tokens": 7,
				"total_tokens":  12,
			},
		})
	}))
	defer upstream.Close()
	c.createRoutedUpstream(upstream.URL)

	headers := map[string]string{"Idempotency-Key": "idem-erin-1"}
	var first, second struct {
		RequestID      string         `json:"requestId"`
		UsageRecord    map[string]any `json:"usageRecord"`
		ChargedTokens  int64          `json:"chargedTokens"`
		Idempotent     bool           `json:"idempotent"`
		UpstreamStatus any            `json:"upstreamStatus"`
	}
	if code := c.gatewayRunWithHeaders(c.clientToken, headers, map[string]any{"model": "gpt-5.5", "input": "hello"}, &first); code != http.StatusOK {
		t.Fatalf("first gateway status = %d", code)
	}
	if code := c.gatewayRunWithHeaders(c.clientToken, headers, map[string]any{"model": "gpt-5.5", "input": "hello again"}, &second); code != http.StatusOK {
		t.Fatalf("second gateway status = %d", code)
	}
	if calls != 1 {
		t.Fatalf("upstream calls = %d", calls)
	}
	if first.RequestID != "idem-erin-1" || second.RequestID != "idem-erin-1" || intField(first.UsageRecord, "totalTokens") != 12 || intField(second.UsageRecord, "totalTokens") != 12 {
		t.Fatalf("idempotency usage response first=%#v second=%#v", first, second)
	}
	for _, item := range []map[string]any{first.UsageRecord, second.UsageRecord} {
		for _, key := range []string{"id", "userId"} {
			if _, ok := item[key]; ok {
				t.Fatalf("gateway usage record leaked %q: %#v", key, item)
			}
		}
	}
	if first.UpstreamStatus != nil || second.UpstreamStatus != nil {
		t.Fatalf("gateway success leaked upstream status first=%#v second=%#v", first, second)
	}
	if first.ChargedTokens != 12 || second.ChargedTokens != 12 || !second.Idempotent {
		t.Fatalf("idempotency response first=%#v second=%#v", first, second)
	}
	var me struct {
		User map[string]any `json:"user"`
	}
	if code := c.request(http.MethodGet, "/api/client/me", c.clientToken, nil, &me); code != http.StatusOK {
		t.Fatalf("me status = %d", code)
	}
	if got := int64(me.User["tokenBalance"].(float64)); got != 99988 {
		t.Fatalf("token balance = %d", got)
	}
	var usage struct {
		Items []UsageRecord `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/client/usage", c.clientToken, nil, &usage); code != http.StatusOK {
		t.Fatalf("usage status = %d", code)
	}
	if len(usage.Items) != 1 {
		t.Fatalf("usage records = %#v", usage.Items)
	}
}

func TestGatewayRequestIDSanitizesSensitiveIdempotencyKey(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("safe-id")
	c.loginClient("safe-id")
	c.approvePaidRecharge()

	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		writeJSON(w, http.StatusOK, map[string]any{
			"model": "gpt-5.5",
			"usage": map[string]any{
				"input_tokens":  3,
				"output_tokens": 4,
				"total_tokens":  7,
			},
		})
	}))
	defer upstream.Close()
	c.createRoutedUpstream(upstream.URL)

	rawKey := `raw secret C:\Users\1\.codex\config.toml token=abc`
	headers := map[string]string{"Idempotency-Key": rawKey}
	var first, second struct {
		RequestID  string `json:"requestId"`
		Idempotent bool   `json:"idempotent"`
	}
	if code := c.gatewayRunWithHeaders(c.clientToken, headers, map[string]any{"model": "gpt-5.5", "input": "hello"}, &first); code != http.StatusOK {
		t.Fatalf("first gateway status = %d", code)
	}
	if code := c.gatewayRunWithHeaders(c.clientToken, headers, map[string]any{"model": "gpt-5.5", "input": "hello again"}, &second); code != http.StatusOK {
		t.Fatalf("second gateway status = %d", code)
	}
	if calls != 1 {
		t.Fatalf("upstream calls = %d", calls)
	}
	if first.RequestID != second.RequestID || !second.Idempotent {
		t.Fatalf("idempotency response first=%#v second=%#v", first, second)
	}
	if !strings.HasPrefix(first.RequestID, "req_") || strings.Contains(first.RequestID, "raw secret") || strings.Contains(first.RequestID, "token=abc") || strings.Contains(first.RequestID, "C:\\Users") {
		t.Fatalf("request id not sanitized: %#v", first)
	}
	var audit struct {
		Items []AuditLog `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/admin/audit?size=100", c.adminToken, nil, &audit); code != http.StatusOK {
		t.Fatalf("audit status = %d", code)
	}
	usageAuditFound := false
	for _, item := range audit.Items {
		if strings.Contains(item.TargetID, "raw secret") || strings.Contains(item.TargetID, "token=abc") || strings.Contains(item.TargetID, "C:\\Users") {
			t.Fatalf("audit target leaked raw request id: %#v", item)
		}
		if strings.Contains(item.Detail, "raw secret") || strings.Contains(item.Detail, "token=abc") || strings.Contains(item.Detail, "C:\\Users") {
			t.Fatalf("audit detail leaked raw request id: %#v", item)
		}
		if item.Action == "gateway.usage.debit" && item.TargetID == first.RequestID {
			usageAuditFound = true
		}
	}
	if !usageAuditFound {
		t.Fatalf("sanitized usage audit not found: %#v", audit.Items)
	}
}

func TestGatewayFailedRequestReleasesReservationForRetry(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("faye")
	c.loginClient("faye")
	c.approvePaidRecharge()

	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			writeJSON(w, http.StatusOK, map[string]any{"result": map[string]any{"error": "raw secret access token leaked by upstream"}})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"model": "gpt-5.5",
			"usage": map[string]any{
				"input_tokens":  1,
				"output_tokens": 2,
				"total_tokens":  3,
			},
		})
	}))
	defer upstream.Close()
	c.createRoutedUpstream(upstream.URL)

	headers := map[string]string{"Idempotency-Key": "idem-faye-retry"}
	if code := c.gatewayRunWithHeaders(c.clientToken, headers, map[string]any{"model": "gpt-5.5", "input": "first"}, nil); code != http.StatusBadGateway {
		t.Fatalf("first gateway status = %d", code)
	}
	var audit struct {
		Items []AuditLog `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/admin/audit?size=100", c.adminToken, nil, &audit); code != http.StatusOK {
		t.Fatalf("audit status = %d", code)
	}
	releaseAuditFound := false
	for _, item := range audit.Items {
		if item.Action != "gateway.request.release" {
			continue
		}
		releaseAuditFound = true
		if !strings.Contains(item.Detail, "error=codex_app_server_usage_missing") || !strings.Contains(item.Detail, "upstream_status=0") {
			t.Fatalf("release audit = %#v", item)
		}
		if strings.Contains(item.Detail, "raw secret") || strings.Contains(item.Detail, "access token") {
			t.Fatalf("release audit leaked raw upstream detail: %#v", item)
		}
	}
	if !releaseAuditFound {
		t.Fatalf("release audit not found: %#v", audit.Items)
	}
	var resp struct {
		UsageRecord   UsageRecord `json:"usageRecord"`
		ChargedTokens int64       `json:"chargedTokens"`
	}
	if code := c.gatewayRunWithHeaders(c.clientToken, headers, map[string]any{"model": "gpt-5.5", "input": "retry"}, &resp); code != http.StatusOK {
		t.Fatalf("retry gateway status = %d", code)
	}
	if calls != 2 {
		t.Fatalf("upstream calls = %d", calls)
	}
	if resp.UsageRecord.TotalTokens != 3 || resp.ChargedTokens != 3 {
		t.Fatalf("retry response = %#v", resp)
	}
	var me struct {
		User map[string]any `json:"user"`
	}
	if code := c.request(http.MethodGet, "/api/client/me", c.clientToken, nil, &me); code != http.StatusOK {
		t.Fatalf("me status = %d", code)
	}
	if got := int64(me.User["tokenBalance"].(float64)); got != 99997 {
		t.Fatalf("token balance = %d", got)
	}
}

func TestPostgresStoreRoundTrip(t *testing.T) {
	databaseURL := os.Getenv("CODEXPPP_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("CODEXPPP_TEST_DATABASE_URL is not set")
	}
	t.Setenv("CODEXPPP_DATABASE_URL", databaseURL)
	now := time.Now().UTC().Truncate(time.Second)
	store, err := OpenStore(filepath.Join(t.TempDir(), "unused.json"))
	if err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	store.state = State{
		NextID: 42,
		Admins: []Admin{{
			ID: "adm_1", Account: "root", PasswordSalt: "salt", PasswordHash: "hash", MustChangePassword: false, CreatedAt: now, UpdatedAt: now,
		}},
		TokenTopups: []TokenTopup{{
			ID: "topup_1", Name: "100K token", PriceCents: 990, Tokens: 100000, Enabled: true, Sort: 10, Description: "test", CreatedAt: now, UpdatedAt: now,
		}},
	}
	store.mu.Unlock()
	if err := store.save(); err != nil {
		t.Fatal(err)
	}
	if store.db != nil {
		_ = store.db.Close()
	}

	reopened, err := OpenStore(filepath.Join(t.TempDir(), "unused-2.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.db.Close()
	if reopened.state.NextID != 42 {
		t.Fatalf("next id = %d", reopened.state.NextID)
	}
	if len(reopened.state.Admins) != 1 || reopened.state.Admins[0].Account != "root" {
		t.Fatalf("admins not loaded from postgres: %#v", reopened.state.Admins)
	}
	if len(reopened.state.TokenTopups) != 1 || reopened.state.TokenTopups[0].Tokens != 100000 {
		t.Fatalf("topups not loaded from postgres: %#v", reopened.state.TokenTopups)
	}
}

func (c *testClient) approvePaidRecharge() {
	c.t.Helper()
	var topups struct {
		Items []TokenTopup `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/client/topups", c.clientToken, nil, &topups); code != http.StatusOK {
		c.t.Fatalf("topups status = %d", code)
	}
	var paidID string
	for _, topup := range topups.Items {
		if topup.PriceCents > 0 {
			paidID = topup.ID
			break
		}
	}
	if paidID == "" {
		c.t.Fatal("expected paid topup")
	}
	if code := c.request(http.MethodPost, "/api/client/recharges", c.clientToken, map[string]any{"topupId": paidID}, nil); code != http.StatusCreated {
		c.t.Fatalf("paid recharge status = %d", code)
	}
	var pending struct {
		Items []map[string]any `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/admin/recharges?status=pending&page=1&size=1", c.adminToken, nil, &pending); code != http.StatusOK {
		c.t.Fatalf("pending recharges status = %d", code)
	}
	if len(pending.Items) != 1 {
		c.t.Fatalf("pending recharge missing: %#v", pending.Items)
	}
	rechargeID, _ := pending.Items[0]["id"].(string)
	if rechargeID == "" {
		c.t.Fatalf("pending recharge id missing: %#v", pending.Items[0])
	}
	if code := c.request(http.MethodPost, "/api/admin/recharges/"+rechargeID+"/approve", c.adminToken, nil, nil); code != http.StatusOK {
		c.t.Fatalf("approve status = %d", code)
	}
}

func (c *testClient) createRoutedUpstream(serverURL string) string {
	c.t.Helper()
	var upstream struct {
		ID string `json:"id"`
	}
	accessToken := "access"
	if serverURL != "" {
		accessToken = "test-upstream-url:" + serverURL
	}
	upstreamBody := map[string]any{"name": "codex-upstream", "group": "default", "credentialType": "oauth", "accessToken": accessToken, "refreshToken": "refresh", "chatgptAccountId": "account-123", "subscriptionTier": "pro"}
	if code := c.request(http.MethodPost, "/api/admin/upstreams", c.adminToken, upstreamBody, &upstream); code != http.StatusCreated {
		c.t.Fatalf("upstream status = %d", code)
	}
	if code := c.request(http.MethodPost, "/api/admin/api-keys", c.adminToken, map[string]any{"upstreamAccountId": upstream.ID}, nil); code != http.StatusCreated {
		c.t.Fatalf("api key status = %d", code)
	}
	return upstream.ID
}

func (c *testClient) auditActionCounts() map[string]int {
	c.t.Helper()
	var audit struct {
		Items []AuditLog `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/admin/audit?size=100", c.adminToken, nil, &audit); code != http.StatusOK {
		c.t.Fatalf("audit status = %d", code)
	}
	out := make(map[string]int)
	for _, item := range audit.Items {
		out[item.Action]++
	}
	return out
}
