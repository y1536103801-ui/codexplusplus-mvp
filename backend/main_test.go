package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
)

type testClient struct {
	t           *testing.T
	handler     http.Handler
	app         *App
	store       *Store
	adminToken  string
	clientToken string
}

func testJWT(t *testing.T, payload map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	body := base64.RawURLEncoding.EncodeToString(raw)
	return header + "." + body + ".signature"
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
	if err := app.initializeRuntimeState(); err != nil {
		t.Fatal(err)
	}
	return &testClient{t: t, handler: app.routes(), app: app, store: store}
}

func installTestCodexRun(t *testing.T) {
	t.Helper()
	originalRun := codexResponsesRun
	originalStreamRun := codexResponsesStreamRun
	codexResponsesRun = func(ctx context.Context, credentials codexProbeCredentials, requestBody *gatewayRequestSource, requestHeaders http.Header) (codexResponsesResult, error) {
		const prefix = "test-upstream-url:"
		if !strings.HasPrefix(credentials.AccessToken, prefix) {
			return codexResponsesResult{}, fmt.Errorf("codex_responses_unavailable")
		}
		rawRequestBody, err := requestBody.Bytes()
		if err != nil {
			return codexResponsesResult{}, fmt.Errorf("codex_responses_request_invalid")
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimPrefix(credentials.AccessToken, prefix), bytes.NewReader(rawRequestBody))
		if err != nil {
			return codexResponsesResult{}, fmt.Errorf("codex_responses_request_invalid")
		}
		copyCodexRequestHeaders(req.Header, requestHeaders)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer access")
		req.Header.Set("ChatGPT-Account-ID", credentials.ChatGPTAccountID)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return codexResponsesResult{}, fmt.Errorf("codex_responses_unavailable")
		}
		defer res.Body.Close()
		raw, err := io.ReadAll(res.Body)
		if err != nil {
			return codexResponsesResult{}, fmt.Errorf("codex_responses_read_failed")
		}
		result := codexResponsesResult{Status: res.StatusCode, Header: filterCodexResponseHeaders(res.Header), Body: raw, ContentType: res.Header.Get("Content-Type")}
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			return result, nil
		}
		payload, usage, err := parseCodexResponsesPayload(raw, result.ContentType)
		if err != nil {
			return codexResponsesResult{}, err
		}
		result.Payload = payload
		result.Usage = usage
		return result, nil
	}
	codexResponsesStreamRun = func(ctx context.Context, credentials codexProbeCredentials, requestBody *gatewayRequestSource, requestHeaders http.Header, target *codexStreamTarget) (codexResponsesResult, error) {
		result, err := codexResponsesRun(ctx, credentials, requestBody, requestHeaders)
		if err != nil || result.Status < 200 || result.Status >= 300 {
			return result, err
		}
		for name, values := range result.Header {
			for _, value := range values {
				target.Writer.Header().Add(name, value)
			}
		}
		target.Writer.WriteHeader(result.Status)
		target.Started = true
		_, err = target.Writer.Write(result.Body)
		return result, err
	}
	t.Cleanup(func() {
		codexResponsesRun = originalRun
		codexResponsesStreamRun = originalStreamRun
	})
}

func completedCodexTestResult(model, text string, usage gatewayUsage) codexResponsesResult {
	payload := map[string]any{
		"id":          "resp_test",
		"object":      "response",
		"status":      "completed",
		"model":       model,
		"output_text": text,
		"usage":       appServerUsagePayload(usage),
	}
	raw, _ := json.Marshal(payload)
	return codexResponsesResult{
		Status:      http.StatusOK,
		Header:      http.Header{"Content-Type": []string{"application/json"}},
		Body:        raw,
		Payload:     payload,
		Usage:       usage,
		ContentType: "application/json",
	}
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
	headers = withClientInteropTestHeader(path, headers)
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
	headers = withClientInteropTestHeader(path, headers)
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

func withClientInteropTestHeader(path string, headers map[string]string) map[string]string {
	if !strings.HasPrefix(path, "/api/client/") {
		return headers
	}
	if headers == nil {
		headers = map[string]string{}
	}
	if _, ok := headers[clientInteropHeader]; !ok {
		headers[clientInteropHeader] = clientInteropMajor
	}
	return headers
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
		Account     any    `json:"account,omitempty"`
	} `json:"provider"`
	Diagnostics map[string]string `json:"diagnostics"`
}

func (c *testClient) prepareCodexProvider() launchPrepareResponse {
	c.t.Helper()
	var prepare launchPrepareResponse
	if code := c.request(http.MethodPost, "/api/client/launch/prepare", c.clientToken, map[string]any{"managedRuntime": true}, &prepare); code != http.StatusOK {
		c.t.Fatalf("launch prepare status = %d", code)
	}
	if prepare.LaunchState != "ready" || prepare.Provider.BearerToken == "" {
		c.t.Fatalf("launch prepare response = %#v", prepare)
	}
	if !isSub2APIStyleKey(prepare.Provider.BearerToken) {
		c.t.Fatalf("launch prepare provider key has invalid API-key shape")
	}
	if prepare.Provider.Account != nil {
		c.t.Fatalf("launch prepare must not return account display data: %#v", prepare.Provider.Account)
	}
	c.store.mu.Lock()
	found := false
	keyProblem := ""
	for _, key := range c.store.state.ClientAccessKeys {
		if key.KeyHash == hashString(prepare.Provider.BearerToken) {
			found = true
			if key.UserID == "" {
				keyProblem = fmt.Sprintf("launch prepare created a client access key without a user: %#v", key)
			}
			if key.DeviceID == "" {
				keyProblem = fmt.Sprintf("launch prepare created an unmanaged client access key: %#v", key)
			}
			if strings.TrimSpace(key.KeyCipher) == "" {
				keyProblem = fmt.Sprintf("launch prepare selected a client access key without stored secret: %#v", key)
			}
		}
	}
	for _, key := range c.store.state.APIKeys {
		if key.KeyHash == hashString(prepare.Provider.BearerToken) {
			keyProblem = fmt.Sprintf("launch prepare leaked an account-pool route key: %#v", key)
		}
	}
	c.store.mu.Unlock()
	if keyProblem != "" {
		c.t.Fatal(keyProblem)
	}
	if !found {
		c.t.Fatalf("launch prepare returned key outside client access keys")
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
		"remark TEXT NOT NULL DEFAULT ''",
		"access_token_cipher TEXT NOT NULL DEFAULT ''",
		"refresh_token_cipher TEXT NOT NULL DEFAULT ''",
		"auth_json_cipher TEXT NOT NULL DEFAULT ''",
		"source_type TEXT NOT NULL DEFAULT ''",
		"authorization_status TEXT NOT NULL DEFAULT 'authorized'",
		"password_cipher TEXT NOT NULL DEFAULT ''",
		"last_authorization_error TEXT NOT NULL DEFAULT ''",
		"credential_fingerprint TEXT NOT NULL DEFAULT ''",
		"CREATE TABLE IF NOT EXISTS api_keys",
		"key_cipher TEXT NOT NULL DEFAULT ''",
		"user_id TEXT NOT NULL DEFAULT ''",
		"last_used_at TIMESTAMPTZ",
		"CREATE TABLE IF NOT EXISTS client_access_keys",
		"idx_client_access_keys_user_status",
		"CREATE TABLE IF NOT EXISTS usage_records",
		"upstream_account_id TEXT NOT NULL DEFAULT ''",
		"api_key_id TEXT NOT NULL DEFAULT ''",
		"client_access_key_id TEXT NOT NULL DEFAULT ''",
		"session_id TEXT NOT NULL DEFAULT ''",
		"idx_usage_records_upstream_created",
		"idx_usage_records_session_created",
		"CREATE TABLE IF NOT EXISTS audit_logs",
		"actor_role IN ('admin', 'client', 'system')",
		"CREATE TABLE IF NOT EXISTS sessions",
		"role IN ('admin', 'client')",
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

func TestGatewayRuntimeMigrationPersistsSessionAffinity(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("migrations", "002_gateway_runtime.sql"))
	if err != nil {
		t.Fatal(err)
	}
	schema := string(body)
	for _, required := range []string{
		"CREATE TABLE IF NOT EXISTS gateway_session_routes",
		"PRIMARY KEY (user_id, session_key)",
		"REFERENCES upstream_accounts(id) ON DELETE CASCADE",
		"idx_gateway_session_routes_expiry",
	} {
		if !strings.Contains(schema, required) {
			t.Fatalf("gateway runtime migration missing %q", required)
		}
	}
}

func TestGatewayReplayRetentionMigrationIndexesCleanupCutoff(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("migrations", "004_gateway_replay_retention.sql"))
	if err != nil {
		t.Fatal(err)
	}
	schema := string(body)
	for _, required := range []string{
		"idx_idempotency_records_updated_at",
		"idempotency_records(updated_at)",
	} {
		if !strings.Contains(schema, required) {
			t.Fatalf("gateway replay retention migration missing %q", required)
		}
	}
}

func TestClientRuntimeControlMigrationBindsAccessKeysToDevices(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("migrations", "003_client_runtime_control.sql"))
	if err != nil {
		t.Fatal(err)
	}
	schema := string(body)
	for _, required := range []string{
		"ALTER TABLE client_access_keys",
		"ADD COLUMN IF NOT EXISTS device_id",
		"idx_client_access_keys_device_status",
	} {
		if !strings.Contains(schema, required) {
			t.Fatalf("client runtime control migration missing %q", required)
		}
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

func TestSessionTokensAreStoredAsNonReusableHashes(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	if len(c.store.state.Sessions) == 0 {
		t.Fatal("admin session was not stored")
	}
	stored := c.store.state.Sessions[0].Token
	if stored == c.adminToken || !strings.HasPrefix(stored, sessionTokenHashPrefix) {
		t.Fatalf("session token was stored in reusable form: %q", stored)
	}
	if stored != sessionTokenDigest(c.adminToken) {
		t.Fatalf("stored session digest does not match issued token")
	}
}

func TestAdminLoginIsRateLimited(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	for i := 0; i < maxAdminLoginAttempts; i++ {
		var out apiError
		if code := c.request(http.MethodPost, "/api/admin/login", "", map[string]any{"account": "root", "password": "wrong"}, &out); code != http.StatusUnauthorized {
			t.Fatalf("failed login %d status = %d error=%#v", i, code, out)
		}
	}
	var limited apiError
	if code := c.request(http.MethodPost, "/api/admin/login", "", map[string]any{"account": "root", "password": "wrong"}, &limited); code != http.StatusTooManyRequests {
		t.Fatalf("rate limited login status = %d error=%#v", code, limited)
	}
	if limited.Error != "login_rate_limited" {
		t.Fatalf("rate limited login error = %#v", limited)
	}
}

func TestReadyEndpointChecksConfiguredDependencies(t *testing.T) {
	c := newTestClient(t)
	var out map[string]any
	if code := c.request(http.MethodGet, "/api/ready", "", nil, &out); code != http.StatusOK {
		t.Fatalf("ready status = %d", code)
	}
	if out["status"] != "ready" {
		t.Fatalf("ready payload = %#v", out)
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
		"CODEXPPP_GATEWAY_UPSTREAM_CONCURRENCY: ${CODEXPPP_GATEWAY_UPSTREAM_CONCURRENCY:-2}",
		"CODEXPPP_GATEWAY_UPSTREAM_USER_LIMIT: ${CODEXPPP_GATEWAY_UPSTREAM_USER_LIMIT:-2}",
		"CODEXPPP_CODEX_COMMAND: ${CODEXPPP_CODEX_COMMAND:-codex}",
		"CODEXPPP_DESKTOP_LATEST_VERSION: ${CODEXPPP_DESKTOP_LATEST_VERSION:-}",
		"CODEXPPP_DESKTOP_DOWNLOAD_URL: ${CODEXPPP_DESKTOP_DOWNLOAD_URL:-}",
		"CODEXPPP_DESKTOP_DOWNLOAD_SHA256: ${CODEXPPP_DESKTOP_DOWNLOAD_SHA256:-}",
		"CODEXPPP_DESKTOP_RELEASE_NOTES: ${CODEXPPP_DESKTOP_RELEASE_NOTES:-}",
		"127.0.0.1:8787:8787",
		"curl -fsS http://127.0.0.1:8787/api/ready >/dev/null",
		`max-size: "20m"`,
		"127.0.0.1:54329:5432",
		"127.0.0.1:63799:6379",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("compose operational stack missing %q", required)
		}
	}
	for _, forbidden := range []string{
		`- "8787:8787"`,
		`- "1455:1455"`,
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
		"ARG TARGETARCH",
		`GOARCH="$TARGETARCH"`,
		"ENV CODEXPPP_CODEX_COMMAND=codex",
		`CMD ["codexppp-backend"]`,
	} {
		if !strings.Contains(image, required) {
			t.Fatalf("backend Dockerfile missing %q", required)
		}
	}
	if strings.Contains(image, "GOARCH=amd64") {
		t.Fatal("backend Dockerfile must not hardcode amd64 because free servers may use arm64")
	}
}

func TestServerBackupIncludesAllBackendModules(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "scripts", "backup-server.sh"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, required := range []string{
		`docker exec codexppp-postgres pg_dump`,
		`runtime_paths=(`,
		`  backend`,
		`deploy/docker-compose.yml`,
		`deploy/nginx`,
		`deploy/systemd`,
		`scripts/backup-server.sh`,
		`if [[ -e "${root}/${optional_path}" ]]`,
		`CODEXPPP_BACKUP_RETENTION_DAYS`,
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("server backup missing %q", required)
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

func TestPublicWebsiteServesProjectDownloadAndManualPurchase(t *testing.T) {
	c := newTestClient(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	c.handler.ServeHTTP(rr, req)
	res := rr.Result()
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("website status = %d", res.StatusCode)
	}
	if !strings.Contains(res.Header.Get("Content-Security-Policy"), "frame-ancestors 'none'") || res.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("website security headers = %#v", res.Header)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, required := range []string{
		"Codex+++",
		"不懂配置",
		"下载桌面客户端",
		"提交购买申请",
		"人工确认",
		"/api/site/config",
		"/api/site/orders",
		"与 OpenAI 不存在官方隶属关系",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("public website missing %q", required)
		}
	}
	if strings.Contains(text, `name="password"`) {
		t.Fatal("public purchase form must not collect a password")
	}
	if code := c.request(http.MethodGet, "/admin", "", nil, nil); code != http.StatusOK {
		t.Fatalf("admin console status = %d", code)
	}
}

func TestPublicAccountOrderIsEncryptedAndFulfilledByAdmin(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	t.Setenv("CODEXPPP_DESKTOP_LATEST_VERSION", "0.1.60")
	t.Setenv("CODEXPPP_DESKTOP_DOWNLOAD_URL", "https://example.com/codexppp-0.1.60.exe")
	t.Setenv("CODEXPPP_MACOS_DESKTOP_LATEST_VERSION", "0.1.60")
	t.Setenv("CODEXPPP_MACOS_DESKTOP_DOWNLOAD_URL", "https://example.com/codexppp-0.1.60.dmg")

	var config struct {
		Product      string           `json:"product"`
		PurchaseMode string           `json:"purchaseMode"`
		Plans        []map[string]any `json:"plans"`
		Downloads    map[string]struct {
			Version string `json:"version"`
			URL     string `json:"url"`
		} `json:"downloads"`
	}
	if code := c.request(http.MethodGet, "/api/site/config", "", nil, &config); code != http.StatusOK {
		t.Fatalf("site config status = %d", code)
	}
	if config.Product != "Codex+++" || config.PurchaseMode != "manual_review" || len(config.Plans) != 1 {
		t.Fatalf("site config = %#v", config)
	}
	if config.Downloads["windows"].Version != "0.1.60" || config.Downloads["windows"].URL != "https://example.com/codexppp-0.1.60.exe" || config.Downloads["macos"].Version != "0.1.60" || config.Downloads["macos"].URL != "https://example.com/codexppp-0.1.60.dmg" {
		t.Fatalf("platform downloads = %#v", config.Downloads)
	}
	if config.Plans[0]["id"] != "topup_100k" || int64(config.Plans[0]["priceCents"].(float64)) != 990 {
		t.Fatalf("public paid plan = %#v", config.Plans[0])
	}
	for _, forbidden := range []string{"enabled", "createdAt", "updatedAt"} {
		if _, ok := config.Plans[0][forbidden]; ok {
			t.Fatalf("public plan exposed internal field %q: %#v", forbidden, config.Plans[0])
		}
	}

	contact := "buyer@example.com"
	preferred := "buyer-2026"
	remark := "请通过邮箱联系"
	var created struct {
		OrderID string `json:"orderId"`
		Status  string `json:"status"`
	}
	requestBody := map[string]any{
		"topupId": "topup_100k", "contact": contact, "preferredAccount": preferred,
		"remark": remark, "website": "",
	}
	if code := c.request(http.MethodPost, "/api/site/orders", "", requestBody, &created); code != http.StatusCreated {
		t.Fatalf("create public account order status = %d", code)
	}
	if created.OrderID == "" || created.Status != accountOrderPending {
		t.Fatalf("created account order = %#v", created)
	}

	c.store.mu.Lock()
	if len(c.store.state.AccountOrders) != 1 {
		c.store.mu.Unlock()
		t.Fatalf("stored account orders = %d", len(c.store.state.AccountOrders))
	}
	stored := c.store.state.AccountOrders[0]
	c.store.mu.Unlock()
	for _, plaintext := range []string{contact, preferred, remark} {
		if strings.Contains(stored.BuyerCipher, plaintext) {
			t.Fatalf("stored buyer cipher leaked %q", plaintext)
		}
	}
	decrypted, err := c.app.decrypt(stored.BuyerCipher)
	if err != nil {
		t.Fatal(err)
	}
	var buyer accountOrderBuyer
	if err := json.Unmarshal([]byte(decrypted), &buyer); err != nil {
		t.Fatal(err)
	}
	if buyer.Contact != contact || buyer.PreferredAccount != preferred || buyer.Remark != remark {
		t.Fatalf("decrypted buyer = %#v", buyer)
	}

	var orders struct {
		Items []map[string]any `json:"items"`
		Total int              `json:"total"`
	}
	if code := c.request(http.MethodGet, "/api/admin/account-orders?status=pending", c.adminToken, nil, &orders); code != http.StatusOK {
		t.Fatalf("list account orders status = %d", code)
	}
	if orders.Total != 1 || len(orders.Items) != 1 || orders.Items[0]["contact"] != contact {
		t.Fatalf("admin account orders = %#v", orders)
	}
	fulfillPath := "/api/admin/account-orders/" + created.OrderID + "/status"
	if code := c.request(http.MethodPost, fulfillPath, c.adminToken, map[string]any{"status": "fulfilled", "userAccount": "missing", "adminRemark": ""}, nil); code != http.StatusBadRequest {
		t.Fatalf("fulfill missing user status = %d", code)
	}

	c.createUser(preferred)
	if code := c.request(http.MethodPost, fulfillPath, c.adminToken, map[string]any{"status": "fulfilled", "userAccount": preferred, "adminRemark": "已确认收款"}, nil); code != http.StatusOK {
		t.Fatalf("fulfill account order status = %d", code)
	}
	c.store.mu.Lock()
	var fulfilledUser User
	for _, user := range c.store.state.Users {
		if user.Account == preferred {
			fulfilledUser = user
			break
		}
	}
	fulfilledOrder := c.store.state.AccountOrders[0]
	ledgers := append([]TokenLedger(nil), c.store.state.TokenLedgers...)
	c.store.mu.Unlock()
	if fulfilledUser.TokenBalance != 100000 || fulfilledOrder.Status != accountOrderFulfilled || fulfilledOrder.UserID != fulfilledUser.ID || fulfilledOrder.FulfilledAt == nil {
		t.Fatalf("fulfilled order/user = %#v / %#v", fulfilledOrder, fulfilledUser)
	}
	if len(ledgers) != 1 || ledgers[0].DeltaTokens != 100000 || ledgers[0].Source != "官网购买：100K token" {
		t.Fatalf("fulfillment ledger = %#v", ledgers)
	}
	if code := c.request(http.MethodPost, fulfillPath, c.adminToken, map[string]any{"status": "fulfilled", "userAccount": preferred, "adminRemark": "重复"}, nil); code != http.StatusConflict {
		t.Fatalf("repeat fulfillment status = %d", code)
	}

	var audit struct {
		Items []map[string]any `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/admin/audit", c.adminToken, nil, &audit); code != http.StatusOK {
		t.Fatalf("audit status = %d", code)
	}
	auditJSON, err := json.Marshal(audit)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(auditJSON), contact) || strings.Contains(string(auditJSON), remark) {
		t.Fatalf("audit leaked buyer details: %s", auditJSON)
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
	if got := res.Header.Get("Access-Control-Allow-Headers"); !strings.Contains(got, "Authorization") || !strings.Contains(got, "Idempotency-Key") || !strings.Contains(got, clientInteropHeader) {
		t.Fatalf("allow headers = %q", got)
	}
}

func TestClientAPIRequiresMatchingInteropMajor(t *testing.T) {
	c := newTestClient(t)
	for _, tt := range []struct {
		name   string
		header string
	}{
		{name: "missing"},
		{name: "mismatch", header: "2"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var out apiError
			req := httptest.NewRequest(http.MethodGet, "/api/client/me", nil)
			if tt.header != "" {
				req.Header.Set(clientInteropHeader, tt.header)
			}
			rr := httptest.NewRecorder()

			c.handler.ServeHTTP(rr, req)

			res := rr.Result()
			defer res.Body.Close()
			_ = json.NewDecoder(res.Body).Decode(&out)
			if res.StatusCode != http.StatusUpgradeRequired {
				t.Fatalf("status = %d", res.StatusCode)
			}
			if out.Error != "client_version_incompatible" {
				t.Fatalf("error = %q", out.Error)
			}
			if got := res.Header.Get(clientInteropHeader); got != clientInteropMajor {
				t.Fatalf("response interop major = %q", got)
			}
		})
	}

	req := httptest.NewRequest(http.MethodGet, "/api/client/me", nil)
	req.Header.Set(clientInteropHeader, clientInteropMajor)
	rr := httptest.NewRecorder()
	c.handler.ServeHTTP(rr, req)
	if rr.Result().StatusCode != http.StatusUnauthorized {
		t.Fatalf("matched interop should continue to auth, status = %d", rr.Result().StatusCode)
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

	t.Setenv("CODEXPPP_MACOS_DESKTOP_LATEST_VERSION", "0.3.0")
	t.Setenv("CODEXPPP_MACOS_DESKTOP_DOWNLOAD_URL", "https://example.com/codexppp-0.3.0.dmg")
	t.Setenv("CODEXPPP_MACOS_DESKTOP_DOWNLOAD_SHA256", "def456")
	t.Setenv("CODEXPPP_MACOS_DESKTOP_RELEASE_NOTES", "新增 macOS 客户端")
	var macUpdate map[string]any
	if code := c.request(http.MethodGet, "/api/client/desktop/update?currentVersion=0.1.0&platform=macos", c.clientToken, nil, &macUpdate); code != http.StatusOK {
		t.Fatalf("macOS desktop update status = %d", code)
	}
	if macUpdate["platform"] != "macos" || macUpdate["available"] != true || macUpdate["latestVersion"] != "0.3.0" || macUpdate["downloadUrl"] != "https://example.com/codexppp-0.3.0.dmg" || macUpdate["sha256"] != "def456" {
		t.Fatalf("macOS update payload = %#v", macUpdate)
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
	if len(c.store.state.APIKeys) != 1 || strings.TrimSpace(c.store.state.APIKeys[0].KeyCipher) == "" {
		t.Fatalf("upstream must create one stored account-pool api key: %#v", c.store.state.APIKeys)
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

func TestClientLaunchCredentialsDoNotReserveAccountPoolRoutes(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"id":     "resp_shared_route",
			"object": "response",
			"status": "completed",
			"model":  "codex",
			"output": []any{},
			"usage": map[string]any{
				"input_tokens":  2,
				"output_tokens": 3,
				"total_tokens":  5,
			},
		})
	}))
	defer upstream.Close()
	var poolAccount struct {
		ID string `json:"id"`
	}
	if code := c.request(http.MethodPost, "/api/admin/upstreams", c.adminToken, map[string]any{
		"name":             "shared-route",
		"credentialType":   "oauth",
		"accessToken":      "test-upstream-url:" + upstream.URL,
		"chatgptAccountId": "account-shared",
		"subscriptionTier": "pro",
	}, &poolAccount); code != http.StatusCreated {
		t.Fatalf("create shared upstream status = %d", code)
	}

	c.createUser("shared-route-user-1")
	c.loginClient("shared-route-user-1")
	c.approvePaidRecharge()
	clientSession1 := c.clientToken
	var prepare1 launchPrepareResponse
	if code := c.request(http.MethodPost, "/api/client/launch/prepare", clientSession1, map[string]any{"managedRuntime": true}, &prepare1); code != http.StatusOK {
		t.Fatalf("first launch prepare status = %d", code)
	}

	c.createUser("shared-route-user-2")
	c.loginClient("shared-route-user-2")
	c.approvePaidRecharge()
	clientSession2 := c.clientToken
	var prepare2 launchPrepareResponse
	if code := c.request(http.MethodPost, "/api/client/launch/prepare", clientSession2, map[string]any{"managedRuntime": true}, &prepare2); code != http.StatusOK {
		t.Fatalf("second launch prepare status = %d", code)
	}
	if prepare1.Provider.BearerToken == "" || prepare2.Provider.BearerToken == "" || prepare1.Provider.BearerToken == prepare2.Provider.BearerToken {
		t.Fatalf("client access tokens must be non-empty and user-scoped")
	}

	for i, providerToken := range []string{prepare1.Provider.BearerToken, prepare2.Provider.BearerToken} {
		headers := map[string]string{"Idempotency-Key": fmt.Sprintf("shared-route-user-%d", i+1)}
		if code := c.requestWithHeaders(http.MethodPost, "/api/codex/v1/responses", providerToken, headers, map[string]any{"model": "codex", "input": "hello"}, nil); code != http.StatusOK {
			t.Fatalf("user %d responses status = %d", i+1, code)
		}
	}

	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	if len(c.store.state.ClientAccessKeys) != 2 {
		t.Fatalf("client access key count = %d, want 2", len(c.store.state.ClientAccessKeys))
	}
	if len(c.store.state.APIKeys) != 1 {
		t.Fatalf("account-pool route key count = %d, want 1", len(c.store.state.APIKeys))
	}
	for _, routeKey := range c.store.state.APIKeys {
		if routeKey.UserID != "" {
			t.Fatalf("account-pool route remained bound to a user: %#v", routeKey)
		}
		for _, providerToken := range []string{prepare1.Provider.BearerToken, prepare2.Provider.BearerToken} {
			if routeKey.KeyHash == hashString(providerToken) {
				t.Fatalf("client credential reused account-pool route key: %#v", routeKey)
			}
		}
	}
	if len(c.store.state.UsageRecords) != 2 {
		t.Fatalf("usage record count = %d, want 2", len(c.store.state.UsageRecords))
	}
	seenClientKeys := map[string]bool{}
	for _, record := range c.store.state.UsageRecords {
		if record.UpstreamAccountID != poolAccount.ID || record.APIKeyID != c.store.state.APIKeys[0].ID || record.ClientAccessKeyID == "" {
			t.Fatalf("usage did not separate user credential from pool route: %#v", record)
		}
		seenClientKeys[record.ClientAccessKeyID] = true
	}
	if len(seenClientKeys) != 2 {
		t.Fatalf("usage records did not preserve both client credentials: %#v", seenClientKeys)
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

func TestCodexResponsesAcceptsZstdRequestBody(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("codex-zstd")
	c.loginClient("codex-zstd")
	c.approvePaidRecharge()

	var received map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Content-Encoding"); got != "" {
			t.Fatalf("compressed content encoding leaked upstream: %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatal(err)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"id": "resp_zstd", "object": "response", "status": "completed", "model": "gpt-5.5",
			"output_text": "zstd request accepted",
			"usage":       map[string]any{"input_tokens": 7, "output_tokens": 5, "total_tokens": 12},
		})
	}))
	defer upstream.Close()
	c.createRoutedUpstream(upstream.URL)
	providerToken := c.prepareCodexProviderToken()

	raw := []byte(`{"model":"gpt-5.5","stream":true,"input":"continue the existing task"}`)
	encoder, err := zstd.NewWriter(nil, zstd.WithEncoderConcurrency(1))
	if err != nil {
		t.Fatal(err)
	}
	compressed := encoder.EncodeAll(raw, nil)
	encoder.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/codex/v1/responses", bytes.NewReader(compressed))
	req.Header.Set("Authorization", "Bearer "+providerToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "zstd")
	recorder := httptest.NewRecorder()
	c.handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("responses status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	if received["model"] != "gpt-5.5" || received["input"] != "continue the existing task" {
		t.Fatalf("decompressed upstream request = %#v", received)
	}
}

func TestCodexResponsesRejectsInvalidZstdWithStructuredError(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("codex-invalid-zstd")
	c.loginClient("codex-invalid-zstd")
	c.approvePaidRecharge()
	c.createRoutedUpstream("")
	providerToken := c.prepareCodexProviderToken()

	req := httptest.NewRequest(http.MethodPost, "/api/codex/v1/responses", strings.NewReader("not-a-zstd-frame"))
	req.Header.Set("Authorization", "Bearer "+providerToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "zstd")
	recorder := httptest.NewRecorder()
	c.handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("responses status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	var out struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Error.Code != "invalid_json" {
		t.Fatalf("responses error = %#v", out.Error)
	}
}

func TestGatewayRequestBodyRejectsEncodedAndDecodedOverflow(t *testing.T) {
	encodedOverflow := httptest.NewRequest(http.MethodPost, "/api/codex/v1/responses", strings.NewReader(`{}`))
	encodedOverflow.ContentLength = 1025
	if _, bodyErr := readGatewayRequestBodyWithinLimits(encodedOverflow, 1024, 1024); bodyErr == nil || bodyErr.Status != http.StatusRequestEntityTooLarge || bodyErr.Code != "request_too_large" {
		t.Fatalf("encoded overflow error = %#v", bodyErr)
	}

	encoder, err := zstd.NewWriter(nil, zstd.WithEncoderConcurrency(1), zstd.WithWindowSize(1024))
	if err != nil {
		t.Fatal(err)
	}
	oversizedJSON := append([]byte(`{"input":"`), bytes.Repeat([]byte("x"), 2048)...)
	oversizedJSON = append(oversizedJSON, []byte(`"}`)...)
	compressed := encoder.EncodeAll(oversizedJSON, nil)
	encoder.Close()
	decodedOverflow := httptest.NewRequest(http.MethodPost, "/api/codex/v1/responses", bytes.NewReader(compressed))
	decodedOverflow.Header.Set("Content-Encoding", "zstd")
	if _, bodyErr := readGatewayRequestBodyWithinLimits(decodedOverflow, 4096, 1024); bodyErr == nil || bodyErr.Status != http.StatusRequestEntityTooLarge || bodyErr.Code != "request_too_large" {
		t.Fatalf("decoded overflow error = %#v", bodyErr)
	}
}

func TestGatewayRequestBodyStreamsToPrivateFileAndCapturesMetadata(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("CODEXPPP_GATEWAY_TEMP_DIR", tempDir)
	largeInput := strings.Repeat("x", 2<<20)
	raw := []byte(`{
		"requestId":"desktop-request-1",
		"route":"client-must-not-control-this",
		"model":"gpt-5.5",
		"stream":true,
		"max_output_tokens":8192,
		"metadata":{"thread_id":"desktop-thread-1"},
		"input":"` + largeInput + `"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/codex/v1/responses", bytes.NewReader(raw))
	prepared, bodyErr := prepareGatewayRequest(req)
	if bodyErr != nil {
		t.Fatal(bodyErr.Cause)
	}
	path := prepared.path
	defer prepared.Close()
	if path == "" || len(prepared.memory) != 0 {
		t.Fatalf("request was not spooled: path=%q memory=%d", path, len(prepared.memory))
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("temporary request mode = %o", info.Mode().Perm())
	}
	if prepared.metadata.Model != "gpt-5.5" || prepared.metadata.RequestID != "desktop-request-1" || !prepared.metadata.Stream || prepared.metadata.MaxOutputTokens != 8192 || prepared.metadata.SessionID != "desktop-thread-1" {
		t.Fatalf("request metadata = %#v", prepared.metadata)
	}
	body, err := prepared.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	if _, exists := payload["requestId"]; exists {
		t.Fatal("requestId leaked upstream")
	}
	if _, exists := payload["route"]; exists {
		t.Fatal("client route leaked upstream")
	}
	if payload["input"] != largeInput {
		t.Fatalf("large input was not preserved: length=%d", len(stringField(payload, "input")))
	}
	prepared.Close()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary request was not removed: %v", err)
	}
}

func TestGatewayUploadAdmissionLimitsGlobalAndPerUserConcurrency(t *testing.T) {
	app := &App{gatewayUploadLimit: 2, gatewayUploadUserLimit: 1, gatewayUploads: map[string]int{}}
	if !app.acquireGatewayUploadSlot("user-1") {
		t.Fatal("first user upload was rejected")
	}
	if app.acquireGatewayUploadSlot("user-1") {
		t.Fatal("second upload for the same user was accepted")
	}
	if !app.acquireGatewayUploadSlot("user-2") {
		t.Fatal("second global upload was rejected")
	}
	if app.acquireGatewayUploadSlot("user-3") {
		t.Fatal("global upload concurrency was exceeded")
	}
	app.releaseGatewayUploadSlot("user-1")
	if !app.acquireGatewayUploadSlot("user-3") {
		t.Fatal("released upload slot was not reusable")
	}
}

func TestCodexResponsesOversizeAndUploadCapacityUseStructuredProviderErrors(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("codex-upload-errors")
	c.loginClient("codex-upload-errors")
	c.approvePaidRecharge()
	c.createRoutedUpstream("")
	providerToken := c.prepareCodexProviderToken()

	oversized := httptest.NewRequest(http.MethodPost, "/api/codex/v1/responses", strings.NewReader(`{}`))
	oversized.Header.Set("Authorization", "Bearer "+providerToken)
	oversized.Header.Set("Content-Type", "application/json")
	oversized.ContentLength = maxGatewayEncodedBodyBytes + 1
	oversizedRecorder := httptest.NewRecorder()
	c.handler.ServeHTTP(oversizedRecorder, oversized)
	if oversizedRecorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized status = %d body=%s", oversizedRecorder.Code, oversizedRecorder.Body.String())
	}
	var oversizedPayload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(oversizedRecorder.Body).Decode(&oversizedPayload); err != nil {
		t.Fatal(err)
	}
	if oversizedPayload.Error.Code != "request_too_large" {
		t.Fatalf("oversized error = %#v", oversizedPayload.Error)
	}

	c.store.mu.Lock()
	userID := c.store.state.Users[0].ID
	c.store.mu.Unlock()
	if !c.app.acquireGatewayUploadSlot(userID) {
		t.Fatal("failed to reserve test upload slot")
	}
	defer c.app.releaseGatewayUploadSlot(userID)
	busy := httptest.NewRequest(http.MethodPost, "/api/codex/v1/responses", strings.NewReader(`{}`))
	busy.Header.Set("Authorization", "Bearer "+providerToken)
	busy.Header.Set("Content-Type", "application/json")
	busyRecorder := httptest.NewRecorder()
	c.handler.ServeHTTP(busyRecorder, busy)
	if busyRecorder.Code != http.StatusTooManyRequests || busyRecorder.Header().Get("Retry-After") != "2" {
		t.Fatalf("busy status = %d retry=%q body=%s", busyRecorder.Code, busyRecorder.Header().Get("Retry-After"), busyRecorder.Body.String())
	}
	var busyPayload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(busyRecorder.Body).Decode(&busyPayload); err != nil {
		t.Fatal(err)
	}
	if busyPayload.Error.Code != "upload_capacity_exhausted" {
		t.Fatalf("busy error = %#v", busyPayload.Error)
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
			"id":          "resp_stored",
			"object":      "response",
			"status":      "completed",
			"model":       "codex",
			"output_text": "stored provider output",
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

func TestCodexResponsesPreservesToolCallSSEAndStickyHeader(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("codex-tool-stream")
	c.loginClient("codex-tool-stream")
	c.approvePaidRecharge()

	completed := map[string]any{
		"type": "response.completed",
		"response": map[string]any{
			"id":     "resp_tool",
			"object": "response",
			"status": "completed",
			"model":  "gpt-5.5",
			"output": []any{map[string]any{
				"id":        "call_1",
				"type":      "function_call",
				"call_id":   "call_local_shell_1",
				"name":      "local_shell",
				"arguments": `{"command":"rg --files"}`,
			}},
			"usage": map[string]any{"input_tokens": 8, "output_tokens": 4, "total_tokens": 12},
		},
	}
	completedJSON, _ := json.Marshal(completed)
	stream := "event: response.output_item.done\ndata: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"function_call\",\"name\":\"local_shell\"}}\n\n" +
		"event: response.completed\ndata: " + string(completedJSON) + "\n\ndata: [DONE]\n\n"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Thread-ID"); got != "thread-local-1" {
			t.Fatalf("thread-id = %q", got)
		}
		if got := r.Header.Get("X-Codex-Turn-State"); got != "turn-in" {
			t.Fatalf("turn state = %q", got)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if _, ok := payload["tools"]; !ok {
			t.Fatalf("tools missing from upstream request: %#v", payload)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("X-Codex-Turn-State", "turn-out")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(stream))
	}))
	defer upstream.Close()
	c.createRoutedUpstream(upstream.URL)
	providerToken := c.prepareCodexProviderToken()

	req := httptest.NewRequest(http.MethodPost, "/api/codex/v1/responses", strings.NewReader(`{"model":"gpt-5.5","stream":true,"input":[{"role":"user","content":"inspect"}],"tools":[{"type":"function","name":"local_shell"}]}`))
	req.Header.Set("Authorization", "Bearer "+providerToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Thread-ID", "thread-local-1")
	req.Header.Set("X-Codex-Turn-State", "turn-in")
	recorder := httptest.NewRecorder()
	c.handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("responses status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Type"); !strings.Contains(got, "text/event-stream") {
		t.Fatalf("content type = %q", got)
	}
	if got := recorder.Header().Get("X-Codex-Turn-State"); got != "turn-out" {
		t.Fatalf("response turn state = %q", got)
	}
	if got := recorder.Body.String(); got != stream {
		t.Fatalf("stream was rewritten\nwant: %q\n got: %q", stream, got)
	}
	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	if len(c.store.state.GatewayRequests) != 1 || c.store.state.GatewayRequests[0].ResultBody != stream {
		t.Fatalf("raw idempotency response was not persisted: %#v", c.store.state.GatewayRequests)
	}
	if c.store.state.Users[0].TokenBalance != 99988 {
		t.Fatalf("token balance = %d", c.store.state.Users[0].TokenBalance)
	}
}

func TestGatewayReplayBodyExpiresWithoutLosingIdempotencyMetadata(t *testing.T) {
	store := &Store{}
	now := time.Now().UTC()
	request := GatewayRequest{
		UserID:         "usr_replay",
		RequestID:      "request_replay",
		Status:         gatewayCompleted,
		UpstreamStatus: http.StatusOK,
		ResultBody:     `{"id":"response_replay"}`,
		ResultType:     "application/json",
		UpdatedAt:      now,
	}
	result, found, err := store.loadGatewayReplay(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !found || string(result.Body) != request.ResultBody {
		t.Fatalf("fresh replay = found:%v body:%q", found, string(result.Body))
	}
	request.UpdatedAt = now.Add(-gatewayReplayBodyRetention - time.Second)
	result, found, err = store.loadGatewayReplay(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if found || len(result.Body) != 0 {
		t.Fatalf("expired replay = found:%v body:%q", found, string(result.Body))
	}
	if request.Status != gatewayCompleted {
		t.Fatalf("idempotency metadata changed: %#v", request)
	}
}

func TestRunCodexResponsesRequestUsesPoolAuthAndForwardsProtocolHeaders(t *testing.T) {
	var received map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer pool-access" {
			t.Fatalf("authorization = %q", got)
		}
		if got := r.Header.Get("ChatGPT-Account-ID"); got != "account-pool-1" {
			t.Fatalf("account id = %q", got)
		}
		if got := r.Header.Get("Authorization"); strings.Contains(got, "client-provider-key") {
			t.Fatalf("client provider authorization leaked upstream: %q", got)
		}
		if got := r.Header.Get("X-Codex-Installation-ID"); got != "install-1" {
			t.Fatalf("installation id = %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatal(err)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"id": "resp_direct", "object": "response", "status": "completed", "model": "gpt-5.5",
			"output": []any{map[string]any{"type": "function_call", "name": "local_shell"}},
			"usage":  map[string]any{"input_tokens": 3, "output_tokens": 2, "total_tokens": 5},
		})
	}))
	defer upstream.Close()
	t.Setenv("CODEXPPP_CODEX_RESPONSES_URL", upstream.URL)
	headers := http.Header{
		"Authorization":           []string{"Bearer client-provider-key"},
		"X-Codex-Installation-ID": []string{"install-1"},
		"Thread-ID":               []string{"thread-1"},
	}
	body := []byte(`{"model":"gpt-5.5","tools":[{"type":"function","name":"local_shell"}]}`)
	result, err := runCodexResponsesRequest(context.Background(), codexProbeCredentials{AccessToken: "pool-access", ChatGPTAccountID: "account-pool-1"}, body, headers)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != http.StatusOK || result.Usage.TotalTokens != 5 {
		t.Fatalf("result = %#v", result)
	}
	if _, ok := received["tools"]; !ok {
		t.Fatalf("tool definitions were not forwarded: %#v", received)
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
	req := httptest.NewRequest(http.MethodPost, "/api/gateway/runs", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+c.clientToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "oversized-body")
	req.ContentLength = maxGatewayEncodedBodyBytes + 1
	recorder := httptest.NewRecorder()
	c.app.gatewayRun(recorder, req)
	var gatewayErr apiError
	_ = json.NewDecoder(recorder.Body).Decode(&gatewayErr)
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized gateway request status = %d", recorder.Code)
	}
	if gatewayErr.Error != "request_too_large" {
		t.Fatalf("oversized gateway request error = %#v", gatewayErr)
	}
}

func TestGatewayRequestBodyAllowsCurrentDesktopContextSize(t *testing.T) {
	const observedDesktopRequestBytes int64 = 67222675
	if observedDesktopRequestBytes <= 64<<20 || observedDesktopRequestBytes >= maxGatewayEncodedBodyBytes {
		t.Fatalf("observed request size %d is outside the intended regression range", observedDesktopRequestBytes)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/codex/v1/responses", strings.NewReader(`{}`))
	req.ContentLength = observedDesktopRequestBytes
	body, bodyErr := readGatewayRequestBody(req)
	if bodyErr != nil || string(body) != `{}` {
		t.Fatalf("current desktop context rejected: body=%q err=%#v", body, bodyErr)
	}
}

func TestNginxStreamsCodexRequestsToBoundedBackend(t *testing.T) {
	if maxGatewayEncodedBodyBytes != 1<<30 || maxGatewayDecodedBodyBytes != 1<<30 {
		t.Fatalf("gateway limits = encoded %d decoded %d", maxGatewayEncodedBodyBytes, maxGatewayDecodedBodyBytes)
	}
	body, err := os.ReadFile(filepath.Join("..", "deploy", "nginx", "codex.52cx.top.conf"))
	if err != nil {
		t.Fatal(err)
	}
	config := string(body)
	if !strings.Contains(config, "location = /api/codex/v1/responses") || !strings.Contains(config, "client_max_body_size 0;") || !strings.Contains(config, "proxy_request_buffering off;") {
		t.Fatal("nginx Codex route is not streamed to the bounded backend")
	}
	if !strings.Contains(config, "client_max_body_size 1m;") {
		t.Fatal("ordinary management routes lost their small request limit")
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

func TestAdminUsageAnalyticsAggregatesAccountSessionsByDay(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("analytics-user")
	accountID := c.createRoutedUpstream("")
	otherAccountID := c.createRoutedUpstream("")

	c.store.mu.Lock()
	userID := c.store.state.Users[0].ID
	c.store.state.UsageRecords = []UsageRecord{
		{ID: c.store.nextID("use"), UserID: userID, UpstreamAccountID: accountID, SessionID: "thread-a", InputTokens: 70, OutputTokens: 30, TotalTokens: 100, CreatedAt: time.Date(2026, 7, 10, 2, 0, 0, 0, time.UTC)},
		{ID: c.store.nextID("use"), UserID: userID, UpstreamAccountID: accountID, SessionID: "thread-a", InputTokens: 250, CachedInputTokens: 20, OutputTokens: 30, TotalTokens: 300, CreatedAt: time.Date(2026, 7, 10, 3, 0, 0, 0, time.UTC)},
		{ID: c.store.nextID("use"), UserID: userID, UpstreamAccountID: accountID, SessionID: "thread-b", InputTokens: 150, OutputTokens: 50, TotalTokens: 200, CreatedAt: time.Date(2026, 7, 10, 4, 0, 0, 0, time.UTC)},
		{ID: c.store.nextID("use"), UserID: userID, UpstreamAccountID: accountID, InputTokens: 90, OutputTokens: 10, TotalTokens: 100, CreatedAt: time.Date(2026, 7, 10, 5, 0, 0, 0, time.UTC)},
		{ID: c.store.nextID("use"), UserID: userID, UpstreamAccountID: accountID, SessionID: "thread-c", InputTokens: 400, OutputTokens: 100, TotalTokens: 500, CreatedAt: time.Date(2026, 7, 11, 2, 0, 0, 0, time.UTC)},
		{ID: c.store.nextID("use"), UserID: userID, UpstreamAccountID: otherAccountID, SessionID: "other", TotalTokens: 900, CreatedAt: time.Date(2026, 7, 9, 2, 0, 0, 0, time.UTC)},
	}
	c.store.mu.Unlock()

	type analyticsPoint struct {
		Bucket               string  `json:"bucket"`
		AccountID            string  `json:"accountId"`
		TaskCount            int64   `json:"taskCount"`
		RecordCount          int64   `json:"recordCount"`
		CachedInputTokens    int64   `json:"cachedInputTokens"`
		TotalTokens          int64   `json:"totalTokens"`
		AverageTokensPerTask float64 `json:"averageTokensPerTask"`
		HistoricalFallback   int64   `json:"historicalFallback"`
	}
	type analyticsResponse struct {
		SelectedAccountIDs []string `json:"selectedAccountIds"`
		Query              struct {
			Metric   string `json:"metric"`
			GroupBy  string `json:"groupBy"`
			Timezone string `json:"timezone"`
		} `json:"query"`
		Summary struct {
			TaskCount            int64   `json:"taskCount"`
			RecordCount          int64   `json:"recordCount"`
			TotalTokens          int64   `json:"totalTokens"`
			AverageTokensPerTask float64 `json:"averageTokensPerTask"`
			MatchedBuckets       int     `json:"matchedBuckets"`
		} `json:"summary"`
		DataQuality struct {
			SessionTrackedRecords int64  `json:"sessionTrackedRecords"`
			HistoricalFallback    int64  `json:"historicalFallback"`
			TaskDefinition        string `json:"taskDefinition"`
		} `json:"dataQuality"`
		Points []analyticsPoint `json:"points"`
	}

	path := "/api/admin/usage/analytics?accountIds=" + url.QueryEscape(accountID) + "&from=2026-07-10&to=2026-07-11&groupBy=day&timezone=Asia%2FShanghai"
	var analytics analyticsResponse
	if code := c.request(http.MethodGet, path, c.adminToken, nil, &analytics); code != http.StatusOK {
		t.Fatalf("analytics status = %d", code)
	}
	if len(analytics.SelectedAccountIDs) != 1 || analytics.SelectedAccountIDs[0] != accountID {
		t.Fatalf("selected accounts = %#v", analytics.SelectedAccountIDs)
	}
	if analytics.Query.Metric != "averageTokensPerTask" || analytics.Query.GroupBy != "day" || analytics.Query.Timezone != "Asia/Shanghai" {
		t.Fatalf("analytics defaults = %#v", analytics.Query)
	}
	if len(analytics.Points) != 2 {
		t.Fatalf("analytics points = %#v", analytics.Points)
	}
	first := analytics.Points[0]
	if first.Bucket != "2026-07-10" || first.AccountID != accountID || first.TaskCount != 3 || first.RecordCount != 4 || first.CachedInputTokens != 20 || first.TotalTokens != 700 || first.HistoricalFallback != 1 {
		t.Fatalf("first analytics point = %#v", first)
	}
	if first.AverageTokensPerTask < 233.33 || first.AverageTokensPerTask > 233.34 {
		t.Fatalf("first average = %f", first.AverageTokensPerTask)
	}
	second := analytics.Points[1]
	if second.Bucket != "2026-07-11" || second.TaskCount != 1 || second.RecordCount != 1 || second.TotalTokens != 500 || second.AverageTokensPerTask != 500 {
		t.Fatalf("second analytics point = %#v", second)
	}
	if analytics.Summary.TaskCount != 4 || analytics.Summary.RecordCount != 5 || analytics.Summary.TotalTokens != 1200 || analytics.Summary.AverageTokensPerTask != 300 || analytics.Summary.MatchedBuckets != 2 {
		t.Fatalf("analytics summary = %#v", analytics.Summary)
	}
	if analytics.DataQuality.SessionTrackedRecords != 4 || analytics.DataQuality.HistoricalFallback != 1 || !strings.Contains(analytics.DataQuality.TaskDefinition, "Codex 会话") {
		t.Fatalf("analytics data quality = %#v", analytics.DataQuality)
	}

	var filtered analyticsResponse
	filterPath := path + "&minTasks=3&maxTasks=3&minTokens=700&maxTokens=700&metric=totalTokens"
	if code := c.request(http.MethodGet, filterPath, c.adminToken, nil, &filtered); code != http.StatusOK {
		t.Fatalf("filtered analytics status = %d", code)
	}
	if len(filtered.Points) != 1 || filtered.Points[0].Bucket != "2026-07-10" || filtered.Summary.TaskCount != 3 || filtered.Summary.TotalTokens != 700 || filtered.Query.Metric != "totalTokens" {
		t.Fatalf("filtered analytics = %#v", filtered)
	}

	var defaultAccount analyticsResponse
	if code := c.request(http.MethodGet, "/api/admin/usage/analytics?from=2026-07-10&to=2026-07-11", c.adminToken, nil, &defaultAccount); code != http.StatusOK {
		t.Fatalf("default account analytics status = %d", code)
	}
	if len(defaultAccount.SelectedAccountIDs) != 1 || defaultAccount.SelectedAccountIDs[0] != accountID {
		t.Fatalf("default analytics account = %#v", defaultAccount.SelectedAccountIDs)
	}
}

func TestAdminUsageAnalyticsRequiresAdminAndValidRanges(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	if code := c.request(http.MethodGet, "/api/admin/usage/analytics", "", nil, nil); code != http.StatusUnauthorized {
		t.Fatalf("unauthorized analytics status = %d", code)
	}
	for _, path := range []string{
		"/api/admin/usage/analytics?from=2026-07-12&to=2026-07-11",
		"/api/admin/usage/analytics?minTasks=2&maxTasks=1",
		"/api/admin/usage/analytics?minTokens=-1",
		"/api/admin/usage/analytics?groupBy=hour",
		"/api/admin/usage/analytics?timezone=Europe%2FLondon",
	} {
		if code := c.request(http.MethodGet, path, c.adminToken, nil, nil); code != http.StatusBadRequest {
			t.Fatalf("invalid analytics path %q status = %d", path, code)
		}
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
		`data-jump="orders" data-order-status="pending"`,
		`data-jump="recharges"`,
		`data-jump="upstreams" data-available="true"`,
		`data-jump="usage" data-today="true"`,
		"state.orderStatus = btn.dataset.orderStatus || '';",
		"state.orderPage = 1;",
		"state.rechargePage = 1;",
		"state.upstreamAvailable = btn.dataset.available;",
		"state.apiKeyPage = 1;",
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

func TestAdminAccountOrderUIUsesInlineFulfillment(t *testing.T) {
	raw, err := adminFiles.ReadFile("web/admin/index.html")
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, required := range []string{
		"账号购买申请",
		"pendingAccountOrders",
		"renderOrders",
		"/admin/account-orders?",
		"data-order-fulfilled",
		`id="orderFulfillForm"`,
		`name="userAccount"`,
		"官网只收集购买意向",
		"state.orderFulfillId",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("admin account order UI missing %q", required)
		}
	}
	for _, forbidden := range []string{"prompt(", "alert(", "confirm("} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("admin account order UI must not use native dialog %q", forbidden)
		}
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

func TestClientLaunchPrepareAllowsMatchingPoolChatGPTLogin(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("pool-login-user")
	c.loginClient("pool-login-user")
	c.approvePaidRecharge()
	var imported struct {
		Items []map[string]any `json:"items"`
	}
	if code := c.request(http.MethodPost, "/api/admin/upstreams/import", c.adminToken, map[string]any{
		"name":              "pool-login",
		"accessToken":       "pool-login-access",
		"chatgptAccountId":  "account-from-local-auth",
		"email":             "pool-login@example.com",
		"entitlementStatus": "available",
	}, &imported); code != http.StatusCreated {
		t.Fatalf("create matching pool account = %d", code)
	}
	var prepare launchPrepareResponse
	if code := c.request(http.MethodPost, "/api/client/launch/prepare", c.clientToken, map[string]any{
		"localAccount":  "POOL-LOGIN@example.com",
		"localAuthMode": "chatgpt",
	}, &prepare); code != http.StatusOK {
		t.Fatalf("prepare matching pool login = %d", code)
	}
	if prepare.Provider.BearerToken == "" {
		t.Fatalf("matching pool login did not receive provider key: %#v", prepare)
	}
}

func TestClientLaunchPrepareRejectsPersonalChatGPTLoginWithoutAssigningKey(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("personal-login-user")
	c.loginClient("personal-login-user")
	c.approvePaidRecharge()
	c.createRoutedUpstream("")
	var launchErr apiError
	if code := c.request(http.MethodPost, "/api/client/launch/prepare", c.clientToken, map[string]any{
		"localAccount":  "personal@example.com",
		"localAuthMode": "chatgpt",
	}, &launchErr); code != http.StatusConflict {
		t.Fatalf("prepare personal login status = %d", code)
	}
	if launchErr.Error != "personal_codex_login_detected" {
		t.Fatalf("prepare personal login error = %#v", launchErr)
	}
	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	for _, key := range c.store.state.APIKeys {
		if key.UserID != "" {
			t.Fatalf("personal login must not assign a pool key: %#v", key)
		}
	}
	if len(c.store.state.ClientAccessKeys) != 0 {
		t.Fatalf("personal login must not create a client access key: %#v", c.store.state.ClientAccessKeys)
	}
}

func TestClientLaunchPrepareRejectsExistingPersonalAPIKeyWithoutAssigningKey(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("personal-api-key-user")
	c.loginClient("personal-api-key-user")
	c.approvePaidRecharge()
	c.createRoutedUpstream("")
	c.store.mu.Lock()
	before := len(c.store.state.APIKeys)
	c.store.mu.Unlock()
	var launchErr apiError
	code := c.request(http.MethodPost, "/api/client/launch/prepare", c.clientToken, map[string]any{
		"localAccount":  "sk-personal...abcd",
		"localAuthMode": "api_key",
	}, &launchErr)
	if code != http.StatusConflict {
		t.Fatalf("prepare status = %d, want 409", code)
	}
	if launchErr.Error != "personal_codex_login_detected" {
		t.Fatalf("prepare error = %q", launchErr.Error)
	}
	c.store.mu.Lock()
	after := len(c.store.state.APIKeys)
	c.store.mu.Unlock()
	if after != before {
		t.Fatalf("user API key count changed from %d to %d", before, after)
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

func TestGatewayReservationEstimateLeavesUnreservedBalanceForDesktopConcurrency(t *testing.T) {
	body := []byte(`{"model":"gpt-5.4-mini","input":"short request"}`)
	reserved := gatewayRequestReservationTokens(body)
	if reserved < gatewayDefaultOutputReservation {
		t.Fatalf("reservation too small = %d", reserved)
	}
	if reserved >= 43455 {
		t.Fatalf("desktop request reserved the entire test balance = %d", reserved)
	}
	if bounded := gatewayReservationForBalance(999999, 43455); bounded != 21727 {
		t.Fatalf("large desktop request should leave balance for one concurrent call = %d", bounded)
	}
	withMaximum := gatewayRequestReservationTokens([]byte(`{"input":"x","max_output_tokens":999999}`))
	if withMaximum > gatewayMaximumRequestReservation || withMaximum <= reserved {
		t.Fatalf("bounded maximum reservation = %d base=%d", withMaximum, reserved)
	}
}

func TestGatewayCompletionChargesActualUsageDespiteAnotherReservedRequest(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("concurrent-charge")
	c.loginClient("concurrent-charge")
	c.approvePaidRecharge()

	originalRun := codexResponsesRun
	codexResponsesRun = func(context.Context, codexProbeCredentials, *gatewayRequestSource, http.Header) (codexResponsesResult, error) {
		return completedCodexTestResult("gpt-5.6-sol", "done", gatewayUsage{InputTokens: 15000, OutputTokens: 5000, TotalTokens: 20000}), nil
	}
	t.Cleanup(func() { codexResponsesRun = originalRun })
	c.createRoutedUpstream("")

	c.store.mu.Lock()
	userID := c.store.state.Users[0].ID
	now := time.Now().UTC()
	c.store.state.GatewayRequests = append(c.store.state.GatewayRequests, GatewayRequest{
		ID:             c.store.nextID("gw"),
		UserID:         userID,
		RequestID:      "concurrent-auxiliary-request",
		Status:         gatewayReserved,
		ReservedTokens: 90000,
		CreatedAt:      now,
		UpdatedAt:      now,
	})
	c.store.mu.Unlock()

	var resp struct {
		ChargedTokens int64 `json:"chargedTokens"`
	}
	if code := c.gatewayRunWithHeaders(c.clientToken, map[string]string{"Idempotency-Key": "primary-request"}, map[string]any{"model": "gpt-5.6-sol", "input": "primary"}, &resp); code != http.StatusOK {
		t.Fatalf("gateway status = %d", code)
	}
	if resp.ChargedTokens != 20000 {
		t.Fatalf("charged tokens = %d, want actual usage 20000", resp.ChargedTokens)
	}
	c.store.mu.Lock()
	balance := c.store.state.Users[0].TokenBalance
	ledger := c.store.state.TokenLedgers[len(c.store.state.TokenLedgers)-1]
	c.store.mu.Unlock()
	if balance != 80000 {
		t.Fatalf("token balance = %d, want 80000", balance)
	}
	if ledger.Source != "Codex token 用量" {
		t.Fatalf("ledger source = %q", ledger.Source)
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
		CheckStatus           string   `json:"checkStatus"`
		CheckFailureReason    string   `json:"checkFailureReason"`
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
	if checked.CheckStatus != "completed" || checked.CheckFailureReason != "" {
		t.Fatalf("probe check state = %#v", checked)
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

func TestUpstreamDailyCheckScheduleUsesBeijingNineAM(t *testing.T) {
	before := time.Date(2026, 7, 14, 8, 30, 0, 0, upstreamCheckLocation)
	if got := nextUpstreamDailyCheck(before); !got.Equal(time.Date(2026, 7, 14, 9, 0, 0, 0, upstreamCheckLocation)) {
		t.Fatalf("next check before 09:00 = %s", got)
	}
	after := time.Date(2026, 7, 14, 9, 0, 1, 0, upstreamCheckLocation)
	if got := nextUpstreamDailyCheck(after); !got.Equal(time.Date(2026, 7, 15, 9, 0, 0, 0, upstreamCheckLocation)) {
		t.Fatalf("next check after 09:00 = %s", got)
	}
	utc := time.Date(2026, 7, 14, 0, 30, 0, 0, time.UTC)
	if got := upstreamDailyCheckBoundary(utc); got.Hour() != 9 || got.Location().String() != "Asia/Shanghai" {
		t.Fatalf("Beijing boundary = %s (%s)", got, got.Location())
	}
}

func TestScheduledUpstreamCheckCatchesUpAuthorizedAccountsOnce(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	var authorized struct {
		ID string `json:"id"`
	}
	if code := c.request(http.MethodPost, "/api/admin/upstreams", c.adminToken, map[string]any{
		"name": "scheduled-account", "credentialType": "oauth", "accessToken": "scheduled-access", "chatgptAccountId": "scheduled-account-id",
	}, &authorized); code != http.StatusCreated {
		t.Fatalf("create scheduled upstream = %d", code)
	}
	if code := c.request(http.MethodPost, "/api/admin/upstreams/import", c.adminToken, map[string]any{
		"email": "pending-schedule@example.com", "password": "pending-secret",
	}, nil); code != http.StatusCreated {
		t.Fatalf("create pending upstream = %d", code)
	}

	probeCalls := 0
	originalProbe := codexAppServerProbe
	codexAppServerProbe = func(ctx context.Context, credentials codexProbeCredentials) (codexProbeResult, error) {
		probeCalls++
		if credentials.ChatGPTAccountID != "scheduled-account-id" {
			t.Fatalf("scheduled credentials = %#v", credentials)
		}
		return codexProbeResult{AccountType: "chatgpt", PlanType: "plus"}, nil
	}
	t.Cleanup(func() { codexAppServerProbe = originalProbe })

	due := time.Now().UTC().Add(-time.Minute)
	c.app.runScheduledUpstreamChecks(context.Background(), due, true)
	if probeCalls != 1 {
		t.Fatalf("scheduled probe calls = %d", probeCalls)
	}
	c.app.runScheduledUpstreamChecks(context.Background(), due, true)
	if probeCalls != 1 {
		t.Fatalf("catch-up repeated a completed check: calls=%d", probeCalls)
	}

	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	up := c.app.upstreamByID(authorized.ID)
	if up.LastCheckedAt == nil || up.LastCheckedAt.Before(due) || up.EntitlementStatus != "available" {
		t.Fatalf("scheduled upstream state = %#v", up)
	}
	foundAudit := false
	for _, item := range c.store.state.AuditLogs {
		if item.Action == "upstream.check.scheduled" && item.ActorRole == "system" && item.TargetID == authorized.ID {
			foundAudit = true
		}
	}
	if !foundAudit {
		t.Fatalf("scheduled audit missing: %#v", c.store.state.AuditLogs)
	}
}

func TestAdminCanEditAndClearUpstreamRemark(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	var upstream struct {
		ID string `json:"id"`
	}
	if code := c.request(http.MethodPost, "/api/admin/upstreams", c.adminToken, map[string]any{
		"name": "remark-account", "remark": "首批账号", "credentialType": "oauth", "accessToken": "remark-access", "chatgptAccountId": "remark-account-id",
	}, &upstream); code != http.StatusCreated {
		t.Fatalf("create upstream with remark = %d", code)
	}
	var updated map[string]any
	if code := c.request(http.MethodPost, "/api/admin/upstreams/"+upstream.ID+"/remark", c.adminToken, map[string]any{"remark": " 新加坡节点 / 购买批次 A "}, &updated); code != http.StatusOK {
		t.Fatalf("update remark = %d", code)
	}
	if updated["remark"] != "新加坡节点 / 购买批次 A" {
		t.Fatalf("updated remark = %#v", updated)
	}
	var listed struct {
		Items []map[string]any `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/admin/upstreams", c.adminToken, nil, &listed); code != http.StatusOK {
		t.Fatalf("list upstream remarks = %d", code)
	}
	if len(listed.Items) != 1 || listed.Items[0]["remark"] != "新加坡节点 / 购买批次 A" {
		t.Fatalf("listed remark = %#v", listed.Items)
	}
	if code := c.request(http.MethodPost, "/api/admin/upstreams/"+upstream.ID+"/remark", c.adminToken, map[string]any{"remark": strings.Repeat("备", maxUpstreamRemarkRunes+1)}, nil); code != http.StatusBadRequest {
		t.Fatalf("oversized remark = %d", code)
	}
	if code := c.request(http.MethodPost, "/api/admin/upstreams/"+upstream.ID+"/remark", c.adminToken, map[string]any{"remark": "", "unexpected": true}, nil); code != http.StatusBadRequest {
		t.Fatalf("remark unknown field = %d", code)
	}
	if code := c.request(http.MethodPost, "/api/admin/upstreams/"+upstream.ID+"/remark", c.adminToken, map[string]any{"remark": ""}, &updated); code != http.StatusOK || updated["remark"] != "" {
		t.Fatalf("clear remark = code:%d payload:%#v", code, updated)
	}
	for _, item := range c.store.state.AuditLogs {
		if item.Action == "upstream.remark" && strings.Contains(item.Detail, "新加坡节点") {
			t.Fatalf("remark content leaked into audit: %#v", item)
		}
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
	staleUsedPercent := 35.0
	staleCredit := 7.5
	staleReset := time.Now().UTC().Add(time.Hour)
	c.store.mu.Lock()
	idx := c.app.upstreamIndex(upstream.ID)
	c.store.state.UpstreamAccounts[idx].UsageTokens = 999
	c.store.state.UpstreamAccounts[idx].RateLimitUsedPercent = &staleUsedPercent
	c.store.state.UpstreamAccounts[idx].RateLimitResetsAt = &staleReset
	c.store.state.UpstreamAccounts[idx].CreditBalance = &staleCredit
	c.store.state.UpstreamAccounts[idx].CreditBalanceLabel = "7.5"
	c.store.mu.Unlock()

	var checked struct {
		EntitlementStatus  string     `json:"entitlementStatus"`
		AvailabilityStatus string     `json:"availabilityStatus"`
		CheckStatus        string     `json:"checkStatus"`
		CheckFailureReason string     `json:"checkFailureReason"`
		LastCheckedAt      *time.Time `json:"lastCheckedAt"`
		UsageTokens        int64      `json:"usageTokens"`
		RateLimitUsed      *float64   `json:"rateLimitUsedPercent"`
		CreditBalance      *float64   `json:"creditBalance"`
	}
	if code := c.request(http.MethodPost, "/api/admin/upstreams/"+upstream.ID+"/check", c.adminToken, nil, &checked); code != http.StatusOK {
		t.Fatalf("check status = %d", code)
	}
	if checked.EntitlementStatus != "check_failed" || checked.AvailabilityStatus != "unavailable" || checked.CheckStatus != "failed" || checked.CheckFailureReason != "check_failed" {
		t.Fatalf("checked failure status = %#v", checked)
	}
	if checked.LastCheckedAt == nil || checked.UsageTokens != 0 || checked.RateLimitUsed != nil || checked.CreditBalance != nil {
		t.Fatalf("failed check kept stale quota data = %#v", checked)
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
		CheckStatus        string `json:"checkStatus"`
		CheckFailureReason string `json:"checkFailureReason"`
	}
	if code := c.request(http.MethodPost, "/api/admin/upstreams/"+upstream.ID+"/check", c.adminToken, nil, &checked); code != http.StatusOK {
		t.Fatalf("check status = %d", code)
	}
	if checked.EntitlementStatus != "auth_failed" || checked.AvailabilityStatus != "unavailable" || checked.CheckStatus != "failed" || checked.CheckFailureReason != "auth_failed" {
		t.Fatalf("checked auth failure = %#v", checked)
	}
}

func TestAdminUpstreamCheckStatusIsVisibleWhileProbeRuns(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	started := make(chan struct{})
	release := make(chan struct{})
	originalProbe := codexAppServerProbe
	codexAppServerProbe = func(ctx context.Context, credentials codexProbeCredentials) (codexProbeResult, error) {
		close(started)
		select {
		case <-release:
			return codexProbeResult{}, errors.New("codex_app_server_http_401")
		case <-ctx.Done():
			return codexProbeResult{}, ctx.Err()
		}
	}
	t.Cleanup(func() { codexAppServerProbe = originalProbe })

	var upstream struct {
		ID string `json:"id"`
	}
	if code := c.request(http.MethodPost, "/api/admin/upstreams", c.adminToken, map[string]any{"name": "checking-upstream", "credentialType": "oauth", "accessToken": "access", "chatgptAccountId": "account-123"}, &upstream); code != http.StatusCreated {
		t.Fatalf("upstream status = %d", code)
	}
	type checkOutcome struct {
		Payload map[string]any
		Err     *upstreamCheckError
	}
	done := make(chan checkOutcome, 1)
	go func() {
		payload, checkErr := c.app.checkUpstream(context.Background(), upstream.ID, "admin-test", "admin", "upstream.check")
		done <- checkOutcome{Payload: payload, Err: checkErr}
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("upstream probe did not start")
	}

	c.store.mu.Lock()
	checking := c.app.publicAdminUpstreamLocked(c.app.upstreamByID(upstream.ID))
	c.store.mu.Unlock()
	if checking["checkStatus"] != "checking" {
		t.Fatalf("in-progress check state = %#v", checking)
	}
	if _, exists := checking["checkFailureReason"]; exists {
		t.Fatalf("stale failure reason exposed while checking: %#v", checking)
	}

	close(release)
	select {
	case outcome := <-done:
		if outcome.Err != nil {
			t.Fatalf("check failed = %v", outcome.Err)
		}
		if outcome.Payload["checkStatus"] != "failed" || outcome.Payload["checkFailureReason"] != "auth_failed" {
			t.Fatalf("completed check state = %#v", outcome.Payload)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("upstream probe did not finish")
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
	if preview, ok := key["keyPreview"].(string); !ok || !strings.HasSuffix(preview, "...") {
		t.Fatalf("api key preview missing: %#v", key)
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
	if preview, ok := keys.Items[0]["keyPreview"].(string); !ok || !strings.HasSuffix(preview, "...") {
		t.Fatalf("api key list preview missing: %#v", keys.Items[0])
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
	if code := c.request(http.MethodDelete, "/api/admin/api-keys/"+keyID, c.adminToken, nil, nil); code != http.StatusOK {
		t.Fatalf("delete api key = %d", code)
	}
	var keysAfterDelete struct {
		Items []map[string]any `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/admin/api-keys", c.adminToken, nil, &keysAfterDelete); code != http.StatusOK {
		t.Fatalf("list api keys after delete = %d", code)
	}
	if len(keysAfterDelete.Items) != 0 {
		t.Fatalf("api keys after delete = %#v", keysAfterDelete.Items)
	}
	var upstreamsAfterDelete struct {
		Items []map[string]any `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/admin/upstreams", c.adminToken, nil, &upstreamsAfterDelete); code != http.StatusOK {
		t.Fatalf("list upstreams after delete = %d", code)
	}
	if len(upstreamsAfterDelete.Items) != 0 {
		t.Fatalf("orphan upstreams after api key delete = %#v", upstreamsAfterDelete.Items)
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
		APIKeys  []map[string]any `json:"apiKeys"`
	}
	if code := c.request(http.MethodPost, "/api/admin/upstreams/import", c.adminToken, body, &imported); code != http.StatusCreated {
		t.Fatalf("import upstream status = %d", code)
	}
	if imported.Imported != 1 || len(imported.Items) != 1 {
		t.Fatalf("imported upstreams = %#v", imported)
	}
	if len(imported.APIKeys) != 1 || imported.APIKeys[0]["upstream"] == nil {
		t.Fatalf("imported api keys = %#v", imported.APIKeys)
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
	if len(c.store.state.APIKeys) != 1 || c.store.state.APIKeys[0].UpstreamAccountID != c.store.state.UpstreamAccounts[0].ID {
		t.Fatalf("stored api keys = %#v", c.store.state.APIKeys)
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

func TestAdminImportChatGPTSessionJSONCreatesShortLivedPoolAccount(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	expiresAt := time.Now().UTC().Add(2 * time.Hour).Truncate(time.Second)
	accessToken := testJWT(t, map[string]any{
		"exp": expiresAt.Unix(),
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": "session-account",
			"chatgpt_plan_type":  "plus",
		},
		"https://api.openai.com/profile": map[string]any{"email": "session@example.com"},
	})
	sessionRaw, err := json.Marshal(map[string]any{
		"user": map[string]any{
			"id":    "session-user",
			"name":  "Session User",
			"email": "session@example.com",
			"image": "private-avatar-value",
		},
		"account": map[string]any{
			"id":       "session-account",
			"planType": "plus",
		},
		"expires":     expiresAt.Add(24 * time.Hour).Format(time.RFC3339),
		"accessToken": accessToken,
	})
	if err != nil {
		t.Fatal(err)
	}
	var imported struct {
		Imported int              `json:"imported"`
		Updated  int              `json:"updated"`
		Items    []map[string]any `json:"items"`
		APIKeys  []map[string]any `json:"apiKeys"`
	}
	if code := c.request(http.MethodPost, "/api/admin/upstreams/import", c.adminToken, map[string]any{"content": string(sessionRaw)}, &imported); code != http.StatusCreated {
		t.Fatalf("import ChatGPT session status = %d", code)
	}
	if imported.Imported != 1 || imported.Updated != 0 || len(imported.Items) != 1 || len(imported.APIKeys) != 1 {
		t.Fatalf("import ChatGPT session result = %#v", imported)
	}
	item := imported.Items[0]
	if item["sourceType"] != "chatgpt_session" || item["credentialType"] != "oauth" || item["credentialStatus"] != "short_lived_auth" || item["chatgptAccountId"] != "session-account" || item["email"] != "session@example.com" || item["subscriptionTier"] != "plus" {
		t.Fatalf("imported ChatGPT session metadata = %#v", item)
	}
	responseRaw, err := json.Marshal(imported)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(responseRaw), accessToken) || strings.Contains(string(responseRaw), "private-avatar-value") {
		t.Fatalf("ChatGPT session import response leaked credentials: %s", responseRaw)
	}

	c.store.mu.Lock()
	if len(c.store.state.UpstreamAccounts) != 1 || len(c.store.state.APIKeys) != 1 {
		c.store.mu.Unlock()
		t.Fatalf("stored ChatGPT session pool state = upstreams:%d keys:%d", len(c.store.state.UpstreamAccounts), len(c.store.state.APIKeys))
	}
	up := c.store.state.UpstreamAccounts[0]
	c.store.mu.Unlock()
	if up.ExpiresAt == nil || !up.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("stored ChatGPT access-token expiry = %#v, want %s", up.ExpiresAt, expiresAt.Format(time.RFC3339))
	}
	if up.RefreshTokenCipher != "" || up.AuthJSONCipher == "" {
		t.Fatalf("stored ChatGPT session credential shape = %#v", up)
	}
	storedAccessToken, err := c.app.decrypt(up.AccessTokenCipher)
	if err != nil || storedAccessToken != accessToken {
		t.Fatalf("stored ChatGPT access token mismatch: token=%q err=%v", storedAccessToken, err)
	}
	storedAuthJSON, err := c.app.decrypt(up.AuthJSONCipher)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(storedAuthJSON, "private-avatar-value") || strings.Contains(storedAuthJSON, `"user"`) || strings.Contains(storedAuthJSON, `"account"`) {
		t.Fatalf("stored ChatGPT auth snapshot kept unrelated session profile data: %s", storedAuthJSON)
	}
	var authSnapshot map[string]any
	if err := json.Unmarshal([]byte(storedAuthJSON), &authSnapshot); err != nil {
		t.Fatal(err)
	}
	tokens, _ := authSnapshot["tokens"].(map[string]any)
	if authSnapshot["auth_mode"] != "chatgpt" || tokens["access_token"] != accessToken || tokens["account_id"] != "session-account" || tokens["refresh_token"] != "" {
		t.Fatalf("stored minimal ChatGPT auth snapshot = %#v", authSnapshot)
	}

	newExpiry := expiresAt.Add(90 * time.Minute)
	newAccessToken := testJWT(t, map[string]any{
		"exp":                         newExpiry.Unix(),
		"https://api.openai.com/auth": map[string]any{"chatgpt_account_id": "session-account", "chatgpt_plan_type": "plus"},
	})
	var renewedSession map[string]any
	if err := json.Unmarshal(sessionRaw, &renewedSession); err != nil {
		t.Fatal(err)
	}
	renewedSession["accessToken"] = newAccessToken
	renewedSession["expires"] = newExpiry.Add(24 * time.Hour).Format(time.RFC3339)
	if code := c.request(http.MethodPost, "/api/admin/upstreams/import", c.adminToken, renewedSession, &imported); code != http.StatusCreated {
		t.Fatalf("renew ChatGPT session status = %d", code)
	}
	if imported.Imported != 0 || imported.Updated != 1 || len(imported.Items) != 1 || len(imported.APIKeys) != 1 {
		t.Fatalf("renew ChatGPT session result = %#v", imported)
	}
	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	if len(c.store.state.UpstreamAccounts) != 1 || len(c.store.state.APIKeys) != 1 || c.store.state.UpstreamAccounts[0].ExpiresAt == nil || !c.store.state.UpstreamAccounts[0].ExpiresAt.Equal(newExpiry) {
		t.Fatalf("renewed ChatGPT session pool state = upstreams:%#v keys:%#v", c.store.state.UpstreamAccounts, c.store.state.APIKeys)
	}
}

func TestAdminExportUpstreamsRoundTripsCredentialsWithoutInternalKeys(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	if code := c.request(http.MethodPost, "/api/admin/upstreams/import", c.adminToken, map[string]any{"email": "pending-export@example.com", "password": "pending-export-secret", "remark": "待授权备注"}, nil); code != http.StatusCreated {
		t.Fatalf("import pending export account = %d", code)
	}
	authJSON := map[string]any{
		"auth_mode": "chatgpt",
		"tokens": map[string]any{
			"access_token":  "export-access-secret",
			"refresh_token": "export-refresh-secret",
			"account_id":    "export-account",
		},
		"email":  "authorized-export@example.com",
		"remark": "正式账号备注",
	}
	if code := c.request(http.MethodPost, "/api/admin/upstreams/import", c.adminToken, authJSON, nil); code != http.StatusCreated {
		t.Fatalf("import authorized export account = %d", code)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/admin/upstreams/export", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.adminToken)
	rr := httptest.NewRecorder()
	c.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("export account pool status = %d body=%s", rr.Code, rr.Body.String())
	}
	if disposition := rr.Header().Get("Content-Disposition"); !strings.Contains(disposition, "attachment") || !strings.Contains(disposition, "codexppp-account-pool-") {
		t.Fatalf("export content disposition = %q", disposition)
	}
	var exported map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &exported); err != nil {
		t.Fatal(err)
	}
	accounts, _ := exported["accounts"].([]any)
	if exported["format"] != "codexppp-account-pool" || len(accounts) != 2 {
		t.Fatalf("exported account pool = %#v", exported)
	}
	exportText := rr.Body.String()
	for _, secret := range []string{"pending-export-secret", "export-access-secret", "export-refresh-secret"} {
		if !strings.Contains(exportText, secret) {
			t.Fatalf("account export lost credential %q: %s", secret, exportText)
		}
	}
	for _, forbidden := range []string{"keyCipher", "keyHash", "publicPrefix", "credentialFingerprint"} {
		if strings.Contains(exportText, forbidden) {
			t.Fatalf("account export leaked internal key field %q: %s", forbidden, exportText)
		}
	}

	restored := newTestClient(t)
	restored.setupAdmin()
	var imported struct {
		Imported int              `json:"imported"`
		Pending  int              `json:"pending"`
		Items    []map[string]any `json:"items"`
		APIKeys  []map[string]any `json:"apiKeys"`
	}
	if code := restored.request(http.MethodPost, "/api/admin/upstreams/import", restored.adminToken, exported, &imported); code != http.StatusCreated {
		t.Fatalf("restore exported account pool = %d", code)
	}
	if imported.Imported != 2 || imported.Pending != 1 || len(imported.Items) != 2 || len(imported.APIKeys) != 1 {
		t.Fatalf("restored account pool = %#v", imported)
	}
	restored.store.mu.Lock()
	defer restored.store.mu.Unlock()
	if len(restored.store.state.UpstreamAccounts) != 2 || len(restored.store.state.APIKeys) != 1 {
		t.Fatalf("restored pool storage = upstreams:%#v keys:%#v", restored.store.state.UpstreamAccounts, restored.store.state.APIKeys)
	}
	remarks := map[string]bool{}
	for _, upstream := range restored.store.state.UpstreamAccounts {
		remarks[upstream.Remark] = true
	}
	if !remarks["待授权备注"] || !remarks["正式账号备注"] {
		t.Fatalf("restored pool lost remarks: %#v", restored.store.state.UpstreamAccounts)
	}
}

func TestAdminUpstreamsPrioritizesCurrentUserUsageAndRemaining(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("pool-user")
	var created map[string]any
	if code := c.request(http.MethodPost, "/api/admin/upstreams/import", c.adminToken, map[string]any{
		"name":              "usage-account",
		"accessToken":       "usage-access",
		"chatgptAccountId":  "usage-account-id",
		"subscriptionTier":  "plus",
		"entitlementStatus": "available",
	}, &created); code != http.StatusCreated {
		t.Fatalf("create usage upstream = %d", code)
	}
	items, _ := created["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("created usage upstream response = %#v", created)
	}
	upstreamID, _ := items[0].(map[string]any)["id"].(string)
	c.store.mu.Lock()
	user := c.store.state.Users[0]
	now := time.Now().UTC()
	remaining := float64(37.5)
	idx := c.app.upstreamIndex(upstreamID)
	c.store.state.UpstreamAccounts[idx].RateLimitUsedPercent = &remaining
	c.store.state.UpstreamAccounts[idx].CreditBalanceLabel = "8.25"
	c.store.state.UsageRecords = append(c.store.state.UsageRecords,
		UsageRecord{ID: c.store.nextID("use"), UserID: user.ID, UpstreamAccountID: upstreamID, SessionID: "session-1", Model: "gpt-5.5", TotalTokens: 12, CreatedAt: now.Add(-time.Minute)},
		UsageRecord{ID: c.store.nextID("use"), UserID: user.ID, UpstreamAccountID: upstreamID, SessionID: "session-1", Model: "gpt-5.5", TotalTokens: 30, CreatedAt: now},
	)
	c.store.mu.Unlock()
	c.app.setGatewayRouteActive(upstreamID, user.ID, true)
	t.Cleanup(func() { c.app.setGatewayRouteActive(upstreamID, user.ID, false) })
	var upstreams struct {
		Items []map[string]any `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/admin/upstreams?authorization=authorized", c.adminToken, nil, &upstreams); code != http.StatusOK {
		t.Fatalf("list usage-priority upstreams = %d", code)
	}
	if len(upstreams.Items) != 1 {
		t.Fatalf("usage-priority upstreams = %#v", upstreams.Items)
	}
	item := upstreams.Items[0]
	active, _ := item["activeUserAccounts"].([]any)
	if len(active) != 1 || active[0] != "pool-user" || item["activeUserCount"] != float64(1) || item["activeUserLimit"] != float64(defaultGatewayUpstreamUserLimit) || item["lastUserAccount"] != "pool-user" || item["routedUsageTokens"] != float64(42) || item["rateLimitRemainingPercent"] != float64(62.5) || item["creditBalanceLabel"] != "8.25" {
		t.Fatalf("usage-priority upstream item = %#v", item)
	}
	lastUsedFrom, _ := item["lastUsedFrom"].(string)
	lastUsedAt, _ := item["lastUsedAt"].(string)
	if lastUsedFrom == "" || lastUsedAt == "" || lastUsedFrom == lastUsedAt {
		t.Fatalf("latest session usage period missing: %#v", item)
	}
}

func TestClientLaunchHeartbeatKeepsGatewayAssignedUserActiveUntilStop(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("heartbeat-user")
	c.loginClient("heartbeat-user")
	c.approvePaidRecharge()
	upstreamID := c.createRoutedUpstream("")
	c.store.mu.Lock()
	user := c.store.state.Users[0]
	c.store.state.UsageRecords = append(c.store.state.UsageRecords, UsageRecord{
		ID: c.store.nextID("use"), UserID: user.ID, UpstreamAccountID: upstreamID,
		Model: "gpt-5.4-mini", TotalTokens: 9, CreatedAt: time.Now().UTC(),
	})
	c.store.mu.Unlock()
	var heartbeat map[string]any
	if code := c.request(http.MethodPost, "/api/client/launch/heartbeat", c.clientToken, map[string]any{}, &heartbeat); code != http.StatusOK || heartbeat["active"] != true {
		t.Fatalf("launch heartbeat = code:%d payload:%#v", code, heartbeat)
	}
	var upstreams struct {
		Items []map[string]any `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/admin/upstreams", c.adminToken, nil, &upstreams); code != http.StatusOK {
		t.Fatalf("list upstreams after heartbeat = %d", code)
	}
	active, _ := upstreams.Items[0]["activeUserAccounts"].([]any)
	if len(active) != 1 || active[0] != "heartbeat-user" {
		t.Fatalf("heartbeat active users = %#v", upstreams.Items[0])
	}
	if code := c.request(http.MethodPost, "/api/client/launch/stop", c.clientToken, map[string]any{}, nil); code != http.StatusOK {
		t.Fatalf("launch stop = %d", code)
	}
	upstreams.Items = nil
	if code := c.request(http.MethodGet, "/api/admin/upstreams", c.adminToken, nil, &upstreams); code != http.StatusOK {
		t.Fatalf("list upstreams after stop = %d", code)
	}
	active, _ = upstreams.Items[0]["activeUserAccounts"].([]any)
	if len(active) != 0 || upstreams.Items[0]["lastUserAccount"] != "heartbeat-user" {
		t.Fatalf("stopped heartbeat usage = %#v", upstreams.Items[0])
	}
}

func TestClientPresenceSeparatesLoginFromManagedCodexRuntime(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("online-user")
	c.loginClient("online-user")

	var users struct {
		Items []map[string]any `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/admin/users", c.adminToken, nil, &users); code != http.StatusOK {
		t.Fatalf("list users after login = %d", code)
	}
	if len(users.Items) != 1 || users.Items[0]["clientOnline"] != true || users.Items[0]["clientActive"] != false || users.Items[0]["gatewayActive"] != false {
		t.Fatalf("logged-in client presence = %#v", users.Items)
	}

	if code := c.request(http.MethodPost, "/api/client/presence/stop", c.clientToken, map[string]any{}, nil); code != http.StatusOK {
		t.Fatalf("presence stop = %d", code)
	}
	users.Items = nil
	if code := c.request(http.MethodGet, "/api/admin/users", c.adminToken, nil, &users); code != http.StatusOK || users.Items[0]["clientOnline"] != false {
		t.Fatalf("offline client presence = code:%d users:%#v", code, users.Items)
	}

	var heartbeat map[string]any
	if code := c.request(http.MethodPost, "/api/client/presence/heartbeat", c.clientToken, map[string]any{}, &heartbeat); code != http.StatusOK || heartbeat["online"] != true {
		t.Fatalf("presence heartbeat = code:%d payload:%#v", code, heartbeat)
	}
	c.store.mu.Lock()
	for userID, devices := range c.app.clientPresence {
		for deviceID := range devices {
			devices[deviceID] = time.Now().UTC().Add(-time.Second)
		}
		c.app.clientPresence[userID] = devices
	}
	c.store.mu.Unlock()
	users.Items = nil
	if code := c.request(http.MethodGet, "/api/admin/users", c.adminToken, nil, &users); code != http.StatusOK || users.Items[0]["clientOnline"] != false {
		t.Fatalf("expired client presence = code:%d users:%#v", code, users.Items)
	}
}

func TestAdminUserReflectsGatewayAssignmentShownByAccountPool(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("assigned-user")
	c.loginClient("assigned-user")
	c.approvePaidRecharge()
	upstreamID := c.createRoutedUpstream("")
	c.store.mu.Lock()
	userID := c.store.state.Users[0].ID
	c.store.mu.Unlock()
	if !c.app.acquireGatewayUserSlot(upstreamID, userID, false) {
		t.Fatal("gateway assignment failed")
	}

	var users struct {
		Items []map[string]any `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/admin/users", c.adminToken, nil, &users); code != http.StatusOK {
		t.Fatalf("list assigned user = %d", code)
	}
	assigned, _ := users.Items[0]["assignedUpstreamAccount"].(map[string]any)
	if users.Items[0]["gatewayActive"] != true || assigned["id"] != upstreamID {
		t.Fatalf("assigned user state = %#v", users.Items[0])
	}

	var upstreams struct {
		Items                []map[string]any `json:"items"`
		OnlineClientAccounts []string         `json:"onlineClientAccounts"`
	}
	if code := c.request(http.MethodGet, "/api/admin/upstreams", c.adminToken, nil, &upstreams); code != http.StatusOK {
		t.Fatalf("list assigned upstream = %d", code)
	}
	active, _ := upstreams.Items[0]["activeUserAccounts"].([]any)
	if len(active) != 1 || active[0] != "assigned-user" || len(upstreams.OnlineClientAccounts) != 1 || upstreams.OnlineClientAccounts[0] != "assigned-user" {
		t.Fatalf("assigned pool state = %#v", upstreams)
	}
}

func TestClientLaunchHeartbeatShowsActiveBeforeFirstRouteAssignment(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("waiting-user")
	c.loginClient("waiting-user")
	c.approvePaidRecharge()
	c.createRoutedUpstream("")

	var heartbeat map[string]any
	if code := c.request(http.MethodPost, "/api/client/launch/heartbeat", c.clientToken, map[string]any{
		"desktopVersion": "0.1.48",
		"codexVersion":   "26.707.71524.0",
	}, &heartbeat); code != http.StatusOK || heartbeat["active"] != true || heartbeat["assigned"] != false {
		t.Fatalf("unassigned launch heartbeat = code:%d payload:%#v", code, heartbeat)
	}

	var users struct {
		Items []map[string]any `json:"items"`
	}
	if code := c.request(http.MethodGet, "/api/admin/users", c.adminToken, nil, &users); code != http.StatusOK {
		t.Fatalf("list users after heartbeat = %d", code)
	}
	if len(users.Items) != 1 || users.Items[0]["clientOnline"] != true || users.Items[0]["clientActive"] != true || users.Items[0]["desktopVersion"] != "0.1.48" || users.Items[0]["codexVersion"] != "26.707.71524.0" {
		t.Fatalf("active client user = %#v", users.Items)
	}

	var upstreams struct {
		Items                    []map[string]any `json:"items"`
		ActiveClientAccounts     []string         `json:"activeClientAccounts"`
		UnassignedClientAccounts []string         `json:"unassignedClientAccounts"`
	}
	if code := c.request(http.MethodGet, "/api/admin/upstreams", c.adminToken, nil, &upstreams); code != http.StatusOK {
		t.Fatalf("list upstreams with waiting client = %d", code)
	}
	if len(upstreams.ActiveClientAccounts) != 1 || upstreams.ActiveClientAccounts[0] != "waiting-user" || len(upstreams.UnassignedClientAccounts) != 1 || upstreams.UnassignedClientAccounts[0] != "waiting-user" {
		t.Fatalf("waiting client summary = %#v", upstreams)
	}
	active, _ := upstreams.Items[0]["activeUserAccounts"].([]any)
	if len(active) != 0 {
		t.Fatalf("waiting client must not be attached to a route before a task: %#v", upstreams.Items[0])
	}

	if code := c.request(http.MethodPost, "/api/client/launch/stop", c.clientToken, map[string]any{}, nil); code != http.StatusOK {
		t.Fatalf("stop waiting client = %d", code)
	}
	users.Items = nil
	if code := c.request(http.MethodGet, "/api/admin/users", c.adminToken, nil, &users); code != http.StatusOK || users.Items[0]["clientActive"] != false {
		t.Fatalf("stopped waiting client = code:%d users:%#v", code, users.Items)
	}
}

func TestManagedClientAccessKeyRequiresItsDeviceRuntimeLease(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("managed-device-user")
	c.loginClient("managed-device-user")
	c.approvePaidRecharge()
	c.createRoutedUpstream("")
	providerToken := c.prepareCodexProviderToken()

	var models map[string]any
	if code := c.request(http.MethodGet, "/api/codex/v1/models", providerToken, nil, &models); code != http.StatusOK {
		t.Fatalf("managed provider models status = %d payload=%#v", code, models)
	}
	c.store.mu.Lock()
	for userID, devices := range c.app.clientRuntimes {
		for deviceID, runtime := range devices {
			runtime.ExpiresAt = time.Now().UTC().Add(-time.Second)
			devices[deviceID] = runtime
		}
		c.app.clientRuntimes[userID] = devices
	}
	c.store.mu.Unlock()
	models = nil
	if code := c.request(http.MethodGet, "/api/codex/v1/models", providerToken, nil, &models); code != http.StatusUnauthorized {
		t.Fatalf("expired managed provider status = %d payload=%#v", code, models)
	}
}

func TestStoppingOneManagedDeviceKeepsAnotherDeviceAndUserAssignmentActive(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("multi-device-user")
	c.loginClient("multi-device-user")
	c.approvePaidRecharge()
	upstreamID := c.createRoutedUpstream("")

	prepare := func(token, fingerprint string) string {
		t.Helper()
		if code := c.request(http.MethodPost, "/api/client/launch/prepare", token, map[string]any{"managedRuntime": true}, nil); code != http.StatusOK {
			t.Fatalf("prepare %s = %d", fingerprint, code)
		}
		return token
	}
	firstToken := prepare(c.clientToken, "managed-device-1")
	var secondLogin struct {
		Token string `json:"token"`
	}
	secondBody := map[string]any{"account": "multi-device-user", "password": "", "deviceName": "managed-device-2", "fingerprint": "managed-device-2"}
	if code := c.request(http.MethodPost, "/api/client/login", "", secondBody, &secondLogin); code != http.StatusOK {
		t.Fatalf("login second device = %d", code)
	}
	secondToken := prepare(secondLogin.Token, "managed-device-2")

	c.store.mu.Lock()
	userID := c.store.state.Users[0].ID
	c.store.mu.Unlock()
	if !c.app.acquireGatewayUserSlot(upstreamID, userID, false) {
		t.Fatal("initial user assignment failed")
	}
	if code := c.request(http.MethodPost, "/api/client/launch/stop", firstToken, map[string]any{}, nil); code != http.StatusOK {
		t.Fatalf("stop first managed device = %d", code)
	}
	c.store.mu.Lock()
	_, runtimeActive := c.app.clientRuntimeSnapshotLocked(userID, time.Now().UTC())
	assigned := c.app.gatewayUserAssignedUpstreamLocked(userID, time.Now().UTC())
	c.store.mu.Unlock()
	if !runtimeActive || assigned != upstreamID {
		t.Fatalf("second device lost management: active=%v assigned=%q", runtimeActive, assigned)
	}
	if code := c.request(http.MethodPost, "/api/client/launch/stop", secondToken, map[string]any{}, nil); code != http.StatusOK {
		t.Fatalf("stop second managed device = %d", code)
	}
	c.store.mu.Lock()
	_, runtimeActive = c.app.clientRuntimeSnapshotLocked(userID, time.Now().UTC())
	assigned = c.app.gatewayUserAssignedUpstreamLocked(userID, time.Now().UTC())
	c.store.mu.Unlock()
	if runtimeActive || assigned != "" {
		t.Fatalf("final device stop left management state: active=%v assigned=%q", runtimeActive, assigned)
	}
}

func TestHeartbeatDoesNotRetainAnUnavailableUpstreamAssignment(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("unavailable-heartbeat-user")
	c.loginClient("unavailable-heartbeat-user")
	c.approvePaidRecharge()
	upstreamID := c.createRoutedUpstream("")
	c.store.mu.Lock()
	userID := c.store.state.Users[0].ID
	c.store.state.UsageRecords = append(c.store.state.UsageRecords, UsageRecord{
		ID: c.store.nextID("use"), UserID: userID, UpstreamAccountID: upstreamID,
		Model: "gpt-5.4-mini", TotalTokens: 9, CreatedAt: time.Now().UTC(),
	})
	c.store.mu.Unlock()
	var heartbeat map[string]any
	if code := c.request(http.MethodPost, "/api/client/launch/heartbeat", c.clientToken, map[string]any{}, &heartbeat); code != http.StatusOK || heartbeat["assigned"] != true {
		t.Fatalf("initial heartbeat = code:%d payload:%#v", code, heartbeat)
	}
	c.store.mu.Lock()
	idx := c.app.upstreamIndex(upstreamID)
	c.store.state.UpstreamAccounts[idx].Status = statusDisabled
	c.store.mu.Unlock()
	heartbeat = nil
	if code := c.request(http.MethodPost, "/api/client/launch/heartbeat", c.clientToken, map[string]any{}, &heartbeat); code != http.StatusOK || heartbeat["assigned"] != false {
		t.Fatalf("unavailable heartbeat = code:%d payload:%#v", code, heartbeat)
	}
	c.store.mu.Lock()
	assigned := c.app.gatewayUserAssignedUpstreamLocked(userID, time.Now().UTC())
	c.store.mu.Unlock()
	if assigned != "" {
		t.Fatalf("unavailable upstream assignment retained as %q", assigned)
	}
}

func TestGatewayUpstreamUserLimitKeepsAssignmentsStable(t *testing.T) {
	if limit, err := gatewayUpstreamUserLimitFromEnv(""); err != nil || limit != defaultGatewayUpstreamUserLimit {
		t.Fatalf("default upstream user limit = %d err=%v", limit, err)
	}
	if limit, err := gatewayUpstreamUserLimitFromEnv("3"); err != nil || limit != 3 {
		t.Fatalf("configured upstream user limit = %d err=%v", limit, err)
	}
	if _, err := gatewayUpstreamUserLimitFromEnv("0"); err == nil {
		t.Fatal("zero upstream user limit must be rejected")
	}
	app := &App{
		store:             &Store{},
		upstreamUserLimit: 2,
		gatewayActive:     map[string]map[string]int{},
		gatewayLeases:     map[string]map[string]time.Time{},
	}
	if !app.acquireGatewayUserSlot("upstream-1", "user-1", false) || !app.acquireGatewayUserSlot("upstream-1", "user-2", false) {
		t.Fatal("first two users should acquire the configured upstream user slots")
	}
	if app.acquireGatewayUserSlot("upstream-1", "user-3", false) {
		t.Fatal("third user exceeded the upstream user limit")
	}
	if app.acquireGatewayUserSlot("upstream-2", "user-1", false) {
		t.Fatal("a healthy assignment must not switch accounts without a route failure")
	}
	if !app.acquireGatewayUserSlot("upstream-2", "user-1", true) {
		t.Fatal("an explicit route failure should permit a controlled account switch")
	}
	app.store.mu.Lock()
	assigned := app.gatewayUserAssignedUpstreamLocked("user-1", time.Now().UTC())
	app.store.mu.Unlock()
	if assigned != "upstream-2" {
		t.Fatalf("controlled switch assigned %q, want upstream-2", assigned)
	}
}

func TestClientUsageHidesUpstreamOwnership(t *testing.T) {
	item := publicClientUsage(UsageRecord{UserID: "user-1", UpstreamAccountID: "up-secret", APIKeyID: "key-secret", Model: "gpt-5.5", TotalTokens: 9, CreatedAt: time.Now().UTC()})
	for _, field := range []string{"upstreamAccountId", "apiKeyId", "userId"} {
		if _, ok := item[field]; ok {
			t.Fatalf("client usage leaked routing field %q: %#v", field, item)
		}
	}
}

func TestAdminCanCancelCodexAuthorizationAndRestoreCandidate(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	var imported struct {
		Items []map[string]any `json:"items"`
	}
	if code := c.request(http.MethodPost, "/api/admin/upstreams/import", c.adminToken, map[string]any{"email": "cancel@example.com", "password": "cancel-secret"}, &imported); code != http.StatusCreated {
		t.Fatalf("import cancellation candidate = %d", code)
	}
	candidateID, _ := imported.Items[0]["id"].(string)
	cleaned := false
	c.store.mu.Lock()
	adminID := c.store.state.Admins[0].ID
	idx := c.app.upstreamIndex(candidateID)
	c.store.state.UpstreamAccounts[idx].AuthorizationStatus = upstreamAuthAction
	c.store.mu.Unlock()
	session := c.app.createCodexOAuthSession(adminID, candidateID, &codexDeviceCodeLogin{Cleanup: func() { cleaned = true }})
	var cancelled map[string]any
	if code := c.request(http.MethodPost, "/api/admin/codex-oauth/cancel", c.adminToken, map[string]any{"state": session.State}, &cancelled); code != http.StatusOK {
		t.Fatalf("cancel authorization status = %d", code)
	}
	if cancelled["status"] != "cancelled" || !cleaned {
		t.Fatalf("cancel authorization response = %#v cleaned=%v", cancelled, cleaned)
	}
	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	up := c.store.state.UpstreamAccounts[c.app.upstreamIndex(candidateID)]
	if up.AuthorizationStatus != upstreamAuthPending || up.LastAuthorizationError != "" || up.Status != statusDisabled {
		t.Fatalf("cancelled candidate state = %#v", up)
	}
}

func TestAdminImportUpstreamsStoresEmailPasswordAsPendingCandidate(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	body := map[string]any{
		"content": "email,password\nfirst@example.com,first-secret\nsecond@example.com----second-secret\nthird@example.com|third-secret",
	}
	var imported struct {
		Imported int              `json:"imported"`
		Pending  int              `json:"pending"`
		Items    []map[string]any `json:"items"`
		APIKeys  []map[string]any `json:"apiKeys"`
	}
	if code := c.request(http.MethodPost, "/api/admin/upstreams/import", c.adminToken, body, &imported); code != http.StatusCreated {
		t.Fatalf("import email/password status = %d", code)
	}
	if imported.Imported != 3 || imported.Pending != 3 || len(imported.Items) != 3 || len(imported.APIKeys) != 0 {
		t.Fatalf("pending import result = %#v", imported)
	}
	for _, item := range imported.Items {
		if item["authorizationStatus"] != upstreamAuthPending || item["credentialType"] != "email_password" || item["sourceType"] != "email_password" || item["hasPassword"] != true || item["enabled"] != false {
			t.Fatalf("pending item = %#v", item)
		}
		for _, secretField := range []string{"password", "passwordCipher", "accessToken", "refreshToken"} {
			if _, exists := item[secretField]; exists {
				t.Fatalf("pending item leaked %s: %#v", secretField, item)
			}
		}
	}
	rawResponse, err := json.Marshal(imported)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"first-secret", "second-secret", "third-secret"} {
		if strings.Contains(string(rawResponse), secret) {
			t.Fatalf("pending import leaked %q: %s", secret, rawResponse)
		}
	}
	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	if len(c.store.state.UpstreamAccounts) != 3 || len(c.store.state.APIKeys) != 0 {
		t.Fatalf("pending storage = upstreams:%#v keys:%#v", c.store.state.UpstreamAccounts, c.store.state.APIKeys)
	}
	password, err := c.app.decrypt(c.store.state.UpstreamAccounts[0].PasswordCipher)
	if err != nil || password != "first-secret" {
		t.Fatalf("encrypted password mismatch: password=%q err=%v", password, err)
	}
}

func TestEmailPasswordCSVPreservesQuotedPassword(t *testing.T) {
	reqs, err := parseAdminUpstreamImportContent("email,password\nquoted@example.com,\"pass,word with spaces\"", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 || reqs[0].Email != "quoted@example.com" || reqs[0].Password != "pass,word with spaces" || reqs[0].CredentialType != "email_password" {
		t.Fatalf("quoted CSV request = %#v", reqs)
	}
}

func TestAuthorizedImportUpgradesMatchingEmailCandidate(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	if code := c.request(http.MethodPost, "/api/admin/upstreams/import", c.adminToken, map[string]any{"email": "merge@example.com", "password": "merge-secret"}, nil); code != http.StatusCreated {
		t.Fatalf("candidate import = %d", code)
	}
	var authorized struct {
		Imported int              `json:"imported"`
		Updated  int              `json:"updated"`
		Items    []map[string]any `json:"items"`
		APIKeys  []map[string]any `json:"apiKeys"`
	}
	if code := c.request(http.MethodPost, "/api/admin/upstreams/import", c.adminToken, map[string]any{
		"email":            "merge@example.com",
		"accessToken":      "merge-access-token",
		"refreshToken":     "merge-refresh-token",
		"chatgptAccountId": "merge-account",
	}, &authorized); code != http.StatusCreated {
		t.Fatalf("authorized import = %d", code)
	}
	if authorized.Imported != 0 || authorized.Updated != 1 || len(authorized.Items) != 1 || len(authorized.APIKeys) != 1 {
		t.Fatalf("authorized merge response = %#v", authorized)
	}
	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	if len(c.store.state.UpstreamAccounts) != 1 || len(c.store.state.APIKeys) != 1 {
		t.Fatalf("authorized import duplicated candidate: upstreams=%#v keys=%#v", c.store.state.UpstreamAccounts, c.store.state.APIKeys)
	}
	up := c.store.state.UpstreamAccounts[0]
	if up.AuthorizationStatus != upstreamAuthAuthorized || up.PasswordCipher != "" || up.SourceType != "email_password" || up.ChatGPTAccountID != "merge-account" {
		t.Fatalf("merged candidate = %#v", up)
	}
}

func TestBoundCodexAuthorizationUpgradesCandidateAndClearsPassword(t *testing.T) {
	exp := time.Now().Add(24 * time.Hour).Unix()
	accessToken := testJWT(t, map[string]any{
		"exp":                            exp,
		"https://api.openai.com/auth":    map[string]any{"chatgpt_account_id": "bound-account", "chatgpt_plan_type": "plus"},
		"https://api.openai.com/profile": map[string]any{"email": "bound@example.com"},
	})
	authJSONRaw, err := json.Marshal(map[string]any{
		"auth_mode": "chatgpt",
		"tokens":    map[string]any{"access_token": accessToken, "refresh_token": "refresh-secret", "account_id": "bound-account"},
		"email":     "bound@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	oldStart := codexDeviceCodeLoginStart
	codexDeviceCodeLoginStart = func(context.Context, time.Duration) (*codexDeviceCodeLogin, error) {
		return &codexDeviceCodeLogin{
			VerificationURL: "https://auth.openai.com/codex/device",
			UserCode:        "BOUND-CODE",
			Wait:            func(context.Context) (string, error) { return string(authJSONRaw), nil },
		}, nil
	}
	t.Cleanup(func() { codexDeviceCodeLoginStart = oldStart })

	c := newTestClient(t)
	c.setupAdmin()
	var candidateImport struct {
		Items []map[string]any `json:"items"`
	}
	if code := c.request(http.MethodPost, "/api/admin/upstreams/import", c.adminToken, map[string]any{"email": "bound@example.com", "password": "candidate-secret"}, &candidateImport); code != http.StatusCreated {
		t.Fatalf("candidate import status = %d", code)
	}
	candidateID, _ := candidateImport.Items[0]["id"].(string)
	var started struct {
		State string `json:"state"`
	}
	if code := c.request(http.MethodPost, "/api/admin/codex-oauth/start", c.adminToken, map[string]any{"method": "device_code", "upstreamId": candidateID}, &started); code != http.StatusOK {
		t.Fatalf("bound oauth start = %d", code)
	}
	var status struct {
		Status string `json:"status"`
	}
	for i := 0; i < 50; i++ {
		if code := c.request(http.MethodGet, "/api/admin/codex-oauth/status?state="+url.QueryEscape(started.State), c.adminToken, nil, &status); code != http.StatusOK {
			t.Fatalf("bound oauth status = %d", code)
		}
		if status.Status == "imported" || status.Status == "failed" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if status.Status != "imported" {
		t.Fatalf("bound oauth completion = %#v", status)
	}
	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	if len(c.store.state.UpstreamAccounts) != 1 || len(c.store.state.APIKeys) != 1 {
		t.Fatalf("bound authorization duplicated account: upstreams=%#v keys=%#v", c.store.state.UpstreamAccounts, c.store.state.APIKeys)
	}
	up := c.store.state.UpstreamAccounts[0]
	if up.ID != candidateID || up.AuthorizationStatus != upstreamAuthAuthorized || up.Status != statusActive || up.PasswordCipher != "" || up.ChatGPTAccountID != "bound-account" {
		t.Fatalf("upgraded candidate = %#v", up)
	}
}

func TestAdminCanDeletePendingUpstreamCandidate(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	var imported struct {
		Items []map[string]any `json:"items"`
	}
	if code := c.request(http.MethodPost, "/api/admin/upstreams/import", c.adminToken, map[string]any{"email": "delete@example.com", "password": "delete-secret"}, &imported); code != http.StatusCreated {
		t.Fatalf("candidate import status = %d", code)
	}
	id, _ := imported.Items[0]["id"].(string)
	if code := c.request(http.MethodDelete, "/api/admin/upstreams/"+id, c.adminToken, nil, nil); code != http.StatusNoContent {
		t.Fatalf("candidate delete status = %d", code)
	}
	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	if len(c.store.state.UpstreamAccounts) != 0 || len(c.store.state.APIKeys) != 0 {
		t.Fatalf("candidate delete left state: upstreams=%#v keys=%#v", c.store.state.UpstreamAccounts, c.store.state.APIKeys)
	}
}

func TestSub2APIImportSkipsNonCodexPlatforms(t *testing.T) {
	reqs, err := parseAdminUpstreamImportPayload(map[string]any{
		"data": map[string]any{
			"type": "sub2api_backup",
			"accounts": []any{
				map[string]any{"name": "claude", "platform": "anthropic", "credentials": map[string]any{"access_token": "wrong-token"}},
				map[string]any{"name": "codex", "platform": "openai", "type": "codex", "credentials": map[string]any{"access_token": "codex-token"}},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 || reqs[0].Name != "codex" || reqs[0].AccessToken != "codex-token" || reqs[0].SourceType != "sub2api" {
		t.Fatalf("filtered sub2api requests = %#v", reqs)
	}
}

func TestAdminCodexBrowserCallbackRelayIsSessionBound(t *testing.T) {
	oldStart := codexBrowserLoginStart
	callbackReceived := make(chan string, 1)
	codexBrowserLoginStart = func(context.Context, time.Duration) (*codexDeviceCodeLogin, error) {
		return &codexDeviceCodeLogin{
			AuthMethod:      "app_server_browser",
			VerificationURL: "https://chatgpt.com/auth?state=expected",
			LoginID:         "browser-login",
			Callback: func(_ context.Context, value string) error {
				callbackReceived <- value
				return nil
			},
		}, nil
	}
	t.Cleanup(func() { codexBrowserLoginStart = oldStart })

	c := newTestClient(t)
	c.setupAdmin()
	var started struct {
		State      string `json:"state"`
		AuthMethod string `json:"authMethod"`
	}
	if code := c.request(http.MethodPost, "/api/admin/codex-oauth/start", c.adminToken, map[string]any{"method": "browser"}, &started); code != http.StatusOK {
		t.Fatalf("browser oauth start = %d", code)
	}
	callbackURL := "http://localhost:1455/auth/callback?code=code&state=expected"
	var relayed map[string]any
	if code := c.request(http.MethodPost, "/api/admin/codex-oauth/callback", c.adminToken, map[string]any{"state": started.State, "callbackUrl": callbackURL}, &relayed); code != http.StatusAccepted {
		t.Fatalf("browser callback relay = %d", code)
	}
	select {
	case got := <-callbackReceived:
		if got != callbackURL {
			t.Fatalf("relayed callback = %q", got)
		}
	default:
		t.Fatal("browser callback was not relayed")
	}
	if started.AuthMethod != "app_server_browser" || relayed["status"] != "callback_received" {
		t.Fatalf("browser callback response = started:%#v relayed:%#v", started, relayed)
	}
}

func TestValidateCodexBrowserCallbackURL(t *testing.T) {
	valid := "http://localhost:1455/auth/callback?code=secret-code&state=expected"
	if got, err := validateCodexBrowserCallbackURL(valid, "expected"); err != nil || !strings.Contains(got, "code=secret-code") {
		t.Fatalf("valid callback = %q, %v", got, err)
	}
	if _, err := validateCodexBrowserCallbackURL(valid, ""); err == nil {
		t.Fatal("callback without an expected OAuth state must be rejected")
	}
	for _, invalid := range []string{
		"https://localhost:1455/auth/callback?code=x&state=expected",
		"http://example.com:1455/auth/callback?code=x&state=expected",
		"http://127.0.0.1:1455/other?code=x&state=expected",
		"http://127.0.0.1:1455/auth/callback?code=x&state=wrong",
	} {
		if _, err := validateCodexBrowserCallbackURL(invalid, "expected"); err == nil {
			t.Fatalf("callback should be rejected: %s", invalid)
		}
	}
}

func TestAdminImportUpstreamsRecognizesCodexAuthJSON(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	exp := time.Now().Add(24 * time.Hour).Unix()
	accessToken := testJWT(t, map[string]any{
		"exp": exp,
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": "account-from-access",
			"chatgpt_plan_type":  "plus",
		},
		"https://api.openai.com/profile": map[string]any{
			"email": "codex-auth@example.com",
		},
	})
	idToken := testJWT(t, map[string]any{
		"iss":   "https://auth.openai.com",
		"aud":   []any{"test-codex-client"},
		"email": "codex-auth-id@example.com",
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": "account-from-id",
			"chatgpt_plan_type":  "team",
		},
	})
	body := map[string]any{
		"auth_mode": "chatgpt",
		"tokens": map[string]any{
			"access_token":  accessToken,
			"id_token":      idToken,
			"refresh_token": "",
			"account_id":    "account-from-tokens",
		},
		"last_refresh": "2026-07-06T07:47:26Z",
	}
	var imported struct {
		Imported int              `json:"imported"`
		Items    []map[string]any `json:"items"`
		APIKeys  []map[string]any `json:"apiKeys"`
	}
	if code := c.request(http.MethodPost, "/api/admin/upstreams/import", c.adminToken, body, &imported); code != http.StatusCreated {
		t.Fatalf("import codex auth status = %d", code)
	}
	if imported.Imported != 1 || len(imported.Items) != 1 || len(imported.APIKeys) != 1 {
		t.Fatalf("imported codex auth = %#v", imported)
	}
	item := imported.Items[0]
	if item["chatgptAccountId"] != "account-from-tokens" || item["email"] != "codex-auth@example.com" || item["subscriptionTier"] != "plus" || item["entitlementStatus"] != "short_lived_auth" || item["credentialStatus"] != "short_lived_auth" {
		t.Fatalf("imported codex auth metadata = %#v", item)
	}
	if item["expiresAt"] == nil {
		t.Fatalf("imported codex auth missing expiry: %#v", item)
	}
	rawResp, err := json.Marshal(imported)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{accessToken, idToken, "refresh_token"} {
		if strings.Contains(string(rawResp), secret) {
			t.Fatalf("import response leaked secret %q: %s", secret, rawResp)
		}
	}
	c.store.mu.Lock()
	up := c.store.state.UpstreamAccounts[0]
	refreshCipher := up.RefreshTokenCipher
	authJSONCipher := up.AuthJSONCipher
	c.store.mu.Unlock()
	if refreshCipher != "" {
		t.Fatalf("empty refresh token should not be encrypted as present: %#v", up)
	}
	authJSONRaw, err := c.app.decrypt(authJSONCipher)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(authJSONRaw, `"auth_mode":"chatgpt"`) || !strings.Contains(authJSONRaw, accessToken) {
		t.Fatalf("stored auth json snapshot not preserved")
	}

	var repeated struct {
		Imported int              `json:"imported"`
		Updated  int              `json:"updated"`
		Items    []map[string]any `json:"items"`
		APIKeys  []map[string]any `json:"apiKeys"`
	}
	if code := c.request(http.MethodPost, "/api/admin/upstreams/import", c.adminToken, body, &repeated); code != http.StatusCreated {
		t.Fatalf("repeat import codex auth status = %d", code)
	}
	if repeated.Imported != 0 || repeated.Updated != 1 || len(repeated.Items) != 1 || len(repeated.APIKeys) != 1 {
		t.Fatalf("repeat import must update existing upstream/key only: %#v", repeated)
	}
	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	if len(c.store.state.UpstreamAccounts) != 1 || len(c.store.state.APIKeys) != 1 {
		t.Fatalf("repeat import created duplicates: upstreams=%#v keys=%#v", c.store.state.UpstreamAccounts, c.store.state.APIKeys)
	}
}

func TestAdminCodexOAuthStartUsesAppServerDeviceCode(t *testing.T) {
	oldStart := codexDeviceCodeLoginStart
	codexDeviceCodeLoginStart = func(ctx context.Context, ttl time.Duration) (*codexDeviceCodeLogin, error) {
		return &codexDeviceCodeLogin{
			VerificationURL: "https://auth.openai.com/activate",
			UserCode:        "ABCD-EFGH",
			LoginID:         "login-1",
		}, nil
	}
	t.Cleanup(func() { codexDeviceCodeLoginStart = oldStart })

	c := newTestClient(t)
	c.setupAdmin()
	var started struct {
		State           string    `json:"state"`
		AuthMethod      string    `json:"authMethod"`
		VerificationURL string    `json:"verificationUrl"`
		AuthorizeURL    string    `json:"authorizeUrl"`
		UserCode        string    `json:"userCode"`
		LoginID         string    `json:"loginId"`
		ExpiresAt       time.Time `json:"expiresAt"`
	}
	if code := c.request(http.MethodPost, "/api/admin/codex-oauth/start", c.adminToken, map[string]any{}, &started); code != http.StatusOK {
		t.Fatalf("start codex oauth status = %d", code)
	}
	if started.State == "" || started.VerificationURL == "" || !started.ExpiresAt.After(time.Now()) {
		t.Fatalf("start codex oauth payload = %#v", started)
	}
	if started.AuthMethod != "app_server_device_code" || started.AuthorizeURL != started.VerificationURL || started.UserCode != "ABCD-EFGH" || started.LoginID != "login-1" {
		t.Fatalf("start codex oauth device-code payload = %#v", started)
	}
	var status struct {
		Status          string `json:"status"`
		State           string `json:"state"`
		AuthMethod      string `json:"authMethod"`
		VerificationURL string `json:"verificationUrl"`
		UserCode        string `json:"userCode"`
	}
	if code := c.request(http.MethodGet, "/api/admin/codex-oauth/status?state="+url.QueryEscape(started.State), c.adminToken, nil, &status); code != http.StatusOK {
		t.Fatalf("codex oauth status code = %d", code)
	}
	if status.Status != "pending" || status.State != started.State || status.AuthMethod != "app_server_device_code" || status.VerificationURL != started.VerificationURL || status.UserCode != "ABCD-EFGH" {
		t.Fatalf("codex oauth status = %#v", status)
	}
}

func TestAdminCodexOAuthDeviceCodeCompletionImportsAuthJSON(t *testing.T) {
	exp := time.Now().Add(24 * time.Hour).Unix()
	accessToken := testJWT(t, map[string]any{
		"exp": exp,
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": "device-account",
			"chatgpt_plan_type":  "plus",
		},
		"https://api.openai.com/profile": map[string]any{
			"email": "device@example.com",
		},
	})
	idToken := testJWT(t, map[string]any{
		"iss":   "https://auth.openai.com",
		"aud":   []any{"test-codex-client"},
		"email": "device-id@example.com",
	})
	authJSON := map[string]any{
		"auth_mode": "chatgpt",
		"tokens": map[string]any{
			"access_token":  accessToken,
			"id_token":      idToken,
			"refresh_token": "refresh-secret",
			"account_id":    "device-account",
		},
		"email":        "device@example.com",
		"plan_type":    "plus",
		"last_refresh": time.Now().UTC().Format(time.RFC3339),
	}
	authJSONRaw, err := json.Marshal(authJSON)
	if err != nil {
		t.Fatal(err)
	}
	oldStart := codexDeviceCodeLoginStart
	codexDeviceCodeLoginStart = func(ctx context.Context, ttl time.Duration) (*codexDeviceCodeLogin, error) {
		return &codexDeviceCodeLogin{
			VerificationURL: "https://auth.openai.com/activate",
			UserCode:        "ABCD-EFGH",
			LoginID:         "login-1",
			Wait: func(context.Context) (string, error) {
				return string(authJSONRaw), nil
			},
		}, nil
	}
	t.Cleanup(func() { codexDeviceCodeLoginStart = oldStart })

	c := newTestClient(t)
	c.setupAdmin()
	var started struct {
		State string `json:"state"`
	}
	if code := c.request(http.MethodPost, "/api/admin/codex-oauth/start", c.adminToken, map[string]any{}, &started); code != http.StatusOK {
		t.Fatalf("start codex oauth status = %d", code)
	}
	var status struct {
		Status      string `json:"status"`
		Imported    int    `json:"imported"`
		Updated     int    `json:"updated"`
		AccountName string `json:"accountName"`
	}
	for i := 0; i < 50; i++ {
		if code := c.request(http.MethodGet, "/api/admin/codex-oauth/status?state="+url.QueryEscape(started.State), c.adminToken, nil, &status); code != http.StatusOK {
			t.Fatalf("codex oauth status code = %d", code)
		}
		if status.Status == "imported" || status.Status == "failed" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if status.Status != "imported" || status.Imported != 1 || status.Updated != 0 || status.AccountName == "" {
		t.Fatalf("codex oauth completion status = %#v", status)
	}
	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	if len(c.store.state.UpstreamAccounts) != 1 || len(c.store.state.APIKeys) != 1 {
		t.Fatalf("device-code completion did not import one upstream/key: upstreams=%#v keys=%#v", c.store.state.UpstreamAccounts, c.store.state.APIKeys)
	}
	up := c.store.state.UpstreamAccounts[0]
	if up.ChatGPTAccountID != "device-account" || up.Email != "device@example.com" || up.SubscriptionTier != "plus" {
		t.Fatalf("imported device-code upstream = %#v", up)
	}
	if up.AccessTokenCipher == accessToken || up.RefreshTokenCipher == "refresh-secret" || up.AuthJSONCipher == string(authJSONRaw) {
		t.Fatalf("device-code import stored plaintext secrets: %#v", up)
	}
}

func TestCodexAuthJSONRequiresRefreshableAuth(t *testing.T) {
	exp := time.Now().Add(24 * time.Hour).Unix()
	accessToken := testJWT(t, map[string]any{
		"exp": exp,
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": "oauth-account",
			"chatgpt_plan_type":  "plus",
		},
		"https://api.openai.com/profile": map[string]any{
			"email": "oauth@example.com",
		},
	})
	idToken := testJWT(t, map[string]any{
		"iss":   "https://auth.openai.com",
		"aud":   []any{"test-codex-client"},
		"email": "oauth-id@example.com",
	})
	authJSONRaw, err := json.Marshal(map[string]any{
		"auth_mode": "chatgpt",
		"tokens": map[string]any{
			"access_token":  accessToken,
			"id_token":      idToken,
			"refresh_token": "refresh-secret",
			"account_id":    "oauth-account",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err := codexAuthRequestFromAuthJSONRaw(string(authJSONRaw))
	if err != nil {
		t.Fatal(err)
	}
	if req.AccessToken != accessToken || req.RefreshToken != "refresh-secret" || req.ChatGPTAccountID != "oauth-account" || req.Email != "oauth@example.com" || req.SubscriptionTier != "plus" {
		t.Fatalf("codex auth json request = %#v", req)
	}
	if !strings.Contains(req.AuthJSONRaw, `"refresh_token":"refresh-secret"`) || !strings.Contains(req.AuthJSONRaw, `"auth_mode":"chatgpt"`) {
		t.Fatalf("codex auth json missing refreshable auth: %s", req.AuthJSONRaw)
	}
	shortAuthJSONRaw, err := json.Marshal(map[string]any{
		"auth_mode": "chatgpt",
		"tokens": map[string]any{
			"access_token": accessToken,
			"id_token":     idToken,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := codexAuthRequestFromAuthJSONRaw(string(shortAuthJSONRaw)); err == nil || err.Error() != "codex_device_code_refresh_token_missing" {
		t.Fatalf("device-code auth json must be refreshable, err=%v", err)
	}
}

func TestAdminImportUpstreamsRecognizesMixedTextLikeSub2API(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	body := map[string]any{
		"content": "raw-token-1\n{\"accessToken\":\"json-token\",\"email\":\"json@example.com\",\"account\":{\"id\":\"acct-json\",\"planType\":\"plus\"}}\n[\"array-token\"]",
	}
	var imported struct {
		Imported int              `json:"imported"`
		APIKeys  []map[string]any `json:"apiKeys"`
	}
	if code := c.request(http.MethodPost, "/api/admin/upstreams/import", c.adminToken, body, &imported); code != http.StatusCreated {
		t.Fatalf("mixed import status = %d", code)
	}
	if imported.Imported != 3 || len(imported.APIKeys) != 3 {
		t.Fatalf("mixed import result = %#v", imported)
	}
	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	if len(c.store.state.UpstreamAccounts) != 3 || len(c.store.state.APIKeys) != 3 {
		t.Fatalf("stored mixed import upstreams=%#v keys=%#v", c.store.state.UpstreamAccounts, c.store.state.APIKeys)
	}
	if c.store.state.UpstreamAccounts[1].Email != "json@example.com" || c.store.state.UpstreamAccounts[1].ChatGPTAccountID != "acct-json" || c.store.state.UpstreamAccounts[1].SubscriptionTier != "plus" {
		t.Fatalf("json-line metadata = %#v", c.store.state.UpstreamAccounts[1])
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
		"switch-toggle",
		"号池账号与 API Key",
		"导出号池",
		"/admin/upstreams/export",
		"reauth_required",
		"requireFreshAdminLogin",
		"为保护号池授权数据，请重新登录后再导出",
		"<th>号池账号</th>",
		"<th>使用用户</th>",
		"<th>今日使用</th>",
		"<th>账号剩余</th>",
		"<th>备注</th>",
		"正在使用",
		"空闲",
		"最近使用",
		"使用时间段",
		"lastUsedFrom",
		"activeUserAccounts",
		"routedUsageTokens",
		"授权来源",
		"套餐",
		"授权",
		"额度",
		"添加号池账号",
		"待授权账号",
		"邮箱密码",
		"ChatGPT session JSON",
		"从已登录的 ChatGPT 会话导入",
		"https://chatgpt.com/api/auth/session",
		"获取 session JSON",
		"chatgpt_session: 'ChatGPT session'",
		"Sub2API",
		"Codex auth.json",
		"data-authorize-candidate",
		"data-delete-candidate",
		"已删除待授权账号",
		`data-method="browser"`,
		`data-method="device_code"`,
		"/admin/codex-oauth/start",
		"/admin/codex-oauth/callback",
		"/admin/codex-oauth/cancel",
		"/admin/codex-oauth/status",
		`aria-label="关闭添加账号"`,
		`aria-label="关闭授权"`,
		"window.open",
		"const authUrl = data.verificationUrl || data.authorizeUrl",
		"popup.location.href = authUrl",
		"window.location.href = authUrl",
		"验证码：${data.userCode}",
		"已导入 Codex 授权",
		"Codex 授权失败",
		"支持 ChatGPT session JSON、邮箱密码、Codex auth.json、Sub2API 账号备份",
		`id="upstreamImportJson"`,
		`id="upstreamImportFile"`,
		"/admin/upstreams/import",
		"content: raw",
		"其中 ${data.pending || 0} 条待授权",
		"<th>状态</th>",
		"<th>操作</th>",
		"data-api-key-delete",
		"data-edit-upstream-remark",
		"upstreamRemarkForm",
		"/remark",
		"备注已保存",
		"已删除 API Key",
		"检查中",
		"upstreamCheckingIds",
		"upstreamRenderRevision",
		"renderRevision !== upstreamRenderRevision",
		"state.upstreamCheckingIds.length",
		"onlineClientAccounts",
		"客户端在线",
		"Codex 运行中",
		"checkFailureReason",
		"正在读取授权与额度",
		"检查失败：${reason}",
		"正在检查授权",
		"已完成授权检查",
		"无额度数据",
		"已检查，无额度数据",
		"检查于",
		"未过期",
		"检查失败",
		"认证失败",
		"短期授权",
		"可自动续期",
		"缺少 chatgptAccountId",
		"switch-slot",
		"grid-template-columns: 1fr",
		"upstreamLimitLines(up)",
		"upstreamValidity(up)",
		"upstreamAvailabilityCell(up)",
		"key.keyPreview || key.id",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("admin upstream UI missing %q", required)
		}
	}
	for _, forbidden := range []string{"手动导入", "替换 Codex 上游账号凭据", "data-up-credentials", "data-up-enabled", "data-cancel-upstream-credentials", "upstreamCredentialEditId", "<label>access_token", "<label>refresh_token", "credentialType", "token_type", "<th>账号详情</th>", "<th>启用</th>", "账号详情", "账号启用", "<th>剩余</th>", "<th>余额</th>", "<th>风控</th>", "上游余额", "风控不可用", "余额不可用", "min-width: 1180px", "flex-wrap: nowrap", "生成 Key</button>", "data-up-balance", "data-up-risk", "data-up-availability", "balanceStatus", "riskStatus", "credentialFingerprint", "accessTokenCipher", "refreshTokenCipher", "authJsonCipher", "JSON.parse(raw)", "prompt(", "alert(", "confirm("} {
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
		"已更新 API Key 状态",
		"已删除 API Key",
		"keyPreview",
		"未生成 Key",
		"data-api-key-delete",
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
		"usageAnalyticsAccounts",
		"request(analyticsPath)",
		"/admin/usage/analytics?",
		"renderUsageAnalyticsChart",
		"averageTokensPerTask",
		"账号 Token 统计",
		"任务量下限",
		"Token 上限",
		"恢复默认",
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
		"本机 Codex 版本",
		"本机 Codex 版本兼容",
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
	if rows := strings.Count(block, "        ['"); rows != 5 {
		t.Fatalf("client diagnostics must show exactly five rows, got %d in %s", rows, block)
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
		"install_desktop_update",
		"exit_for_desktop_update",
		"更新客户端",
		"正在更新",
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
	end := strings.Index(text[start:], "\n    function table(")
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
		"/client/presence/heartbeat",
		"/client/presence/stop",
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
		"未检测到 Codex 桌面端，请先在客户端内安装官方 ChatGPT 桌面应用",
		"本机 Codex 主程序启动失败，请确认 Codex 可从${officialAppLocation()}正常打开",
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
		"$('launchBtn').textContent = '安装 Codex 桌面端';",
		"$('launchBtn').textContent = '启动 Codex';",
		"$('launchBtn').disabled = true;",
		"$('launchBtn').disabled = false;",
		"async function confirmCodexRunningInBackground()",
		"if (state.codexLaunchPhase === 'launched') return '已启动';",
		"$('launchBtn').textContent = 'Codex 已启动';",
		"state.codexLaunchPhase = 'launched';",
		"capabilityWarning ? `Codex 已启动；${capabilityWarning}` : 'Codex 已启动'",
		"async function stopCodex()",
		"const stopped = await invokeTauri('stop_codex');",
		"async function installCodex()",
		"await invokeTauri('install_codex');",
		"async function launchButtonAction()",
		"if (status === '未检测到' || status === '需要更新' || status === '版本待验证') return installCodex();",
		`<div id="installProgress" class="install-progress hidden" aria-live="polite">`,
		`id="installProgressFill"`,
		"function startInstallProgress()",
		"function finishInstallProgress(ok)",
		"if (state.codexLaunchPhase === 'installing') return '正在安装';",
		"state.codexLaunchPhase = 'installing';",
		"$('launchBtn').textContent = updating ? '正在更新 Codex 桌面端' : '正在安装 Codex 桌面端';",
		"$('spark').classList.add('hidden');",
		"$('spark').classList.remove('hidden');",
		"setInstallProgress(100, 'Codex 桌面端已就绪');",
		"Codex 桌面端已安装",
		"function startDiagnosticsAutoRefresh()",
		"setInterval(refreshLocalDiagnostics, 6000)",
		`<p>Codex 账号 <span id="currentAccount" class="status"></span></p>`,
		"function codexAccountDisplay()",
		"if (state.diagnostics.codexAccount) return state.diagnostics.codexAccount;",
		"if (state.diagnostics.codexRunning === '可用') return;",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("client launch state UI missing %q", required)
		}
	}
	for _, forbidden := range []string{"启动 Codex+++", "请使用 Windows 客户端启动 Codex+++", `id="launchState"`, "$('launchState')", "Codex 运行中", "previousAccount", "providerAccount", "backendCodexAccount", "prep.provider.account", "Codex+++ 授权"} {
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
	end := strings.Index(text[start:], "\n    async function confirmCodexRunningInBackground")
	if end < 0 {
		t.Fatal("launchCodex function boundary missing")
	}
	block := text[start : start+end]
	meIdx := strings.Index(block, "const me = await request('/client/me');")
	applyIdx := strings.Index(block, "applyClientMe(me);")
	backendPrepareIdx := strings.Index(block, "const prep = await request('/client/launch/prepare'")
	tokenIdx := strings.Index(block, "const providerToken = prep.provider && prep.provider.bearerToken;")
	localPrepareIdx := strings.Index(block, "const localPrep = await invokeTauri('prepare_codex', { backendUrl: state.api, providerToken });")
	if meIdx < 0 || applyIdx < 0 || backendPrepareIdx < 0 || tokenIdx < 0 || localPrepareIdx < 0 {
		t.Fatalf("launchCodex must refresh client state before local prepare: %s", block)
	}
	if !(meIdx < applyIdx && applyIdx < backendPrepareIdx && backendPrepareIdx < tokenIdx && tokenIdx < localPrepareIdx) {
		t.Fatalf("launchCodex check order invalid me=%d apply=%d backendPrepare=%d token=%d localPrepare=%d block=%s", meIdx, applyIdx, backendPrepareIdx, tokenIdx, localPrepareIdx, block)
	}
	for _, required := range []string{"if (!providerToken) throw new Error('codex_provider_unavailable');"} {
		if !strings.Contains(block, required) {
			t.Fatalf("launchCodex missing API key login guard %q in %s", required, block)
		}
	}
	if strings.Contains(block, "state.diagnostics.codexAuthMode !== 'chatgpt'") {
		t.Fatalf("launchCodex must route existing ChatGPT login through the account pool: %s", block)
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
		"desktop_client_required: '请使用 Codex+++ 桌面客户端登录'",
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
		"backend_timeout: '连接服务器超时，请检查系统代理或防火墙'",
		"backend_connect_failed: '客户端无法连接服务器，请检查系统代理、防火墙或安全软件'",
		"backend_response_failed: '服务器响应异常，请稍后重试'",
		"backend_request_invalid: '客户端网络配置异常，请更新客户端'",
		"invokeTauri('backend_request'",
		"desktop_client_required: '请使用 Codex+++ 桌面客户端登录'",
		"codex_stop_failed: 'Codex 停止失败，请稍后重试'",
		"codex_restore_failed: 'Codex 已停止，但原登录配置恢复失败，请联系管理员'",
		"codex_install_unavailable: 'Codex 桌面端安装不可用，请联系管理员'",
		"codex_install_component_missing: 'Codex 桌面端安装组件不可用，请联系管理员'",
		"codex_install_failed: 'Codex 桌面端安装失败，请联系管理员'",
		"codex_browser_plugin_install_failed: 'Browser 能力未能自动配置，请在 Codex 的插件目录确认 Browser 已启用'",
		"codex_version_not_verified: `Codex 尚未通过${officialUpdateSource()}实时版本检查，请先检查更新`",
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
	if !strings.Contains(text, "role IN ('admin', 'client')") || strings.Contains(text, "role IN ('admin', 'client', 'codex')") {
		t.Fatalf("migration must not keep codex provider sessions")
	}
	if !strings.Contains(text, "type IN ('recharge', 'adjustment', 'debit')") {
		t.Fatalf("migration must keep token ledger types clean")
	}
	if strings.Contains(text, "'release', 'refund', 'correction'") {
		t.Fatalf("migration contains unused token ledger types")
	}
}

func TestGatewayUsesDirectCodexResponsesResult(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("cora")
	c.loginClient("cora")
	c.approvePaidRecharge()

	originalRun := codexResponsesRun
	codexResponsesRun = func(ctx context.Context, credentials codexProbeCredentials, requestBody *gatewayRequestSource, requestHeaders http.Header) (codexResponsesResult, error) {
		if credentials.AccessToken != "access" || credentials.ChatGPTAccountID != "account-123" || credentials.ChatGPTPlanType != "pro" {
			t.Fatalf("run credentials = %#v", credentials)
		}
		var payload map[string]any
		rawRequestBody, err := requestBody.Bytes()
		if err != nil {
			t.Fatal(err)
		}
		if err := json.NewDecoder(bytes.NewReader(rawRequestBody)).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if _, ok := payload["usage"]; ok {
			t.Fatalf("client usage was forwarded upstream: %#v", payload)
		}
		for _, key := range []string{"route", "routeId", "apiKey", "apiKeyId", "upstream", "upstreamId", "gatewayPath", "proxy", "endpoint", "base_url", "baseUrl"} {
			if _, ok := payload[key]; ok {
				t.Fatalf("old route field %q was forwarded upstream: %#v", key, payload)
			}
		}
		return completedCodexTestResult("gpt-5.5", "done", gatewayUsage{InputTokens: 8, CachedInputTokens: 3, OutputTokens: 13, TotalTokens: 21}), nil
	}
	t.Cleanup(func() { codexResponsesRun = originalRun })
	c.createRoutedUpstream("")

	var resp struct {
		UsageRecord   UsageRecord    `json:"usageRecord"`
		ChargedTokens int64          `json:"chargedTokens"`
		Result        map[string]any `json:"result"`
	}
	status := c.gatewayRun(c.clientToken, map[string]any{
		"model":       "gpt-5.5",
		"input":       "run through direct responses proxy",
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
	if failure["error"] != "codex_responses_usage_missing" {
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
	if failure["error"] != "codex_responses_usage_missing" {
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

func TestGatewaySwitchesOnCodexResponsesRateLimit(t *testing.T) {
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
	originalRun := codexResponsesRun
	codexResponsesRun = func(ctx context.Context, credentials codexProbeCredentials, requestBody *gatewayRequestSource, requestHeaders http.Header) (codexResponsesResult, error) {
		calls[credentials.AccessToken]++
		switch credentials.AccessToken {
		case firstToken:
			return codexResponsesResult{Status: http.StatusTooManyRequests}, nil
		case secondToken:
			return completedCodexTestResult("gpt-5.5", "ok", gatewayUsage{InputTokens: 5, OutputTokens: 3, TotalTokens: 8}), nil
		default:
			return codexResponsesResult{}, fmt.Errorf("codex_responses_unavailable")
		}
	}
	t.Cleanup(func() { codexResponsesRun = originalRun })

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
		if item.ActorRole != "system" || item.TargetID != secondID || !strings.Contains(item.Detail, "reason=upstream_limited") {
			t.Fatalf("switch audit = %#v", item)
		}
		if strings.Contains(item.Detail, "usageLimitExceeded") || strings.Contains(item.Detail, "message=limit") {
			t.Fatalf("switch audit leaked raw upstream details: %#v", item)
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

func TestGatewayKeepsUserOnSelectedUpstreamAcrossTasks(t *testing.T) {
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
	if firstCalls != 2 || secondCalls != 0 || secondResp.ChargedTokens != 4 {
		t.Fatalf("stable user dispatch calls first=%d second=%d resp=%#v", firstCalls, secondCalls, secondResp)
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
	used := 0
	for _, item := range keys.Items {
		if item["lastUsedAt"] != nil {
			used++
		}
	}
	if used != 1 {
		t.Fatalf("stable user should touch one route key, got %#v", keys.Items)
	}
}

func TestGatewayKeepsSessionOnSelectedUpstream(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("mira")
	c.loginClient("mira")
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

	headers := map[string]string{"Idempotency-Key": "sticky-1", "X-Codex-Session-ID": "thread-1"}
	if code := c.gatewayRunWithHeaders(c.clientToken, headers, map[string]any{"model": "gpt-5.5", "input": "first"}, nil); code != http.StatusOK {
		t.Fatalf("first sticky gateway status = %d", code)
	}
	headers = map[string]string{"Idempotency-Key": "sticky-2", "X-Codex-Session-ID": "thread-1"}
	if code := c.gatewayRunWithHeaders(c.clientToken, headers, map[string]any{"model": "gpt-5.5", "input": "second"}, nil); code != http.StatusOK {
		t.Fatalf("second sticky gateway status = %d", code)
	}
	if firstCalls != 2 || secondCalls != 0 {
		t.Fatalf("sticky session dispatch calls first=%d second=%d", firstCalls, secondCalls)
	}
	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	if len(c.store.state.UsageRecords) != 2 || c.store.state.UsageRecords[0].SessionID != "thread-1" || c.store.state.UsageRecords[1].SessionID != "thread-1" {
		t.Fatalf("usage session ids = %#v", c.store.state.UsageRecords)
	}
}

func TestGatewayKeepsSessionOnSelectedUpstreamAfterRestart(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("restart-sticky")
	c.loginClient("restart-sticky")
	c.approvePaidRecharge()

	firstCalls := 0
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		firstCalls++
		writeJSON(w, http.StatusOK, map[string]any{
			"model": "gpt-5.5",
			"usage": map[string]any{"input_tokens": 2, "output_tokens": 2, "total_tokens": 4},
		})
	}))
	defer first.Close()
	c.createRoutedUpstream(first.URL)

	secondCalls := 0
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondCalls++
		writeJSON(w, http.StatusOK, map[string]any{
			"model": "gpt-5.5",
			"usage": map[string]any{"input_tokens": 3, "output_tokens": 3, "total_tokens": 6},
		})
	}))
	defer second.Close()
	c.createRoutedUpstream(second.URL)

	headers := map[string]string{"Idempotency-Key": "restart-sticky-1", "X-Codex-Session-ID": "private-thread-id"}
	if code := c.gatewayRunWithHeaders(c.clientToken, headers, map[string]any{"model": "gpt-5.5", "input": "before restart"}, nil); code != http.StatusOK {
		t.Fatalf("first sticky gateway status = %d", code)
	}

	reopened, err := OpenStore(c.store.path)
	if err != nil {
		t.Fatal(err)
	}
	restartedApp := &App{store: reopened, secretKey: deriveKey("test-secret")}
	if err := restartedApp.initializeRuntimeState(); err != nil {
		t.Fatal(err)
	}
	restarted := &testClient{t: t, handler: restartedApp.routes(), app: restartedApp, store: reopened, clientToken: c.clientToken}
	headers = map[string]string{"Idempotency-Key": "restart-sticky-2", "X-Codex-Session-ID": "private-thread-id"}
	if code := restarted.gatewayRunWithHeaders(restarted.clientToken, headers, map[string]any{"model": "gpt-5.5", "input": "after restart"}, nil); code != http.StatusOK {
		t.Fatalf("second sticky gateway status = %d", code)
	}
	if firstCalls != 2 || secondCalls != 0 {
		t.Fatalf("persisted sticky session dispatch calls first=%d second=%d", firstCalls, secondCalls)
	}
	reopened.mu.Lock()
	defer reopened.mu.Unlock()
	if len(reopened.state.GatewaySessions) != 1 {
		t.Fatalf("persisted gateway sessions = %#v", reopened.state.GatewaySessions)
	}
	session := reopened.state.GatewaySessions[0]
	if session.SessionKey != hashString("private-thread-id") || strings.Contains(session.SessionKey, "private-thread-id") {
		t.Fatalf("session identifier was not hashed: %#v", session)
	}
}

func TestAppReleasesInterruptedGatewayReservationsOnRestart(t *testing.T) {
	c := newTestClient(t)
	c.setupAdmin()
	c.createUser("interrupted-reservation")
	c.loginClient("interrupted-reservation")
	c.approvePaidRecharge()

	c.store.mu.Lock()
	userID := c.store.state.Users[0].ID
	now := time.Now().UTC().Add(-time.Minute)
	c.store.state.GatewayRequests = append(c.store.state.GatewayRequests, GatewayRequest{
		ID:             c.store.nextID("gw"),
		UserID:         userID,
		RequestID:      "interrupted-request",
		Status:         gatewayReserved,
		ReservedTokens: 65536,
		CreatedAt:      now,
		UpdatedAt:      now,
	})
	if err := c.store.save(); err != nil {
		c.store.mu.Unlock()
		t.Fatal(err)
	}
	c.store.mu.Unlock()

	reopened, err := OpenStore(c.store.path)
	if err != nil {
		t.Fatal(err)
	}
	restartedApp := &App{store: reopened, secretKey: deriveKey("test-secret")}
	if err := restartedApp.initializeRuntimeState(); err != nil {
		t.Fatal(err)
	}
	reopened.mu.Lock()
	defer reopened.mu.Unlock()
	if got := restartedApp.availableTokenBalanceLocked(userID); got != reopened.state.Users[0].TokenBalance {
		t.Fatalf("available balance after restart = %d, balance = %d", got, reopened.state.Users[0].TokenBalance)
	}
	req := reopened.state.GatewayRequests[len(reopened.state.GatewayRequests)-1]
	if req.Status != gatewayFailed || req.ReservedTokens != 0 || req.Error != "backend_restarted" {
		t.Fatalf("interrupted reservation was not released: %#v", req)
	}
	foundAudit := false
	for _, item := range reopened.state.AuditLogs {
		if item.Action == "gateway.request.recovered" && item.TargetID == "interrupted-request" {
			foundAudit = true
			break
		}
	}
	if !foundAudit {
		t.Fatalf("reservation recovery audit not found: %#v", reopened.state.AuditLogs)
	}
}

func TestGatewaySessionIDRecognizesOfficialCodexHeaders(t *testing.T) {
	for _, header := range []string{"Session-ID", "Thread-ID"} {
		req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
		req.Header.Set(header, "codex-thread-123")
		if got := gatewaySessionID(req, []byte(`{}`)); got != "codex-thread-123" {
			t.Fatalf("%s session id = %q", header, got)
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	if got := gatewaySessionID(req, []byte(`{"previous_response_id":"resp_previous"}`)); got != "" {
		t.Fatalf("previous response id must not be treated as a stable task id: %q", got)
	}
}

type observedStreamWriter struct {
	header http.Header
	status int
	body   bytes.Buffer
	first  chan struct{}
}

func (w *observedStreamWriter) Header() http.Header { return w.header }
func (w *observedStreamWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}
func (w *observedStreamWriter) Write(chunk []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	select {
	case <-w.first:
	default:
		close(w.first)
	}
	return w.body.Write(chunk)
}
func (w *observedStreamWriter) Flush() {}

func TestCodexResponsesStreamingForwardsBeforeCompletion(t *testing.T) {
	release := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"first\"}\n\n")
		w.(http.Flusher).Flush()
		<-release
		_, _ = io.WriteString(w, "data: {\"type\":\"response.completed\",\"response\":{\"model\":\"gpt-5.5\",\"usage\":{\"input_tokens\":2,\"output_tokens\":3,\"total_tokens\":5}}}\n\n")
	}))
	defer upstream.Close()
	t.Setenv("CODEXPPP_CODEX_RESPONSES_URL", upstream.URL)
	target := &codexStreamTarget{Writer: &observedStreamWriter{header: http.Header{}, first: make(chan struct{})}}
	type streamResult struct {
		result codexResponsesResult
		err    error
	}
	done := make(chan streamResult, 1)
	go func() {
		result, err := runCodexResponsesStreamingRequest(context.Background(), codexProbeCredentials{AccessToken: "access", ChatGPTAccountID: "account"}, []byte(`{"stream":true}`), http.Header{}, target)
		done <- streamResult{result: result, err: err}
	}()
	writer := target.Writer.(*observedStreamWriter)
	select {
	case <-writer.first:
		// The first event reached the downstream before the upstream completed.
	case <-time.After(time.Second):
		t.Fatal("first SSE event was buffered instead of streamed")
	}
	close(release)
	select {
	case completed := <-done:
		if completed.err != nil {
			t.Fatal(completed.err)
		}
		if completed.result.Usage.TotalTokens != 5 || !target.Started {
			t.Fatalf("stream result = %#v target=%#v", completed.result, target)
		}
		if !strings.Contains(writer.body.String(), "response.completed") {
			t.Fatalf("downstream SSE body = %q", writer.body.String())
		}
	case <-time.After(time.Second):
		t.Fatal("stream did not complete")
	}
}

func TestInProcessUpstreamLeaseEnforcesCapacity(t *testing.T) {
	app := &App{store: &Store{}, upstreamLimit: 1, gatewayInstanceID: "test"}
	firstLease, acquired, err := app.acquireGatewayUpstreamLease(context.Background(), "up_1", "usr_1", "req_1")
	if err != nil || !acquired {
		t.Fatalf("first lease acquired=%v err=%v", acquired, err)
	}
	_, acquired, err = app.acquireGatewayUpstreamLease(context.Background(), "up_1", "usr_2", "req_2")
	if err != nil || acquired {
		t.Fatalf("second lease acquired=%v err=%v", acquired, err)
	}
	app.maintainGatewayUpstreamLease("up_1", "usr_1", firstLease)()
	_, acquired, err = app.acquireGatewayUpstreamLease(context.Background(), "up_1", "usr_2", "req_3")
	if err != nil || !acquired {
		t.Fatalf("lease after release acquired=%v err=%v", acquired, err)
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
		if !strings.Contains(item.Detail, "error=codex_responses_usage_missing") || !strings.Contains(item.Detail, "upstream_status=0") {
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

func TestAppInitializesOrphanUpstreamsIntoAPIKeys(t *testing.T) {
	t.Setenv("CODEXPPP_DATABASE_URL", "")
	now := time.Now().UTC().Truncate(time.Second)
	path := filepath.Join(t.TempDir(), "state.json")
	state := State{
		NextID: 10,
		UpstreamAccounts: []UpstreamAccount{{
			ID: "up_1", Name: "orphan", CredentialType: "oauth", AccessTokenCipher: "legacy-encrypted-access", TokenType: "Bearer", Status: statusActive, BalanceStatus: "available", RiskStatus: "available", CreatedAt: now, UpdatedAt: now,
		}},
	}
	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	app := &App{store: store, secretKey: deriveKey("test-secret")}
	if err := app.initializeRuntimeState(); err != nil {
		t.Fatal(err)
	}
	if len(store.state.APIKeys) != 1 || store.state.APIKeys[0].UpstreamAccountID != "up_1" || store.state.APIKeys[0].Status != statusActive {
		t.Fatalf("normalized api keys = %#v", store.state.APIKeys)
	}
	if strings.TrimSpace(store.state.APIKeys[0].KeyCipher) == "" {
		t.Fatalf("normalized api key missing encrypted raw key: %#v", store.state.APIKeys[0])
	}
	if store.state.NextID != 11 {
		t.Fatalf("next id = %d", store.state.NextID)
	}
	savedRaw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var saved State
	if err := json.Unmarshal(savedRaw, &saved); err != nil {
		t.Fatal(err)
	}
	if len(saved.APIKeys) != 1 || saved.APIKeys[0].UpstreamAccountID != "up_1" {
		t.Fatalf("saved normalized api keys = %#v", saved.APIKeys)
	}
}

func TestAppMigratesLegacyBoundRouteKeyToClientAccessKey(t *testing.T) {
	t.Setenv("CODEXPPP_DATABASE_URL", "")
	now := time.Now().UTC().Truncate(time.Second)
	oldRaw, err := generateSub2APIKey()
	if err != nil {
		t.Fatal(err)
	}
	secretKey := deriveKey("test-secret")
	seedApp := &App{secretKey: secretKey}
	oldCipher, err := seedApp.encrypt(oldRaw)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "state.json")
	state := State{
		NextID: 10,
		Users: []User{{
			ID: "usr_1", Account: "legacy-user", Status: statusActive, TokenBalance: 1000, CreatedAt: now, UpdatedAt: now,
		}},
		UpstreamAccounts: []UpstreamAccount{{
			ID: "up_2", Name: "legacy-upstream", CredentialType: "oauth", SourceType: "legacy", AuthorizationStatus: upstreamAuthAuthorized, AccessTokenCipher: "encrypted-access", Status: statusActive, BalanceStatus: "available", RiskStatus: "available", CreatedAt: now, UpdatedAt: now,
		}},
		APIKeys: []APIKey{{
			ID: "key_3", KeyCipher: oldCipher, KeyHash: hashString(oldRaw), PublicPrefix: oldRaw[:10], UpstreamAccountID: "up_2", UserID: "usr_1", Status: statusActive, CreatedAt: now, UpdatedAt: now,
		}},
	}
	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	app := &App{store: store, secretKey: secretKey}
	if err := app.initializeRuntimeState(); err != nil {
		t.Fatal(err)
	}
	if len(store.state.ClientAccessKeys) != 1 {
		t.Fatalf("migrated client access keys = %#v", store.state.ClientAccessKeys)
	}
	clientKey := store.state.ClientAccessKeys[0]
	if clientKey.UserID != "usr_1" || clientKey.KeyHash != hashString(oldRaw) {
		t.Fatalf("legacy client credential was not preserved: %#v", clientKey)
	}
	if len(store.state.APIKeys) != 1 || store.state.APIKeys[0].UserID != "" || store.state.APIKeys[0].KeyHash == hashString(oldRaw) {
		t.Fatalf("legacy route key was not detached and rotated: %#v", store.state.APIKeys)
	}
	if clientKey.ID != "cak_10" || store.state.NextID != 11 {
		t.Fatalf("migrated client access key id=%q next id=%d", clientKey.ID, store.state.NextID)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/codex/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+oldRaw)
	user, _, authenticatedKey, ok := app.codexClientFromRequest(req)
	if !ok || user.ID != "usr_1" || authenticatedKey.ID != clientKey.ID {
		t.Fatalf("legacy client credential no longer authenticates: user=%#v key=%#v ok=%v", user, authenticatedKey, ok)
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
		Users: []User{{
			ID: "usr_2", Account: "postgres-user", PasswordSalt: "salt", PasswordHash: "hash", Status: statusActive, TokenBalance: 1000, CreatedAt: now, UpdatedAt: now,
		}},
		TokenTopups: []TokenTopup{{
			ID: "topup_1", Name: "100K token", PriceCents: 990, Tokens: 100000, Enabled: true, Sort: 10, Description: "test", CreatedAt: now, UpdatedAt: now,
		}},
		UpstreamAccounts: []UpstreamAccount{{
			ID: "up_1", Name: "pending@example.com", Group: "default", CredentialType: "email_password", SourceType: "email_password", AuthorizationStatus: upstreamAuthPending, PasswordCipher: "encrypted-password", Email: "pending@example.com", EntitlementStatus: upstreamAuthPending, Status: statusDisabled, BalanceStatus: "unavailable", RiskStatus: "unavailable", CredentialFingerprint: "fingerprint", CreatedAt: now, UpdatedAt: now,
		}},
		ClientAccessKeys: []ClientAccessKey{{
			ID: "cak_3", KeyCipher: "encrypted-client-key", KeyHash: "client-key-hash", PublicPrefix: "sk-client-", UserID: "usr_2", Status: statusActive, CreatedAt: now, UpdatedAt: now,
		}},
		UsageRecords: []UsageRecord{{
			ID: "use_4", UserID: "usr_2", UpstreamAccountID: "up_1", APIKeyID: "key-route", ClientAccessKeyID: "cak_3", SessionID: "session-1", Model: "codex", InputTokens: 3, OutputTokens: 2, TotalTokens: 5, CreatedAt: now,
		}},
		GatewaySessions: []GatewaySession{{
			UserID: "usr_2", SessionKey: hashString("session-1"), UpstreamAccountID: "up_1", ExpiresAt: now.Add(24 * time.Hour), UpdatedAt: now,
		}},
	}
	store.mu.Unlock()
	if err := store.save(); err != nil {
		t.Fatal(err)
	}
	if store.db != nil {
		_ = store.Close()
	}

	reopened, err := OpenStore(filepath.Join(t.TempDir(), "unused-2.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if reopened.state.NextID != 42 {
		t.Fatalf("next id = %d", reopened.state.NextID)
	}
	if len(reopened.state.Admins) != 1 || reopened.state.Admins[0].Account != "root" {
		t.Fatalf("admins not loaded from postgres: %#v", reopened.state.Admins)
	}
	if len(reopened.state.TokenTopups) != 1 || reopened.state.TokenTopups[0].Tokens != 100000 {
		t.Fatalf("topups not loaded from postgres: %#v", reopened.state.TokenTopups)
	}
	if len(reopened.state.UpstreamAccounts) != 1 {
		t.Fatalf("upstreams not loaded from postgres: %#v", reopened.state.UpstreamAccounts)
	}
	up := reopened.state.UpstreamAccounts[0]
	if up.SourceType != "email_password" || up.AuthorizationStatus != upstreamAuthPending || up.PasswordCipher != "encrypted-password" || up.Email != "pending@example.com" || up.Status != statusDisabled {
		t.Fatalf("pending upstream round trip = %#v", up)
	}
	if len(reopened.state.ClientAccessKeys) != 1 || reopened.state.ClientAccessKeys[0].UserID != "usr_2" {
		t.Fatalf("client access keys not loaded from postgres: %#v", reopened.state.ClientAccessKeys)
	}
	if len(reopened.state.UsageRecords) != 1 || reopened.state.UsageRecords[0].ClientAccessKeyID != "cak_3" {
		t.Fatalf("usage client access key not loaded from postgres: %#v", reopened.state.UsageRecords)
	}
	if len(reopened.state.GatewaySessions) != 1 || reopened.state.GatewaySessions[0].UpstreamAccountID != "up_1" {
		t.Fatalf("gateway session affinity not loaded from postgres: %#v", reopened.state.GatewaySessions)
	}
}

func TestPostgresGatewaySettlementIsAtomic(t *testing.T) {
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
		NextID:           10,
		Users:            []User{{ID: "usr_tx", Account: "tx-user", Status: statusActive, TokenBalance: 1000, CreatedAt: now, UpdatedAt: now}},
		UpstreamAccounts: []UpstreamAccount{{ID: "up_tx", Name: "tx-upstream", AuthorizationStatus: upstreamAuthAuthorized, Status: statusActive, BalanceStatus: "available", RiskStatus: "available", CreatedAt: now, UpdatedAt: now}},
		TokenTopups:      []TokenTopup{{ID: "topup_tx", Name: "test", Enabled: true, CreatedAt: now, UpdatedAt: now}},
	}
	if err := store.save(); err != nil {
		store.mu.Unlock()
		t.Fatal(err)
	}
	request := GatewayRequest{ID: store.nextID("gw"), UserID: "usr_tx", RequestID: "atomic-request", Status: gatewayReserved, ReservedTokens: 100, CreatedAt: now, UpdatedAt: now}
	store.state.GatewayRequests = append(store.state.GatewayRequests, request)
	if err := store.saveGatewayReservation(context.Background(), request); err != nil {
		store.mu.Unlock()
		t.Fatal(err)
	}
	usage := UsageRecord{ID: store.nextID("use"), UserID: "usr_tx", UpstreamAccountID: "up_tx", SessionID: "session", Model: "gpt-5.5", InputTokens: 3, OutputTokens: 2, TotalTokens: 5, CreatedAt: now.Add(time.Second)}
	ledger := TokenLedger{ID: store.nextID("led"), UserID: "usr_tx", Type: "debit", DeltaTokens: -5, BalanceAfter: 995, Source: "Codex token 用量", CreatedAt: usage.CreatedAt}
	request.Status = gatewayCompleted
	request.ReservedTokens = 0
	request.ChargedTokens = 5
	request.UsageRecordID = usage.ID
	request.UpstreamStatus = http.StatusOK
	expectedReplayBody := `{"id":"response_atomic"}`
	request.ResultBody = expectedReplayBody
	request.ResultType = "application/json"
	request.UpdatedAt = usage.CreatedAt
	settlement := gatewaySettlement{Request: request, Usage: usage, OldBalance: 1000, NewBalance: 995, Ledgers: []TokenLedger{ledger}, NextID: store.state.NextID}
	if err := store.saveGatewaySettlement(context.Background(), settlement); err != nil {
		store.mu.Unlock()
		t.Fatal(err)
	}
	// A later unrelated snapshot save must update metadata in place without
	// deleting or copying the short-lived replay body through Go memory.
	store.state.Users[0].TokenBalance = 995
	request.ResultBody = ""
	store.state.GatewayRequests[0] = request
	store.state.UsageRecords = []UsageRecord{usage}
	store.state.TokenLedgers = []TokenLedger{ledger}
	if err := store.save(); err != nil {
		store.mu.Unlock()
		t.Fatal(err)
	}
	store.mu.Unlock()
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenStore(filepath.Join(t.TempDir(), "unused-2.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if len(reopened.state.Users) != 1 || reopened.state.Users[0].TokenBalance != 995 {
		t.Fatalf("atomic balance = %#v", reopened.state.Users)
	}
	if len(reopened.state.GatewayRequests) != 1 || reopened.state.GatewayRequests[0].Status != gatewayCompleted || reopened.state.GatewayRequests[0].ChargedTokens != 5 {
		t.Fatalf("atomic request = %#v", reopened.state.GatewayRequests)
	}
	if reopened.state.GatewayRequests[0].ResultBody != "" {
		t.Fatalf("raw response was loaded into resident state: %d bytes", len(reopened.state.GatewayRequests[0].ResultBody))
	}
	replay, found, err := reopened.loadGatewayReplay(context.Background(), reopened.state.GatewayRequests[0])
	if err != nil {
		t.Fatal(err)
	}
	if !found || string(replay.Body) != expectedReplayBody {
		t.Fatalf("on-demand replay = found:%v body:%q", found, string(replay.Body))
	}
	if len(reopened.state.UsageRecords) != 1 || len(reopened.state.TokenLedgers) != 1 {
		t.Fatalf("atomic usage=%#v ledgers=%#v", reopened.state.UsageRecords, reopened.state.TokenLedgers)
	}
}

func TestPostgresGatewayPersistenceCleanupSeparatesReplayFromMetadata(t *testing.T) {
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
	defer store.Close()
	store.mu.Lock()
	store.state = State{
		NextID:      10,
		Users:       []User{{ID: "usr_cleanup", Account: "cleanup-user", Status: statusActive, TokenBalance: 1000, CreatedAt: now, UpdatedAt: now}},
		TokenTopups: []TokenTopup{{ID: "topup_cleanup", Name: "test", Enabled: true, CreatedAt: now, UpdatedAt: now}},
	}
	if err := store.save(); err != nil {
		store.mu.Unlock()
		t.Fatal(err)
	}
	store.mu.Unlock()
	insert := `INSERT INTO idempotency_records (id,user_id,request_id,status,reserved_tokens,charged_tokens,usage_record_id,upstream_status,error,result_text,result_body,result_type,result_headers,created_at,updated_at) VALUES ($1,$2,$3,$4,0,0,'',200,'','','body','application/json','',$5,$5)`
	if _, err := store.db.ExecContext(context.Background(), insert, "gw_old_body", "usr_cleanup", "old-body", gatewayCompleted, now.Add(-gatewayReplayBodyRetention-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(context.Background(), insert, "gw_fresh_body", "usr_cleanup", "fresh-body", gatewayCompleted, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(context.Background(), insert, "gw_old_metadata", "usr_cleanup", "old-metadata", gatewayFailed, now.Add(-gatewayIdempotencyRetention-time.Minute)); err != nil {
		t.Fatal(err)
	}
	cleared, deleted, err := store.pruneGatewayPersistence(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if cleared != 1 || deleted != 1 {
		t.Fatalf("cleanup counts = cleared:%d deleted:%d", cleared, deleted)
	}
	var oldBody, freshBody string
	if err := store.db.QueryRowContext(context.Background(), `SELECT result_body FROM idempotency_records WHERE request_id='old-body'`).Scan(&oldBody); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRowContext(context.Background(), `SELECT result_body FROM idempotency_records WHERE request_id='fresh-body'`).Scan(&freshBody); err != nil {
		t.Fatal(err)
	}
	if oldBody != "" || freshBody != "body" {
		t.Fatalf("cleanup bodies = old:%q fresh:%q", oldBody, freshBody)
	}
	var staleCount int
	if err := store.db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM idempotency_records WHERE request_id='old-metadata'`).Scan(&staleCount); err != nil {
		t.Fatal(err)
	}
	if staleCount != 0 {
		t.Fatalf("expired metadata count = %d", staleCount)
	}
}

func TestPostgresRejectsConcurrentSnapshotWriters(t *testing.T) {
	databaseURL := os.Getenv("CODEXPPP_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("CODEXPPP_TEST_DATABASE_URL is not set")
	}
	t.Setenv("CODEXPPP_DATABASE_URL", databaseURL)
	first, err := OpenStore(filepath.Join(t.TempDir(), "unused.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := OpenStore(filepath.Join(t.TempDir(), "unused-2.json"))
	if second != nil {
		_ = second.Close()
	}
	if err == nil || !strings.Contains(err.Error(), "already connected") {
		t.Fatalf("second writer error = %v", err)
	}
}

func TestRedisGatewayRuntimeCoordinatesLeasesAndLocks(t *testing.T) {
	addr := os.Getenv("CODEXPPP_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("CODEXPPP_TEST_REDIS_ADDR is not set")
	}
	t.Setenv("CODEXPPP_REDIS_ADDR", addr)
	t.Setenv("CODEXPPP_REDIS_PASSWORD", "")
	t.Setenv("CODEXPPP_REDIS_DB", "0")
	runtime, err := redisRateLimiterFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.client.Close()
	suffix := randomToken(8)
	upstreamID := "test-" + suffix
	defer runtime.client.Del(context.Background(), "codexppp:gateway:upstream:"+upstreamID, "codexppp:gateway:session:"+suffix, "codexppp:lock:test-"+suffix)
	if ok, err := runtime.AcquireUpstream(context.Background(), upstreamID, "lease-1", 1, time.Minute); err != nil || !ok {
		t.Fatalf("first Redis lease ok=%v err=%v", ok, err)
	}
	if ok, err := runtime.AcquireUpstream(context.Background(), upstreamID, "lease-2", 1, time.Minute); err != nil || ok {
		t.Fatalf("second Redis lease ok=%v err=%v", ok, err)
	}
	if err := runtime.ReleaseUpstream(context.Background(), upstreamID, "lease-1"); err != nil {
		t.Fatal(err)
	}
	if ok, err := runtime.AcquireUpstream(context.Background(), upstreamID, "lease-2", 1, time.Minute); err != nil || !ok {
		t.Fatalf("Redis lease after release ok=%v err=%v", ok, err)
	}
	if err := runtime.RememberSessionRoute(context.Background(), suffix, "upstream", time.Minute); err != nil {
		t.Fatal(err)
	}
	if route, err := runtime.SessionRoute(context.Background(), suffix); err != nil || route != "upstream" {
		t.Fatalf("Redis session route=%q err=%v", route, err)
	}
	if ok, err := runtime.AcquireLock(context.Background(), "test-"+suffix, "owner-1", time.Minute); err != nil || !ok {
		t.Fatalf("first Redis lock ok=%v err=%v", ok, err)
	}
	if ok, err := runtime.AcquireLock(context.Background(), "test-"+suffix, "owner-2", time.Minute); err != nil || ok {
		t.Fatalf("second Redis lock ok=%v err=%v", ok, err)
	}
	if err := runtime.ReleaseLock(context.Background(), "test-"+suffix, "owner-1"); err != nil {
		t.Fatal(err)
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
