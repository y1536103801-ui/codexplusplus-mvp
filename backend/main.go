package main

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"
)

//go:embed web/admin/*
var adminFiles embed.FS

//go:embed migrations/*.sql
var migrationFiles embed.FS

const defaultDevSecret = "codexppp-dev-secret-change-me"
const defaultListenAddr = "127.0.0.1:8787"
const defaultGatewayRateLimitPerMinute int64 = 120
const maxJSONBodyBytes int64 = 1 << 20
const maxGatewayBodyBytes int64 = 4 << 20

const (
	statusActive   = "active"
	statusDisabled = "disabled"

	codexDefaultModel = "gpt-5.5"

	sessionRoleAdmin         = "admin"
	sessionRoleClient        = "client"
	sessionRoleCodexProvider = "codex"
	codexProviderKeyPrefix   = "sk-"
	sub2APIKeyRandomBytes    = 32

	rechargePending   = "pending"
	rechargeApproved  = "approved"
	rechargeRejected  = "rejected"
	rechargeCancelled = "cancelled"

	gatewayReserved  = "reserved"
	gatewayCompleted = "completed"
	gatewayFailed    = "failed"
)

type App struct {
	store             *Store
	secretKey         []byte
	corsOrigins       map[string]struct{}
	gatewayLimiter    GatewayRateLimiter
	gatewayRateLimit  int64
	gatewayRateWindow time.Duration
}

type GatewayRateLimiter interface {
	Allow(ctx context.Context, key string, limit int64, window time.Duration) (bool, error)
}

type codexProbeResult struct {
	AccountType          string
	Email                string
	PlanType             string
	RateLimitReachedType string
	UsageTokens          int64
	RateLimitUsedPercent *float64
	RateLimitResetsAt    *time.Time
	CreditBalance        *float64
	CreditBalanceLabel   string
}

type codexProbeCredentials struct {
	AccessToken      string
	ChatGPTAccountID string
	ChatGPTPlanType  string
}

type codexRunResult struct {
	Model  string
	Text   string
	Usage  gatewayUsage
	Status int
}

var codexAppServerProbe = runCodexAppServerProbe
var codexAppServerRun = runCodexAppServerTurn

type Store struct {
	mu          sync.Mutex
	path        string
	databaseURL string
	db          *sql.DB
	state       State
}

type State struct {
	NextID           int64             `json:"nextId"`
	Admins           []Admin           `json:"admins"`
	Users            []User            `json:"users"`
	Devices          []Device          `json:"devices"`
	TokenTopups      []TokenTopup      `json:"tokenTopups"`
	RechargeRequests []RechargeRequest `json:"rechargeRequests"`
	TokenLedgers     []TokenLedger     `json:"tokenLedgers"`
	UpstreamAccounts []UpstreamAccount `json:"upstreamAccounts"`
	APIKeys          []APIKey          `json:"apiKeys"`
	UsageRecords     []UsageRecord     `json:"usageRecords"`
	GatewayRequests  []GatewayRequest  `json:"gatewayRequests"`
	AuditLogs        []AuditLog        `json:"auditLogs"`
	Sessions         []Session         `json:"sessions"`
}

type Admin struct {
	ID                 string    `json:"id"`
	Account            string    `json:"account"`
	PasswordSalt       string    `json:"passwordSalt"`
	PasswordHash       string    `json:"passwordHash"`
	MustChangePassword bool      `json:"mustChangePassword"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

type User struct {
	ID           string     `json:"id"`
	Account      string     `json:"account"`
	PasswordSalt string     `json:"passwordSalt"`
	PasswordHash string     `json:"passwordHash"`
	Status       string     `json:"status"`
	TokenBalance int64      `json:"tokenBalance"`
	LastLoginAt  *time.Time `json:"lastLoginAt,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

type Device struct {
	ID          string     `json:"id"`
	UserID      string     `json:"userId"`
	Name        string     `json:"name"`
	Fingerprint string     `json:"fingerprint"`
	Status      string     `json:"status"`
	LastSeenAt  *time.Time `json:"lastSeenAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

type TokenTopup struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	PriceCents  int64     `json:"priceCents"`
	Tokens      int64     `json:"tokens"`
	Enabled     bool      `json:"enabled"`
	Sort        int       `json:"sort"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type RechargeRequest struct {
	ID                string                     `json:"id"`
	UserID            string                     `json:"userId"`
	TopupID           string                     `json:"topupId"`
	TopupName         string                     `json:"topupName"`
	PriceCents        int64                      `json:"priceCents"`
	Tokens            int64                      `json:"tokens"`
	Status            string                     `json:"status"`
	StatusTransitions []RechargeStatusTransition `json:"statusTransitions,omitempty"`
	SubmittedAt       time.Time                  `json:"submittedAt"`
	ConfirmedAt       *time.Time                 `json:"confirmedAt,omitempty"`
	UpdatedAt         time.Time                  `json:"updatedAt"`
}

type RechargeStatusTransition struct {
	Status    string    `json:"status"`
	At        time.Time `json:"at"`
	Action    string    `json:"action"`
	ActorRole string    `json:"actorRole"`
}

type TokenLedger struct {
	ID           string    `json:"id"`
	UserID       string    `json:"userId"`
	Type         string    `json:"type"`
	DeltaTokens  int64     `json:"deltaTokens"`
	BalanceAfter int64     `json:"balanceAfter"`
	Source       string    `json:"source"`
	CreatedAt    time.Time `json:"createdAt"`
}

type UpstreamAccount struct {
	ID                    string     `json:"id"`
	Name                  string     `json:"name"`
	Group                 string     `json:"group"`
	CredentialType        string     `json:"credentialType"`
	AccessTokenCipher     string     `json:"accessTokenCipher,omitempty"`
	RefreshTokenCipher    string     `json:"refreshTokenCipher,omitempty"`
	TokenType             string     `json:"tokenType,omitempty"`
	ChatGPTAccountID      string     `json:"chatgptAccountId,omitempty"`
	ExpiresAt             *time.Time `json:"expiresAt,omitempty"`
	Email                 string     `json:"email,omitempty"`
	SubscriptionTier      string     `json:"subscriptionTier,omitempty"`
	EntitlementStatus     string     `json:"entitlementStatus,omitempty"`
	Status                string     `json:"status"`
	BalanceStatus         string     `json:"balanceStatus"`
	RiskStatus            string     `json:"riskStatus"`
	UsageTokens           int64      `json:"usageTokens,omitempty"`
	RateLimitUsedPercent  *float64   `json:"rateLimitUsedPercent,omitempty"`
	RateLimitResetsAt     *time.Time `json:"rateLimitResetsAt,omitempty"`
	CreditBalance         *float64   `json:"creditBalance,omitempty"`
	CreditBalanceLabel    string     `json:"creditBalanceLabel,omitempty"`
	LastCheckedAt         *time.Time `json:"lastCheckedAt,omitempty"`
	CredentialFingerprint string     `json:"credentialFingerprint,omitempty"`
	CreatedAt             time.Time  `json:"createdAt"`
	UpdatedAt             time.Time  `json:"updatedAt"`
}

type APIKey struct {
	ID                string     `json:"id"`
	KeyHash           string     `json:"keyHash"`
	PublicPrefix      string     `json:"publicPrefix"`
	UpstreamAccountID string     `json:"upstreamAccountId"`
	Status            string     `json:"status"`
	LastUsedAt        *time.Time `json:"lastUsedAt,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

type UsageRecord struct {
	ID                string    `json:"id"`
	UserID            string    `json:"userId"`
	Model             string    `json:"model"`
	InputTokens       int64     `json:"inputTokens"`
	CachedInputTokens int64     `json:"cachedInputTokens"`
	OutputTokens      int64     `json:"outputTokens"`
	TotalTokens       int64     `json:"totalTokens"`
	CreatedAt         time.Time `json:"createdAt"`
}

type GatewayRequest struct {
	ID             string    `json:"id"`
	UserID         string    `json:"userId"`
	RequestID      string    `json:"requestId"`
	Status         string    `json:"status"`
	ReservedTokens int64     `json:"reservedTokens"`
	ChargedTokens  int64     `json:"chargedTokens"`
	UsageRecordID  string    `json:"usageRecordId,omitempty"`
	UpstreamStatus int       `json:"upstreamStatus,omitempty"`
	Error          string    `json:"error,omitempty"`
	ResultText     string    `json:"resultText,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type AuditLog struct {
	ID        string    `json:"id"`
	ActorID   string    `json:"actorId"`
	ActorRole string    `json:"actorRole"`
	Action    string    `json:"action"`
	TargetID  string    `json:"targetId,omitempty"`
	Detail    string    `json:"detail,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type Session struct {
	Token     string    `json:"token"`
	Role      string    `json:"role"`
	SubjectID string    `json:"subjectId"`
	DeviceID  string    `json:"deviceId,omitempty"`
	ExpiresAt time.Time `json:"expiresAt"`
	CreatedAt time.Time `json:"createdAt"`
}

type apiError struct {
	Error string `json:"error"`
}

func main() {
	dataPath := env("CODEXPPP_DATA", filepath.Join("data", "codexppp.json"))
	store, err := OpenStore(dataPath)
	if err != nil {
		log.Fatal(err)
	}
	secret, err := backendSecret(os.Getenv("CODEXPPP_SECRET"), os.Getenv("CODEXPPP_DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	limiter, err := redisRateLimiterFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	rateLimit, err := gatewayRateLimitFromEnv(os.Getenv("CODEXPPP_GATEWAY_RATE_LIMIT_PER_MINUTE"))
	if err != nil {
		log.Fatal(err)
	}
	corsOrigins, err := corsOriginsFromEnv(os.Getenv("CODEXPPP_CLIENT_ORIGINS"))
	if err != nil {
		log.Fatal(err)
	}
	app := &App{
		store:             store,
		secretKey:         deriveKey(secret),
		corsOrigins:       corsOrigins,
		gatewayLimiter:    limiter,
		gatewayRateLimit:  rateLimit,
		gatewayRateWindow: time.Minute,
	}

	addr := listenAddrFromEnv(os.Getenv("CODEXPPP_ADDR"))
	log.Printf("Codex+++ backend listening on %s", listenDisplayURL(addr))
	log.Fatal(http.ListenAndServe(addr, app.routes()))
}

func (a *App) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", a.adminStatic)
	mux.HandleFunc("/admin", a.adminStatic)
	mux.HandleFunc("/admin/", a.adminStatic)
	mux.HandleFunc("/api/health", a.healthAPI)
	mux.HandleFunc("/api/admin/", a.adminAPI)
	mux.HandleFunc("/api/client/", a.clientAPI)
	mux.HandleFunc("/api/codex/", a.codexAPI)
	return withCORS(mux, a.corsOrigins)
}

func (a *App) healthAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusNotFound, "not_found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func withCORS(next http.Handler, allowedOrigins map[string]struct{}) http.Handler {
	if len(allowedOrigins) == 0 {
		allowedOrigins, _ = corsOriginsFromEnv("")
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin != "" {
			if allowedOrigin, ok := allowedCORSOrigin(r, allowedOrigins, origin); ok {
				writeCORSHeaders(w, allowedOrigin)
				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusNoContent)
					return
				}
			} else if strings.HasPrefix(r.URL.Path, "/api/") {
				writeJSON(w, http.StatusForbidden, apiError{Error: "origin_not_allowed"})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func writeCORSHeaders(w http.ResponseWriter, origin string) {
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Add("Vary", "Origin")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key, X-Request-ID, X-Request-Id")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
	w.Header().Set("Access-Control-Max-Age", "600")
}

func (a *App) adminStatic(w http.ResponseWriter, r *http.Request) {
	path := "web/admin/index.html"
	if r.URL.Path != "/" && r.URL.Path != "/admin" && r.URL.Path != "/admin/" {
		candidate := strings.TrimPrefix(r.URL.Path, "/admin/")
		if candidate != "" && !strings.Contains(candidate, "..") {
			path = "web/admin/" + candidate
		}
	}
	data, err := adminFiles.ReadFile(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if ext := filepath.Ext(path); ext != "" {
		w.Header().Set("Content-Type", mime.TypeByExtension(ext))
	}
	if strings.HasSuffix(path, ".html") {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	}
	_, _ = w.Write(data)
}

func OpenStore(path string) (*Store, error) {
	s := &Store{path: path, databaseURL: os.Getenv("CODEXPPP_DATABASE_URL")}
	if s.databaseURL != "" {
		if err := s.openPostgres(); err != nil {
			return nil, err
		}
		return s, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		s.state.NextID = 1
		s.seedDefaultsLocked()
		return s.saveLocked()
	}
	if err != nil {
		return nil, err
	}
	if len(b) > 0 {
		if err := json.Unmarshal(b, &s.state); err != nil {
			return nil, err
		}
	}
	if s.state.NextID == 0 {
		s.state.NextID = 1
	}
	s.normalizeRechargeTransitions()
	return s, nil
}

func (s *Store) seedDefaultsLocked() {
	now := time.Now().UTC()
	s.state.TokenTopups = []TokenTopup{
		{ID: "topup_free_view", Name: "free", PriceCents: 0, Tokens: 0, Enabled: true, Sort: 1, Description: "仅供查看，不提供申请动作", CreatedAt: now, UpdatedAt: now},
		{ID: "topup_100k", Name: "100K token", PriceCents: 990, Tokens: 100000, Enabled: true, Sort: 10, Description: "首版默认充值项", CreatedAt: now, UpdatedAt: now},
	}
}

func (s *Store) saveLocked() (*Store, error) {
	if err := s.save(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) save() error {
	if s.db != nil {
		return s.savePostgres(context.Background())
	}
	tmp := s.path + ".tmp"
	b, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, b, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *Store) openPostgres() error {
	db, err := sql.Open("pgx", s.databaseURL)
	if err != nil {
		return err
	}
	s.db = db
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return err
	}
	if err := s.applyMigrations(ctx); err != nil {
		return err
	}
	if err := s.loadPostgres(ctx); err != nil {
		return err
	}
	if s.state.NextID == 0 {
		s.state.NextID = 1
	}
	s.normalizeRechargeTransitions()
	if len(s.state.TokenTopups) == 0 {
		s.seedDefaultsLocked()
		return s.savePostgres(ctx)
	}
	return nil
}

func (s *Store) applyMigrations(ctx context.Context) error {
	paths, err := fs.Glob(migrationFiles, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(paths)
	for _, path := range paths {
		sqlBytes, err := migrationFiles.ReadFile(path)
		if err != nil {
			return err
		}
		if _, err := s.db.ExecContext(ctx, string(sqlBytes)); err != nil {
			return fmt.Errorf("apply migration %s: %w", path, err)
		}
	}
	return nil
}

func (s *Store) loadPostgres(ctx context.Context) error {
	var state State
	if err := scanRows(ctx, s.db, `SELECT id, account, password_salt, password_hash, must_change_password, created_at, updated_at FROM admins ORDER BY created_at`, func(rows *sql.Rows) error {
		var item Admin
		if err := rows.Scan(&item.ID, &item.Account, &item.PasswordSalt, &item.PasswordHash, &item.MustChangePassword, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return err
		}
		state.Admins = append(state.Admins, item)
		return nil
	}); err != nil {
		return err
	}
	if err := scanRows(ctx, s.db, `SELECT id, account, password_salt, password_hash, status, token_balance, last_login_at, created_at, updated_at FROM users ORDER BY created_at`, func(rows *sql.Rows) error {
		var item User
		var lastLogin sql.NullTime
		if err := rows.Scan(&item.ID, &item.Account, &item.PasswordSalt, &item.PasswordHash, &item.Status, &item.TokenBalance, &lastLogin, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return err
		}
		item.LastLoginAt = nullableTimePtr(lastLogin)
		state.Users = append(state.Users, item)
		return nil
	}); err != nil {
		return err
	}
	if err := scanRows(ctx, s.db, `SELECT id, user_id, name, fingerprint, status, last_seen_at, created_at, updated_at FROM devices ORDER BY created_at`, func(rows *sql.Rows) error {
		var item Device
		var lastSeen sql.NullTime
		if err := rows.Scan(&item.ID, &item.UserID, &item.Name, &item.Fingerprint, &item.Status, &lastSeen, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return err
		}
		item.LastSeenAt = nullableTimePtr(lastSeen)
		state.Devices = append(state.Devices, item)
		return nil
	}); err != nil {
		return err
	}
	if err := scanRows(ctx, s.db, `SELECT id, name, price_cents, tokens, enabled, sort_order, description, created_at, updated_at FROM token_topups ORDER BY sort_order, created_at`, func(rows *sql.Rows) error {
		var item TokenTopup
		if err := rows.Scan(&item.ID, &item.Name, &item.PriceCents, &item.Tokens, &item.Enabled, &item.Sort, &item.Description, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return err
		}
		state.TokenTopups = append(state.TokenTopups, item)
		return nil
	}); err != nil {
		return err
	}
	if err := scanRows(ctx, s.db, `SELECT id, user_id, topup_id, topup_name, price_cents, tokens, status, status_transitions, submitted_at, confirmed_at, updated_at FROM recharge_requests ORDER BY submitted_at`, func(rows *sql.Rows) error {
		var item RechargeRequest
		var confirmed sql.NullTime
		var transitions []byte
		if err := rows.Scan(&item.ID, &item.UserID, &item.TopupID, &item.TopupName, &item.PriceCents, &item.Tokens, &item.Status, &transitions, &item.SubmittedAt, &confirmed, &item.UpdatedAt); err != nil {
			return err
		}
		item.ConfirmedAt = nullableTimePtr(confirmed)
		parsed, err := parseRechargeTransitions(transitions, item)
		if err != nil {
			return err
		}
		item.StatusTransitions = parsed
		state.RechargeRequests = append(state.RechargeRequests, item)
		return nil
	}); err != nil {
		return err
	}
	if err := scanRows(ctx, s.db, `SELECT id, user_id, type, delta_tokens, balance_after, source, created_at FROM token_ledgers ORDER BY created_at`, func(rows *sql.Rows) error {
		var item TokenLedger
		if err := rows.Scan(&item.ID, &item.UserID, &item.Type, &item.DeltaTokens, &item.BalanceAfter, &item.Source, &item.CreatedAt); err != nil {
			return err
		}
		state.TokenLedgers = append(state.TokenLedgers, item)
		return nil
	}); err != nil {
		return err
	}
	if err := scanRows(ctx, s.db, `SELECT id, name, account_group, credential_type, access_token_cipher, refresh_token_cipher, token_type, chatgpt_account_id, expires_at, email, subscription_tier, entitlement_status, status, balance_status, risk_status, usage_tokens, rate_limit_used_percent, rate_limit_resets_at, credit_balance, credit_balance_label, last_checked_at, credential_fingerprint, created_at, updated_at FROM upstream_accounts ORDER BY created_at`, func(rows *sql.Rows) error {
		var item UpstreamAccount
		var expires, rateLimitResetsAt, lastChecked sql.NullTime
		var rateLimitUsedPercent, creditBalance sql.NullFloat64
		if err := rows.Scan(&item.ID, &item.Name, &item.Group, &item.CredentialType, &item.AccessTokenCipher, &item.RefreshTokenCipher, &item.TokenType, &item.ChatGPTAccountID, &expires, &item.Email, &item.SubscriptionTier, &item.EntitlementStatus, &item.Status, &item.BalanceStatus, &item.RiskStatus, &item.UsageTokens, &rateLimitUsedPercent, &rateLimitResetsAt, &creditBalance, &item.CreditBalanceLabel, &lastChecked, &item.CredentialFingerprint, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return err
		}
		item.ExpiresAt = nullableTimePtr(expires)
		item.RateLimitUsedPercent = nullableFloatPtr(rateLimitUsedPercent)
		item.RateLimitResetsAt = nullableTimePtr(rateLimitResetsAt)
		item.CreditBalance = nullableFloatPtr(creditBalance)
		item.LastCheckedAt = nullableTimePtr(lastChecked)
		state.UpstreamAccounts = append(state.UpstreamAccounts, item)
		return nil
	}); err != nil {
		return err
	}
	if err := scanRows(ctx, s.db, `SELECT id, key_hash, public_prefix, upstream_account_id, status, last_used_at, created_at, updated_at FROM api_keys ORDER BY created_at`, func(rows *sql.Rows) error {
		var item APIKey
		var lastUsed sql.NullTime
		if err := rows.Scan(&item.ID, &item.KeyHash, &item.PublicPrefix, &item.UpstreamAccountID, &item.Status, &lastUsed, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return err
		}
		item.LastUsedAt = nullableTimePtr(lastUsed)
		state.APIKeys = append(state.APIKeys, item)
		return nil
	}); err != nil {
		return err
	}
	if err := scanRows(ctx, s.db, `SELECT id, user_id, model, input_tokens, cached_input_tokens, output_tokens, total_tokens, created_at FROM usage_records ORDER BY created_at`, func(rows *sql.Rows) error {
		var item UsageRecord
		if err := rows.Scan(&item.ID, &item.UserID, &item.Model, &item.InputTokens, &item.CachedInputTokens, &item.OutputTokens, &item.TotalTokens, &item.CreatedAt); err != nil {
			return err
		}
		state.UsageRecords = append(state.UsageRecords, item)
		return nil
	}); err != nil {
		return err
	}
	if err := scanRows(ctx, s.db, `SELECT id, user_id, request_id, status, reserved_tokens, charged_tokens, usage_record_id, upstream_status, error, result_text, created_at, updated_at FROM idempotency_records ORDER BY created_at`, func(rows *sql.Rows) error {
		var item GatewayRequest
		if err := rows.Scan(&item.ID, &item.UserID, &item.RequestID, &item.Status, &item.ReservedTokens, &item.ChargedTokens, &item.UsageRecordID, &item.UpstreamStatus, &item.Error, &item.ResultText, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return err
		}
		state.GatewayRequests = append(state.GatewayRequests, item)
		return nil
	}); err != nil {
		return err
	}
	if err := scanRows(ctx, s.db, `SELECT id, actor_id, actor_role, action, target_id, detail, created_at FROM audit_logs ORDER BY created_at`, func(rows *sql.Rows) error {
		var item AuditLog
		if err := rows.Scan(&item.ID, &item.ActorID, &item.ActorRole, &item.Action, &item.TargetID, &item.Detail, &item.CreatedAt); err != nil {
			return err
		}
		state.AuditLogs = append(state.AuditLogs, item)
		return nil
	}); err != nil {
		return err
	}
	if err := scanRows(ctx, s.db, `SELECT token, role, subject_id, device_id, expires_at, created_at FROM sessions ORDER BY created_at`, func(rows *sql.Rows) error {
		var item Session
		if err := rows.Scan(&item.Token, &item.Role, &item.SubjectID, &item.DeviceID, &item.ExpiresAt, &item.CreatedAt); err != nil {
			return err
		}
		state.Sessions = append(state.Sessions, item)
		return nil
	}); err != nil {
		return err
	}

	var nextIDText string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM app_meta WHERE key = 'next_id'`).Scan(&nextIDText)
	if errors.Is(err, sql.ErrNoRows) {
		state.NextID = computeNextID(state)
	} else if err != nil {
		return err
	} else {
		nextID, err := strconv.ParseInt(nextIDText, 10, 64)
		if err != nil {
			return err
		}
		state.NextID = nextID
	}
	if state.NextID <= 0 {
		state.NextID = computeNextID(state)
	}
	s.state = state
	return nil
}

func (s *Store) savePostgres(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, table := range []string{"sessions", "audit_logs", "idempotency_records", "usage_records", "api_keys", "upstream_accounts", "token_ledgers", "recharge_requests", "devices", "users", "admins", "token_topups", "app_meta"} {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+table); err != nil {
			return err
		}
	}
	for _, item := range s.state.Admins {
		if _, err := tx.ExecContext(ctx, `INSERT INTO admins (id, account, password_salt, password_hash, must_change_password, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`, item.ID, item.Account, item.PasswordSalt, item.PasswordHash, item.MustChangePassword, item.CreatedAt, item.UpdatedAt); err != nil {
			return err
		}
	}
	for _, item := range s.state.Users {
		if _, err := tx.ExecContext(ctx, `INSERT INTO users (id, account, password_salt, password_hash, status, token_balance, last_login_at, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, item.ID, item.Account, item.PasswordSalt, item.PasswordHash, item.Status, item.TokenBalance, item.LastLoginAt, item.CreatedAt, item.UpdatedAt); err != nil {
			return err
		}
	}
	for _, item := range s.state.Devices {
		if _, err := tx.ExecContext(ctx, `INSERT INTO devices (id, user_id, name, fingerprint, status, last_seen_at, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, item.ID, item.UserID, item.Name, item.Fingerprint, item.Status, item.LastSeenAt, item.CreatedAt, item.UpdatedAt); err != nil {
			return err
		}
	}
	for _, item := range s.state.TokenTopups {
		if _, err := tx.ExecContext(ctx, `INSERT INTO token_topups (id, name, price_cents, tokens, enabled, sort_order, description, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, item.ID, item.Name, item.PriceCents, item.Tokens, item.Enabled, item.Sort, item.Description, item.CreatedAt, item.UpdatedAt); err != nil {
			return err
		}
	}
	for _, item := range s.state.RechargeRequests {
		transitions, err := json.Marshal(ensureRechargeTransitions(item))
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO recharge_requests (id, user_id, topup_id, topup_name, price_cents, tokens, status, status_transitions, submitted_at, confirmed_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9,$10,$11)`, item.ID, item.UserID, item.TopupID, item.TopupName, item.PriceCents, item.Tokens, item.Status, string(transitions), item.SubmittedAt, item.ConfirmedAt, item.UpdatedAt); err != nil {
			return err
		}
	}
	for _, item := range s.state.TokenLedgers {
		if _, err := tx.ExecContext(ctx, `INSERT INTO token_ledgers (id, user_id, type, delta_tokens, balance_after, source, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`, item.ID, item.UserID, item.Type, item.DeltaTokens, item.BalanceAfter, item.Source, item.CreatedAt); err != nil {
			return err
		}
	}
	for _, item := range s.state.UpstreamAccounts {
		if _, err := tx.ExecContext(ctx, `INSERT INTO upstream_accounts (id, name, account_group, credential_type, access_token_cipher, refresh_token_cipher, token_type, chatgpt_account_id, expires_at, email, subscription_tier, entitlement_status, status, balance_status, risk_status, usage_tokens, rate_limit_used_percent, rate_limit_resets_at, credit_balance, credit_balance_label, last_checked_at, credential_fingerprint, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24)`, item.ID, item.Name, item.Group, item.CredentialType, item.AccessTokenCipher, item.RefreshTokenCipher, item.TokenType, item.ChatGPTAccountID, item.ExpiresAt, item.Email, item.SubscriptionTier, item.EntitlementStatus, item.Status, item.BalanceStatus, item.RiskStatus, item.UsageTokens, item.RateLimitUsedPercent, item.RateLimitResetsAt, item.CreditBalance, item.CreditBalanceLabel, item.LastCheckedAt, item.CredentialFingerprint, item.CreatedAt, item.UpdatedAt); err != nil {
			return err
		}
	}
	for _, item := range s.state.APIKeys {
		if _, err := tx.ExecContext(ctx, `INSERT INTO api_keys (id, key_hash, public_prefix, upstream_account_id, status, last_used_at, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, item.ID, item.KeyHash, item.PublicPrefix, item.UpstreamAccountID, item.Status, item.LastUsedAt, item.CreatedAt, item.UpdatedAt); err != nil {
			return err
		}
	}
	for _, item := range s.state.UsageRecords {
		if _, err := tx.ExecContext(ctx, `INSERT INTO usage_records (id, user_id, model, input_tokens, cached_input_tokens, output_tokens, total_tokens, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, item.ID, item.UserID, item.Model, item.InputTokens, item.CachedInputTokens, item.OutputTokens, item.TotalTokens, item.CreatedAt); err != nil {
			return err
		}
	}
	for _, item := range s.state.GatewayRequests {
		if _, err := tx.ExecContext(ctx, `INSERT INTO idempotency_records (id, user_id, request_id, status, reserved_tokens, charged_tokens, usage_record_id, upstream_status, error, result_text, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, item.ID, item.UserID, item.RequestID, item.Status, item.ReservedTokens, item.ChargedTokens, item.UsageRecordID, item.UpstreamStatus, item.Error, item.ResultText, item.CreatedAt, item.UpdatedAt); err != nil {
			return err
		}
	}
	for _, item := range s.state.AuditLogs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO audit_logs (id, actor_id, actor_role, action, target_id, detail, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`, item.ID, item.ActorID, item.ActorRole, item.Action, item.TargetID, item.Detail, item.CreatedAt); err != nil {
			return err
		}
	}
	for _, item := range s.state.Sessions {
		if _, err := tx.ExecContext(ctx, `INSERT INTO sessions (token, role, subject_id, device_id, expires_at, created_at) VALUES ($1,$2,$3,$4,$5,$6)`, item.Token, item.Role, item.SubjectID, item.DeviceID, item.ExpiresAt, item.CreatedAt); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO app_meta (key, value) VALUES ('next_id', $1)`, strconv.FormatInt(s.state.NextID, 10)); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func scanRows(ctx context.Context, db *sql.DB, query string, scan func(*sql.Rows) error) error {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := scan(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}

func nullableTimePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}

func nullableFloatPtr(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	return &value.Float64
}

func (s *Store) normalizeRechargeTransitions() {
	for i := range s.state.RechargeRequests {
		s.state.RechargeRequests[i].StatusTransitions = ensureRechargeTransitions(s.state.RechargeRequests[i])
	}
}

func parseRechargeTransitions(raw []byte, rr RechargeRequest) ([]RechargeStatusTransition, error) {
	var transitions []RechargeStatusTransition
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &transitions); err != nil {
			return nil, err
		}
	}
	if len(transitions) == 0 {
		return ensureRechargeTransitions(rr), nil
	}
	return transitions, nil
}

func ensureRechargeTransitions(rr RechargeRequest) []RechargeStatusTransition {
	if len(rr.StatusTransitions) > 0 {
		return rr.StatusTransitions
	}
	submittedAt := rr.SubmittedAt
	if submittedAt.IsZero() {
		submittedAt = rr.UpdatedAt
	}
	if submittedAt.IsZero() {
		submittedAt = time.Now().UTC()
	}
	transitions := []RechargeStatusTransition{{
		Status:    rechargePending,
		At:        submittedAt,
		Action:    "recharge.request",
		ActorRole: "client",
	}}
	if rr.Status != "" && rr.Status != rechargePending {
		at := rr.UpdatedAt
		if rr.ConfirmedAt != nil {
			at = *rr.ConfirmedAt
		}
		if at.IsZero() {
			at = submittedAt
		}
		transitions = append(transitions, RechargeStatusTransition{
			Status:    rr.Status,
			At:        at,
			Action:    rechargeTransitionAction(rr.Status),
			ActorRole: "admin",
		})
	}
	return transitions
}

func rechargeTransitionAction(status string) string {
	switch status {
	case rechargeApproved:
		return "recharge.approve"
	case rechargeRejected:
		return "recharge.reject"
	case rechargeCancelled:
		return "recharge.cancel"
	default:
		return "recharge.status"
	}
}

func computeNextID(state State) int64 {
	var maxID int64
	collect := func(id string) {
		idx := strings.LastIndex(id, "_")
		if idx < 0 || idx == len(id)-1 {
			return
		}
		value, err := strconv.ParseInt(id[idx+1:], 10, 64)
		if err == nil && value > maxID {
			maxID = value
		}
	}
	for _, item := range state.Admins {
		collect(item.ID)
	}
	for _, item := range state.Users {
		collect(item.ID)
	}
	for _, item := range state.Devices {
		collect(item.ID)
	}
	for _, item := range state.TokenTopups {
		collect(item.ID)
	}
	for _, item := range state.RechargeRequests {
		collect(item.ID)
	}
	for _, item := range state.TokenLedgers {
		collect(item.ID)
	}
	for _, item := range state.UpstreamAccounts {
		collect(item.ID)
	}
	for _, item := range state.APIKeys {
		collect(item.ID)
	}
	for _, item := range state.UsageRecords {
		collect(item.ID)
	}
	for _, item := range state.GatewayRequests {
		collect(item.ID)
	}
	for _, item := range state.AuditLogs {
		collect(item.ID)
	}
	if maxID < 1 {
		return 1
	}
	return maxID + 1
}

func (s *Store) nextID(prefix string) string {
	id := fmt.Sprintf("%s_%d", prefix, s.state.NextID)
	s.state.NextID++
	return id
}

func (a *App) adminAPI(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/admin")
	switch {
	case path == "/bootstrap" && r.Method == http.MethodGet:
		a.adminBootstrap(w, r)
	case path == "/setup" && r.Method == http.MethodPost:
		a.adminSetup(w, r)
	case path == "/login" && r.Method == http.MethodPost:
		a.adminLogin(w, r)
	case path == "/password" && r.Method == http.MethodPost:
		a.adminPassword(w, r)
	case path == "/me" && r.Method == http.MethodGet:
		a.adminMe(w, r)
	case path == "/overview" && r.Method == http.MethodGet:
		a.adminOverview(w, r)
	case path == "/users" && r.Method == http.MethodGet:
		a.adminUsers(w, r)
	case path == "/users" && r.Method == http.MethodPost:
		a.adminCreateUser(w, r)
	case strings.HasPrefix(path, "/users/"):
		a.adminUserAction(w, r, strings.TrimPrefix(path, "/users/"))
	case path == "/topups" && r.Method == http.MethodGet:
		a.adminTopups(w, r)
	case path == "/topups" && r.Method == http.MethodPost:
		a.adminCreateTopup(w, r)
	case strings.HasPrefix(path, "/topups/"):
		a.adminTopupAction(w, r, strings.TrimPrefix(path, "/topups/"))
	case path == "/recharges" && r.Method == http.MethodGet:
		a.adminRecharges(w, r)
	case strings.HasPrefix(path, "/recharges/"):
		a.adminRechargeAction(w, r, strings.TrimPrefix(path, "/recharges/"))
	case path == "/upstreams" && r.Method == http.MethodGet:
		a.adminUpstreams(w, r)
	case path == "/upstreams" && r.Method == http.MethodPost:
		a.adminCreateUpstream(w, r)
	case path == "/upstreams/import" && r.Method == http.MethodPost:
		a.adminImportUpstreams(w, r)
	case strings.HasPrefix(path, "/upstreams/"):
		a.adminUpstreamAction(w, r, strings.TrimPrefix(path, "/upstreams/"))
	case path == "/api-keys" && r.Method == http.MethodGet:
		a.adminAPIKeys(w, r)
	case path == "/api-keys" && r.Method == http.MethodPost:
		a.adminCreateAPIKey(w, r)
	case strings.HasPrefix(path, "/api-keys/"):
		a.adminAPIKeyAction(w, r, strings.TrimPrefix(path, "/api-keys/"))
	case path == "/devices" && r.Method == http.MethodGet:
		a.adminDevices(w, r)
	case strings.HasPrefix(path, "/devices/"):
		a.adminDeviceAction(w, r, strings.TrimPrefix(path, "/devices/"))
	case path == "/usage" && r.Method == http.MethodGet:
		a.adminUsage(w, r)
	case path == "/ledger" && r.Method == http.MethodGet:
		a.adminLedger(w, r)
	case path == "/audit" && r.Method == http.MethodGet:
		a.adminAudit(w, r)
	default:
		writeErr(w, http.StatusNotFound, "not_found")
	}
}

func (a *App) clientAPI(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/client")
	switch {
	case path == "/login" && r.Method == http.MethodPost:
		a.clientLogin(w, r)
	case path == "/me" && r.Method == http.MethodGet:
		a.clientMe(w, r)
	case path == "/topups" && r.Method == http.MethodGet:
		a.clientTopups(w, r)
	case path == "/recharges" && r.Method == http.MethodPost:
		a.clientCreateRecharge(w, r)
	case path == "/recharges" && r.Method == http.MethodGet:
		a.clientRecharges(w, r)
	case path == "/usage" && r.Method == http.MethodGet:
		a.clientUsage(w, r)
	case path == "/ledger" && r.Method == http.MethodGet:
		a.clientLedger(w, r)
	case path == "/ledger/summary" && r.Method == http.MethodGet:
		a.clientLedgerSummary(w, r)
	case path == "/desktop/update" && r.Method == http.MethodGet:
		a.clientDesktopUpdate(w, r)
	case path == "/launch/prepare" && r.Method == http.MethodPost:
		a.clientLaunchPrepare(w, r)
	default:
		writeErr(w, http.StatusNotFound, "not_found")
	}
}

func (a *App) codexAPI(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/codex")
	switch {
	case (path == "/v1/models" || path == "/v1/models/") && r.Method == http.MethodGet:
		user, _, ok := a.requireCodexClient(w, r)
		if !ok {
			return
		}
		if status, code, ok := a.codexProviderAvailable(user); !ok {
			writeCodexProviderError(w, status, code)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": []map[string]any{{"id": codexDefaultModel, "object": "model"}}})
	case (path == "/v1/responses" || path == "/v1/responses/") && r.Method == http.MethodPost:
		a.codexResponses(w, r)
	default:
		writeErr(w, http.StatusNotFound, "not_found")
	}
}

func (a *App) requireCodexClient(w http.ResponseWriter, r *http.Request) (User, Device, bool) {
	user, device, ok := a.codexClientFromRequest(r)
	if !ok {
		writeCodexProviderError(w, http.StatusUnauthorized, "login_failed")
		return User{}, Device{}, false
	}
	return user, device, true
}

func (a *App) codexProviderAvailable(user User) (int, string, bool) {
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	idx := a.userIndex(user.ID)
	if idx < 0 || a.store.state.Users[idx].Status != statusActive {
		return http.StatusUnauthorized, "login_failed", false
	}
	if a.availableTokenBalanceLocked(user.ID) <= 0 {
		return http.StatusPaymentRequired, "token_not_available", false
	}
	if len(a.routeCandidatesLocked()) == 0 {
		return http.StatusServiceUnavailable, "route_unavailable", false
	}
	return http.StatusOK, "", true
}

func (a *App) adminBootstrap(w http.ResponseWriter, r *http.Request) {
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"initialized": len(a.store.state.Admins) > 0})
}

func (a *App) adminSetup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Account  string `json:"account"`
		Password string `json:"password"`
	}
	if !readStrictJSON(w, r, &req, "invalid_admin_setup_request", "account", "password") {
		return
	}
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	if len(a.store.state.Admins) > 0 {
		writeErr(w, http.StatusConflict, "already_initialized")
		return
	}
	salt, hash := makePasswordHash(req.Password)
	now := time.Now().UTC()
	admin := Admin{ID: a.store.nextID("adm"), Account: req.Account, PasswordSalt: salt, PasswordHash: hash, MustChangePassword: true, CreatedAt: now, UpdatedAt: now}
	a.store.state.Admins = append(a.store.state.Admins, admin)
	token := a.createSessionLocked(sessionRoleAdmin, admin.ID, "")
	a.auditLocked(admin.ID, "admin", "admin.setup", admin.ID, "initialized first administrator")
	if !a.saveOrErrorLocked(w) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "admin": publicAdmin(admin)})
}

func (a *App) adminLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Account  string `json:"account"`
		Password string `json:"password"`
	}
	if !readStrictJSON(w, r, &req, "invalid_admin_login_request", "account", "password") {
		return
	}
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	for _, admin := range a.store.state.Admins {
		if admin.Account == req.Account && verifyPassword(req.Password, admin.PasswordSalt, admin.PasswordHash) {
			token := a.createSessionLocked(sessionRoleAdmin, admin.ID, "")
			a.auditLocked(admin.ID, "admin", "admin.login.success", admin.ID, "")
			if !a.saveOrErrorLocked(w) {
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"token": token, "admin": publicAdmin(admin)})
			return
		}
	}
	a.auditLocked("unknown", "admin", "admin.login.failed", req.Account, "login_failed")
	if !a.saveOrErrorLocked(w) {
		return
	}
	writeErr(w, http.StatusUnauthorized, "login_failed")
}

func (a *App) adminPassword(w http.ResponseWriter, r *http.Request) {
	admin, ok := a.requireAdmin(w, r, true)
	if !ok {
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if !readStrictJSON(w, r, &req, "invalid_admin_password_request", "password") {
		return
	}
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	idx := a.adminIndex(admin.ID)
	if idx < 0 {
		writeErr(w, http.StatusUnauthorized, "login_failed")
		return
	}
	salt, hash := makePasswordHash(req.Password)
	a.store.state.Admins[idx].PasswordSalt = salt
	a.store.state.Admins[idx].PasswordHash = hash
	a.store.state.Admins[idx].MustChangePassword = false
	a.store.state.Admins[idx].UpdatedAt = time.Now().UTC()
	a.auditLocked(admin.ID, "admin", "admin.password.change", admin.ID, "")
	if !a.saveOrErrorLocked(w) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"admin": publicAdmin(a.store.state.Admins[idx])})
}

func (a *App) adminMe(w http.ResponseWriter, r *http.Request) {
	admin, ok := a.requireAdmin(w, r, true)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"admin": publicAdmin(admin)})
}

func (a *App) adminOverview(w http.ResponseWriter, r *http.Request) {
	admin, ok := a.requireAdmin(w, r, false)
	if !ok {
		return
	}
	_ = admin
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	var pending, available int
	var todayTokens int64
	start, end := utcDayBounds(time.Now())
	for _, rr := range a.store.state.RechargeRequests {
		if rr.Status == rechargePending {
			pending++
		}
	}
	for _, up := range a.store.state.UpstreamAccounts {
		if upstreamIsAvailable(up) {
			available++
		}
	}
	for _, usage := range a.store.state.UsageRecords {
		if !usage.CreatedAt.Before(start) && usage.CreatedAt.Before(end) {
			todayTokens += usage.TotalTokens
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"pendingRecharges": pending, "availableUpstreams": available, "todayTokens": todayTokens})
}

func (a *App) adminUsers(w http.ResponseWriter, r *http.Request) {
	_, ok := a.requireAdmin(w, r, false)
	if !ok {
		return
	}
	query := strings.ToLower(r.URL.Query().Get("q"))
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	items := make([]map[string]any, 0)
	for _, user := range a.store.state.Users {
		if query != "" && !strings.Contains(strings.ToLower(user.Account), query) {
			continue
		}
		items = append(items, a.publicUserLocked(user))
	}
	writeJSON(w, http.StatusOK, listPayload(items, r))
}

func (a *App) adminCreateUser(w http.ResponseWriter, r *http.Request) {
	admin, ok := a.requireAdmin(w, r, false)
	if !ok {
		return
	}
	var req struct {
		Account  string `json:"account"`
		Password string `json:"password"`
	}
	if !readStrictJSON(w, r, &req, "invalid_user_request", "account", "password") {
		return
	}
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	for _, user := range a.store.state.Users {
		if user.Account == req.Account {
			writeErr(w, http.StatusConflict, "user_exists")
			return
		}
	}
	salt, hash := makePasswordHash(req.Password)
	now := time.Now().UTC()
	user := User{ID: a.store.nextID("usr"), Account: req.Account, PasswordSalt: salt, PasswordHash: hash, Status: statusActive, CreatedAt: now, UpdatedAt: now}
	a.store.state.Users = append(a.store.state.Users, user)
	a.auditLocked(admin.ID, "admin", "user.create", user.ID, user.Account)
	if !a.saveOrErrorLocked(w) {
		return
	}
	writeJSON(w, http.StatusCreated, a.publicUserLocked(user))
}

func (a *App) adminUserAction(w http.ResponseWriter, r *http.Request, rest string) {
	admin, ok := a.requireAdmin(w, r, false)
	if !ok {
		return
	}
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeErr(w, http.StatusNotFound, "not_found")
		return
	}
	id := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	idx := a.userIndex(id)
	if idx < 0 {
		writeErr(w, http.StatusNotFound, "user_not_found")
		return
	}
	switch {
	case action == "" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, a.publicUserLocked(a.store.state.Users[idx]))
	case action == "ledger" && r.Method == http.MethodGet:
		items := make([]map[string]any, 0)
		for _, rec := range a.store.state.TokenLedgers {
			if rec.UserID == id {
				items = append(items, a.publicAdminTokenLedgerLocked(rec))
			}
		}
		sortPublicRecordsByCreatedAtDesc(items)
		writeJSON(w, http.StatusOK, listPayload(items, r))
	case action == "recharges" && r.Method == http.MethodGet:
		items := make([]map[string]any, 0)
		for _, rr := range a.store.state.RechargeRequests {
			if rr.UserID == id {
				items = append(items, a.publicAdminRechargeLocked(rr))
			}
		}
		sort.Slice(items, func(i, j int) bool {
			return items[i]["submittedAt"].(time.Time).After(items[j]["submittedAt"].(time.Time))
		})
		writeJSON(w, http.StatusOK, listPayload(items, r))
	case action == "usage" && r.Method == http.MethodGet:
		items := make([]UsageRecord, 0)
		for _, rec := range a.store.state.UsageRecords {
			if rec.UserID == id {
				items = append(items, rec)
			}
		}
		sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
		writeJSON(w, http.StatusOK, listPayload(items, r))
	case action == "status" && r.Method == http.MethodPost:
		var req struct {
			Status string `json:"status"`
		}
		if !readStrictJSONLocked(w, r, &req, "invalid_user_status_request", "status") {
			return
		}
		if req.Status != statusActive && req.Status != statusDisabled {
			writeErr(w, http.StatusBadRequest, "invalid_status")
			return
		}
		a.store.state.Users[idx].Status = req.Status
		a.store.state.Users[idx].UpdatedAt = time.Now().UTC()
		a.auditLocked(admin.ID, "admin", "user.status", id, req.Status)
		if !a.saveOrErrorLocked(w) {
			return
		}
		writeJSON(w, http.StatusOK, a.publicUserLocked(a.store.state.Users[idx]))
	case action == "password" && r.Method == http.MethodPost:
		var req struct {
			Password string `json:"password"`
		}
		if !readStrictJSONLocked(w, r, &req, "invalid_user_password_request", "password") {
			return
		}
		salt, hash := makePasswordHash(req.Password)
		a.store.state.Users[idx].PasswordSalt = salt
		a.store.state.Users[idx].PasswordHash = hash
		a.store.state.Users[idx].UpdatedAt = time.Now().UTC()
		a.auditLocked(admin.ID, "admin", "user.password.reset", id, "")
		if !a.saveOrErrorLocked(w) {
			return
		}
		writeJSON(w, http.StatusOK, a.publicUserLocked(a.store.state.Users[idx]))
	case action == "token-adjustments" && r.Method == http.MethodPost:
		var req struct {
			DeltaTokens int64  `json:"deltaTokens"`
			Remark      string `json:"remark"`
		}
		if !readStrictJSONLocked(w, r, &req, "invalid_token_adjustment_request", "deltaTokens", "remark") {
			return
		}
		if req.DeltaTokens < 0 {
			if req.DeltaTokens == -1<<63 || a.availableTokenBalanceLocked(id) < -req.DeltaTokens {
				writeErr(w, http.StatusBadRequest, "token_not_available")
				return
			}
		}
		source := "管理员调整"
		if strings.TrimSpace(req.Remark) != "" {
			source += "：" + strings.TrimSpace(req.Remark)
		}
		if err := a.applyTokenDeltaLocked(id, req.DeltaTokens, "adjustment", source); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		a.auditLocked(admin.ID, "admin", "user.token.adjust", id, fmt.Sprintf("%d", req.DeltaTokens))
		if !a.saveOrErrorLocked(w) {
			return
		}
		writeJSON(w, http.StatusOK, a.publicUserLocked(a.store.state.Users[idx]))
	default:
		writeErr(w, http.StatusNotFound, "not_found")
	}
}

func (a *App) adminTopups(w http.ResponseWriter, r *http.Request) {
	_, ok := a.requireAdmin(w, r, false)
	if !ok {
		return
	}
	query := strings.ToLower(r.URL.Query().Get("q"))
	enabled := r.URL.Query().Get("enabled")
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	items := append([]TokenTopup(nil), a.store.state.TokenTopups...)
	sort.Slice(items, func(i, j int) bool {
		if items[i].Sort == items[j].Sort {
			return items[i].CreatedAt.Before(items[j].CreatedAt)
		}
		return items[i].Sort < items[j].Sort
	})
	filtered := make([]TokenTopup, 0)
	for _, topup := range items {
		if query != "" && !strings.Contains(strings.ToLower(topup.Name), query) {
			continue
		}
		if enabled == "true" && !topup.Enabled {
			continue
		}
		if enabled == "false" && topup.Enabled {
			continue
		}
		filtered = append(filtered, topup)
	}
	writeJSON(w, http.StatusOK, listPayload(filtered, r))
}

func (a *App) adminCreateTopup(w http.ResponseWriter, r *http.Request) {
	admin, ok := a.requireAdmin(w, r, false)
	if !ok {
		return
	}
	req, ok := readAdminTopupRequest(w, r)
	if !ok {
		return
	}
	if !validTopupPayload(w, req) {
		return
	}
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	now := time.Now().UTC()
	topup := TokenTopup{ID: a.store.nextID("topup"), Name: req.Name, PriceCents: req.PriceCents, Tokens: req.Tokens, Enabled: req.Enabled, Sort: req.Sort, Description: req.Description, CreatedAt: now, UpdatedAt: now}
	a.store.state.TokenTopups = append(a.store.state.TokenTopups, topup)
	a.auditLocked(admin.ID, "admin", "topup.create", topup.ID, topup.Name)
	if !a.saveOrErrorLocked(w) {
		return
	}
	writeJSON(w, http.StatusCreated, topup)
}

func (a *App) adminTopupAction(w http.ResponseWriter, r *http.Request, rest string) {
	admin, ok := a.requireAdmin(w, r, false)
	if !ok {
		return
	}
	id := strings.Trim(rest, "/")
	if r.Method != http.MethodPatch {
		writeErr(w, http.StatusNotFound, "not_found")
		return
	}
	req, ok := readAdminTopupRequest(w, r)
	if !ok {
		return
	}
	if !validTopupPayload(w, req) {
		return
	}
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	idx := a.topupIndex(id)
	if idx < 0 {
		writeErr(w, http.StatusNotFound, "topup_not_found")
		return
	}
	topup := &a.store.state.TokenTopups[idx]
	topup.Name = req.Name
	topup.PriceCents = req.PriceCents
	topup.Tokens = req.Tokens
	topup.Enabled = req.Enabled
	topup.Sort = req.Sort
	topup.Description = req.Description
	topup.UpdatedAt = time.Now().UTC()
	a.auditLocked(admin.ID, "admin", "topup.update", id, topup.Name)
	if !a.saveOrErrorLocked(w) {
		return
	}
	writeJSON(w, http.StatusOK, topup)
}

func (a *App) adminRecharges(w http.ResponseWriter, r *http.Request) {
	_, ok := a.requireAdmin(w, r, false)
	if !ok {
		return
	}
	status := r.URL.Query().Get("status")
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	items := make([]map[string]any, 0)
	for _, rr := range a.store.state.RechargeRequests {
		if status != "" && rr.Status != status {
			continue
		}
		item := a.publicAdminRechargeLocked(rr)
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i]["submittedAt"].(time.Time).After(items[j]["submittedAt"].(time.Time))
	})
	writeJSON(w, http.StatusOK, listPayload(items, r))
}

func (a *App) adminRechargeAction(w http.ResponseWriter, r *http.Request, rest string) {
	admin, ok := a.requireAdmin(w, r, false)
	if !ok {
		return
	}
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) != 2 || r.Method != http.MethodPost {
		writeErr(w, http.StatusNotFound, "not_found")
		return
	}
	id, action := parts[0], parts[1]
	if action != "approve" && action != "reject" {
		writeErr(w, http.StatusNotFound, "not_found")
		return
	}
	if !readEmptyJSON(w, r, "invalid_recharge_action_request") {
		return
	}
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	idx := a.rechargeIndex(id)
	if idx < 0 {
		writeErr(w, http.StatusNotFound, "recharge_not_found")
		return
	}
	rr := &a.store.state.RechargeRequests[idx]
	if rr.Status != rechargePending {
		writeErr(w, http.StatusConflict, "recharge_not_pending")
		return
	}
	now := time.Now().UTC()
	rr.StatusTransitions = ensureRechargeTransitions(*rr)
	switch action {
	case "approve":
		if err := a.applyTokenDeltaLocked(rr.UserID, rr.Tokens, "recharge", "Token 充值项："+rr.TopupName); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		rr.Status = rechargeApproved
		rr.ConfirmedAt = &now
		rr.UpdatedAt = now
		rr.StatusTransitions = append(rr.StatusTransitions, RechargeStatusTransition{Status: rechargeApproved, At: now, Action: "recharge.approve", ActorRole: "admin"})
		a.auditLocked(admin.ID, "admin", "recharge.approve", id, rr.TopupName)
		a.auditLocked(admin.ID, "admin", "token.recharge.add", rr.UserID, fmt.Sprintf("recharge=%s tokens=%d", rr.ID, rr.Tokens))
	case "reject":
		rr.Status = rechargeRejected
		rr.ConfirmedAt = &now
		rr.UpdatedAt = now
		rr.StatusTransitions = append(rr.StatusTransitions, RechargeStatusTransition{Status: rechargeRejected, At: now, Action: "recharge.reject", ActorRole: "admin"})
		a.auditLocked(admin.ID, "admin", "recharge.reject", id, rr.TopupName)
	}
	if !a.saveOrErrorLocked(w) {
		return
	}
	writeJSON(w, http.StatusOK, a.publicAdminRechargeLocked(*rr))
}

func (a *App) adminUpstreams(w http.ResponseWriter, r *http.Request) {
	_, ok := a.requireAdmin(w, r, false)
	if !ok {
		return
	}
	available := r.URL.Query().Get("available")
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	items := make([]map[string]any, 0)
	for _, up := range a.store.state.UpstreamAccounts {
		isAvailable := upstreamIsAvailable(up)
		if available == "true" && !isAvailable {
			continue
		}
		items = append(items, publicUpstream(up))
	}
	writeJSON(w, http.StatusOK, listPayload(items, r))
}

func (a *App) adminCreateUpstream(w http.ResponseWriter, r *http.Request) {
	admin, ok := a.requireAdmin(w, r, false)
	if !ok {
		return
	}
	req, ok := readAdminUpstreamRequest(w, r)
	if !ok {
		return
	}
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	now := time.Now().UTC()
	up, err := a.newUpstreamFromRequestLocked(req, now)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "secret_encrypt_failed")
		return
	}
	a.store.state.UpstreamAccounts = append(a.store.state.UpstreamAccounts, up)
	a.auditLocked(admin.ID, "admin", "upstream.create", up.ID, up.Name)
	if !a.saveOrErrorLocked(w) {
		return
	}
	writeJSON(w, http.StatusCreated, publicUpstream(up))
}

func (a *App) adminImportUpstreams(w http.ResponseWriter, r *http.Request) {
	admin, ok := a.requireAdmin(w, r, false)
	if !ok {
		return
	}
	reqs, ok := readAdminUpstreamImportRequest(w, r)
	if !ok {
		return
	}
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	now := time.Now().UTC()
	created := make([]UpstreamAccount, 0, len(reqs))
	for _, req := range reqs {
		up, err := a.newUpstreamFromRequestLocked(req, now)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "secret_encrypt_failed")
			return
		}
		created = append(created, up)
	}
	items := make([]map[string]any, 0, len(created))
	for _, up := range created {
		a.store.state.UpstreamAccounts = append(a.store.state.UpstreamAccounts, up)
		a.auditLocked(admin.ID, "admin", "upstream.import", up.ID, up.Name)
		items = append(items, publicUpstream(up))
	}
	if !a.saveOrErrorLocked(w) {
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"imported": len(items), "items": items})
}

func (a *App) newUpstreamFromRequestLocked(req adminUpstreamRequest, now time.Time) (UpstreamAccount, error) {
	accessCipher, err := a.encrypt(req.AccessToken)
	if err != nil {
		return UpstreamAccount{}, err
	}
	refreshCipher, err := a.encrypt(req.RefreshToken)
	if err != nil {
		return UpstreamAccount{}, err
	}
	return UpstreamAccount{
		ID: a.store.nextID("up"), Name: req.Name, Group: req.Group, CredentialType: valueOr(req.CredentialType, "oauth"),
		AccessTokenCipher: accessCipher, RefreshTokenCipher: refreshCipher, TokenType: req.TokenType, ChatGPTAccountID: req.ChatGPTAccountID, ExpiresAt: req.ExpiresAt,
		Email: req.Email, SubscriptionTier: req.SubscriptionTier, EntitlementStatus: req.EntitlementStatus,
		Status: statusActive, BalanceStatus: "available", RiskStatus: "available", CredentialFingerprint: a.fingerprint(req.AccessToken + req.RefreshToken),
		CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (a *App) adminUpstreamAction(w http.ResponseWriter, r *http.Request, rest string) {
	admin, ok := a.requireAdmin(w, r, false)
	if !ok {
		return
	}
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) == 2 && parts[1] == "credentials" && r.Method == http.MethodPost {
		a.adminReplaceUpstreamCredentials(w, r, admin, parts[0])
		return
	}
	if len(parts) == 2 && parts[1] == "check" && r.Method == http.MethodPost {
		if !readEmptyJSON(w, r, "invalid_upstream_check_request") {
			return
		}
		a.adminCheckUpstream(w, r, admin, parts[0])
		return
	}
	if len(parts) != 2 || parts[1] != "status" || r.Method != http.MethodPost {
		writeErr(w, http.StatusNotFound, "not_found")
		return
	}
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if !readStrictJSON(w, r, &req, "invalid_upstream_status_request", "enabled") {
		return
	}
	if req.Enabled == nil {
		writeErr(w, http.StatusBadRequest, "invalid_upstream_status_request")
		return
	}
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	idx := a.upstreamIndex(parts[0])
	if idx < 0 {
		writeErr(w, http.StatusNotFound, "upstream_not_found")
		return
	}
	up := &a.store.state.UpstreamAccounts[idx]
	if *req.Enabled {
		up.Status = statusActive
	} else {
		up.Status = statusDisabled
	}
	now := time.Now().UTC()
	up.LastCheckedAt = &now
	up.UpdatedAt = now
	a.auditLocked(admin.ID, "admin", "upstream.status", up.ID, fmt.Sprintf("enabled=%t", *req.Enabled))
	if !a.saveOrErrorLocked(w) {
		return
	}
	writeJSON(w, http.StatusOK, publicUpstream(*up))
}

func (a *App) adminReplaceUpstreamCredentials(w http.ResponseWriter, r *http.Request, admin Admin, id string) {
	req, ok := readAdminUpstreamRequest(w, r)
	if !ok {
		return
	}
	accessCipher, err := a.encrypt(req.AccessToken)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "secret_encrypt_failed")
		return
	}
	refreshCipher, err := a.encrypt(req.RefreshToken)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "secret_encrypt_failed")
		return
	}
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	idx := a.upstreamIndex(id)
	if idx < 0 {
		writeErr(w, http.StatusNotFound, "upstream_not_found")
		return
	}
	up := &a.store.state.UpstreamAccounts[idx]
	up.Name = req.Name
	up.Group = req.Group
	up.CredentialType = valueOr(req.CredentialType, "oauth")
	up.AccessTokenCipher = accessCipher
	up.RefreshTokenCipher = refreshCipher
	up.TokenType = req.TokenType
	up.ChatGPTAccountID = req.ChatGPTAccountID
	up.ExpiresAt = req.ExpiresAt
	up.Email = req.Email
	up.SubscriptionTier = req.SubscriptionTier
	up.EntitlementStatus = req.EntitlementStatus
	up.BalanceStatus = "available"
	up.RiskStatus = "available"
	up.UsageTokens = 0
	up.RateLimitUsedPercent = nil
	up.RateLimitResetsAt = nil
	up.CreditBalance = nil
	up.CreditBalanceLabel = ""
	up.LastCheckedAt = nil
	up.CredentialFingerprint = a.fingerprint(req.AccessToken + req.RefreshToken)
	up.UpdatedAt = time.Now().UTC()
	a.auditLocked(admin.ID, "admin", "upstream.credentials.replace", up.ID, "credentials_replaced")
	if !a.saveOrErrorLocked(w) {
		return
	}
	writeJSON(w, http.StatusOK, publicUpstream(*up))
}

func (a *App) adminCheckUpstream(w http.ResponseWriter, r *http.Request, admin Admin, id string) {
	a.store.mu.Lock()
	up := a.upstreamByID(id)
	a.store.mu.Unlock()
	if up.ID == "" {
		writeErr(w, http.StatusNotFound, "upstream_not_found")
		return
	}
	accessToken, err := a.decrypt(up.AccessTokenCipher)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "secret_decrypt_failed")
		return
	}
	if strings.TrimSpace(accessToken) == "" {
		a.store.mu.Lock()
		idx := a.upstreamIndex(id)
		if idx >= 0 {
			now := time.Now().UTC()
			a.store.state.UpstreamAccounts[idx].BalanceStatus = "unavailable"
			a.store.state.UpstreamAccounts[idx].RiskStatus = "unavailable"
			a.store.state.UpstreamAccounts[idx].EntitlementStatus = "missing_access_token"
			a.store.state.UpstreamAccounts[idx].LastCheckedAt = &now
			a.store.state.UpstreamAccounts[idx].UpdatedAt = now
			a.auditLocked(admin.ID, "admin", "upstream.check", id, "missing_access_token")
			if !a.saveOrErrorLocked(w) {
				a.store.mu.Unlock()
				return
			}
			up = a.store.state.UpstreamAccounts[idx]
		}
		a.store.mu.Unlock()
		writeJSON(w, http.StatusOK, publicUpstream(up))
		return
	}
	credentials := codexProbeCredentials{
		AccessToken:      accessToken,
		ChatGPTAccountID: strings.TrimSpace(up.ChatGPTAccountID),
		ChatGPTPlanType:  up.SubscriptionTier,
	}
	if credentials.ChatGPTAccountID == "" {
		credentials.ChatGPTAccountID = chatGPTAccountIDFromAccessToken(accessToken)
	}
	if credentials.ChatGPTAccountID == "" {
		a.store.mu.Lock()
		idx := a.upstreamIndex(id)
		if idx >= 0 {
			now := time.Now().UTC()
			a.store.state.UpstreamAccounts[idx].BalanceStatus = "unavailable"
			a.store.state.UpstreamAccounts[idx].RiskStatus = "unavailable"
			a.store.state.UpstreamAccounts[idx].EntitlementStatus = "missing_chatgpt_account_id"
			a.store.state.UpstreamAccounts[idx].LastCheckedAt = &now
			a.store.state.UpstreamAccounts[idx].UpdatedAt = now
			a.auditLocked(admin.ID, "admin", "upstream.check", id, "missing_chatgpt_account_id")
			if !a.saveOrErrorLocked(w) {
				a.store.mu.Unlock()
				return
			}
			up = a.store.state.UpstreamAccounts[idx]
		}
		a.store.mu.Unlock()
		writeJSON(w, http.StatusOK, publicUpstream(up))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	result, err := codexAppServerProbe(ctx, credentials)
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	idx := a.upstreamIndex(id)
	if idx < 0 {
		writeErr(w, http.StatusNotFound, "upstream_not_found")
		return
	}
	now := time.Now().UTC()
	target := &a.store.state.UpstreamAccounts[idx]
	target.ChatGPTAccountID = credentials.ChatGPTAccountID
	target.LastCheckedAt = &now
	target.UpdatedAt = now
	if err != nil {
		target.BalanceStatus = "unavailable"
		target.RiskStatus = "unavailable"
		target.EntitlementStatus = upstreamProbeFailureStatus(err)
		a.auditLocked(admin.ID, "admin", "upstream.check", id, target.EntitlementStatus)
		if !a.saveOrErrorLocked(w) {
			return
		}
		writeJSON(w, http.StatusOK, publicUpstream(*target))
		return
	}
	if result.Email != "" {
		target.Email = result.Email
	}
	if result.PlanType != "" {
		target.SubscriptionTier = result.PlanType
	}
	if result.RateLimitReachedType != "" {
		target.BalanceStatus = "unavailable"
		target.EntitlementStatus = result.RateLimitReachedType
	} else {
		target.BalanceStatus = "available"
		target.EntitlementStatus = "available"
	}
	target.UsageTokens = result.UsageTokens
	target.RateLimitUsedPercent = result.RateLimitUsedPercent
	target.RateLimitResetsAt = result.RateLimitResetsAt
	target.CreditBalance = result.CreditBalance
	target.CreditBalanceLabel = result.CreditBalanceLabel
	target.RiskStatus = "available"
	detail := fmt.Sprintf("account=%s plan=%s usage_tokens=%d rate_limit=%s", result.AccountType, result.PlanType, result.UsageTokens, result.RateLimitReachedType)
	a.auditLocked(admin.ID, "admin", "upstream.check", id, detail)
	if !a.saveOrErrorLocked(w) {
		return
	}
	resp := publicUpstream(*target)
	resp["usageTokens"] = result.UsageTokens
	resp["accountType"] = result.AccountType
	writeJSON(w, http.StatusOK, resp)
}

func (a *App) adminAPIKeys(w http.ResponseWriter, r *http.Request) {
	_, ok := a.requireAdmin(w, r, false)
	if !ok {
		return
	}
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	items := make([]map[string]any, 0, len(a.store.state.APIKeys))
	for _, key := range a.store.state.APIKeys {
		items = append(items, a.publicAPIKeyLocked(key))
	}
	writeJSON(w, http.StatusOK, listPayload(items, r))
}

func (a *App) adminCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	admin, ok := a.requireAdmin(w, r, false)
	if !ok {
		return
	}
	req, ok := readAdminAPIKeyCreateRequest(w, r)
	if !ok {
		return
	}
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	if a.upstreamIndex(req.UpstreamAccountID) < 0 {
		writeErr(w, http.StatusNotFound, "upstream_not_found")
		return
	}
	raw, err := generateSub2APIKey()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "key_generate_failed")
		return
	}
	now := time.Now().UTC()
	key := APIKey{ID: a.store.nextID("key"), KeyHash: hashString(raw), PublicPrefix: raw[:10], UpstreamAccountID: req.UpstreamAccountID, Status: statusActive, CreatedAt: now, UpdatedAt: now}
	a.store.state.APIKeys = append(a.store.state.APIKeys, key)
	a.auditLocked(admin.ID, "admin", "api_key.create", key.ID, req.UpstreamAccountID)
	if !a.saveOrErrorLocked(w) {
		return
	}
	writeJSON(w, http.StatusCreated, a.publicAPIKeyLocked(key))
}

func (a *App) adminAPIKeyAction(w http.ResponseWriter, r *http.Request, rest string) {
	admin, ok := a.requireAdmin(w, r, false)
	if !ok {
		return
	}
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) != 2 || parts[1] != "status" || r.Method != http.MethodPost {
		writeErr(w, http.StatusNotFound, "not_found")
		return
	}
	req, ok := readAdminAPIKeyStatusRequest(w, r)
	if !ok {
		return
	}
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	idx := a.apiKeyIndex(parts[0])
	if idx < 0 {
		writeErr(w, http.StatusNotFound, "api_key_not_found")
		return
	}
	if req.Status != statusActive && req.Status != statusDisabled {
		writeErr(w, http.StatusBadRequest, "invalid_status")
		return
	}
	a.store.state.APIKeys[idx].Status = req.Status
	a.store.state.APIKeys[idx].UpdatedAt = time.Now().UTC()
	a.auditLocked(admin.ID, "admin", "api_key.status", parts[0], req.Status)
	if !a.saveOrErrorLocked(w) {
		return
	}
	writeJSON(w, http.StatusOK, a.publicAPIKeyLocked(a.store.state.APIKeys[idx]))
}

func (a *App) adminDevices(w http.ResponseWriter, r *http.Request) {
	_, ok := a.requireAdmin(w, r, false)
	if !ok {
		return
	}
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	items := make([]map[string]any, 0, len(a.store.state.Devices))
	for _, dev := range a.store.state.Devices {
		items = append(items, a.publicAdminDeviceLocked(dev))
	}
	writeJSON(w, http.StatusOK, listPayload(items, r))
}

func (a *App) adminDeviceAction(w http.ResponseWriter, r *http.Request, rest string) {
	admin, ok := a.requireAdmin(w, r, false)
	if !ok {
		return
	}
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) != 2 || parts[1] != "status" || r.Method != http.MethodPost {
		writeErr(w, http.StatusNotFound, "not_found")
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if !readStrictJSON(w, r, &req, "invalid_device_status_request", "status") {
		return
	}
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	idx := a.deviceIndex(parts[0])
	if idx < 0 {
		writeErr(w, http.StatusNotFound, "device_not_found")
		return
	}
	if req.Status != statusActive && req.Status != statusDisabled {
		writeErr(w, http.StatusBadRequest, "invalid_status")
		return
	}
	a.store.state.Devices[idx].Status = req.Status
	a.store.state.Devices[idx].UpdatedAt = time.Now().UTC()
	a.auditLocked(admin.ID, "admin", "device.status", parts[0], req.Status)
	if !a.saveOrErrorLocked(w) {
		return
	}
	writeJSON(w, http.StatusOK, a.publicAdminDeviceLocked(a.store.state.Devices[idx]))
}

func (a *App) adminUsage(w http.ResponseWriter, r *http.Request) {
	_, ok := a.requireAdmin(w, r, false)
	if !ok {
		return
	}
	todayOnly := r.URL.Query().Get("today") == "true"
	var start, end time.Time
	if todayOnly {
		start, end = utcDayBounds(time.Now())
	}
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	items := make([]map[string]any, 0, len(a.store.state.UsageRecords))
	for _, rec := range a.store.state.UsageRecords {
		if todayOnly && (rec.CreatedAt.Before(start) || !rec.CreatedAt.Before(end)) {
			continue
		}
		items = append(items, a.publicAdminUsageLocked(rec))
	}
	sortPublicRecordsByCreatedAtDesc(items)
	writeJSON(w, http.StatusOK, listPayload(items, r))
}

func (a *App) adminLedger(w http.ResponseWriter, r *http.Request) {
	_, ok := a.requireAdmin(w, r, false)
	if !ok {
		return
	}
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	items := make([]map[string]any, 0, len(a.store.state.TokenLedgers))
	for _, rec := range a.store.state.TokenLedgers {
		items = append(items, a.publicAdminTokenLedgerLocked(rec))
	}
	sortPublicRecordsByCreatedAtDesc(items)
	writeJSON(w, http.StatusOK, listPayload(items, r))
}

func (a *App) adminAudit(w http.ResponseWriter, r *http.Request) {
	_, ok := a.requireAdmin(w, r, false)
	if !ok {
		return
	}
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	logs := append([]AuditLog(nil), a.store.state.AuditLogs...)
	sort.Slice(logs, func(i, j int) bool { return logs[i].CreatedAt.After(logs[j].CreatedAt) })
	items := make([]map[string]any, 0, len(logs))
	for _, item := range logs {
		items = append(items, a.publicAdminAuditLocked(item))
	}
	writeJSON(w, http.StatusOK, listPayload(items, r))
}

func (a *App) clientLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Account     string `json:"account"`
		Password    string `json:"password"`
		DeviceID    string `json:"deviceId"`
		DeviceName  string `json:"deviceName"`
		Fingerprint string `json:"fingerprint"`
	}
	if !readStrictJSON(w, r, &req, "invalid_client_login_request", "account", "password", "deviceId", "deviceName", "fingerprint") {
		return
	}
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	idx := -1
	for i, user := range a.store.state.Users {
		if user.Account == req.Account && verifyPassword(req.Password, user.PasswordSalt, user.PasswordHash) {
			idx = i
			break
		}
	}
	if idx < 0 || a.store.state.Users[idx].Status != statusActive {
		actorID := "unknown"
		if idx >= 0 {
			actorID = a.store.state.Users[idx].ID
		}
		a.auditLocked(actorID, "client", "client.login.failed", req.Account, "login_failed")
		if !a.saveOrErrorLocked(w) {
			return
		}
		writeErr(w, http.StatusUnauthorized, "login_failed")
		return
	}
	now := time.Now().UTC()
	userID := a.store.state.Users[idx].ID
	deviceFingerprint := strings.TrimSpace(req.Fingerprint)
	deviceID := strings.TrimSpace(req.DeviceID)
	if deviceFingerprint == "" {
		a.auditLocked(userID, "client", "client.login.failed", req.Account, "login_failed")
		if !a.saveOrErrorLocked(w) {
			return
		}
		writeErr(w, http.StatusUnauthorized, "login_failed")
		return
	}
	devID := deviceID
	devIdx := -1
	if deviceID != "" {
		devIdx = a.deviceIndex(deviceID)
		if devIdx < 0 || a.store.state.Devices[devIdx].UserID != userID || a.store.state.Devices[devIdx].Fingerprint != deviceFingerprint {
			a.auditLocked(userID, "client", "client.login.failed", deviceID, "login_failed")
			if !a.saveOrErrorLocked(w) {
				return
			}
			writeErr(w, http.StatusUnauthorized, "login_failed")
			return
		}
	} else {
		devIdx = a.deviceByUserAndFingerprint(userID, deviceFingerprint)
	}
	if devIdx < 0 {
		devID = a.store.nextID("dev")
		dev := Device{ID: devID, UserID: userID, Name: valueOr(req.DeviceName, "Windows 设备"), Fingerprint: deviceFingerprint, Status: statusActive, LastSeenAt: &now, CreatedAt: now, UpdatedAt: now}
		a.store.state.Devices = append(a.store.state.Devices, dev)
	} else {
		if a.store.state.Devices[devIdx].Status != statusActive {
			a.auditLocked(userID, "client", "client.login.failed", a.store.state.Devices[devIdx].ID, "login_failed")
			if !a.saveOrErrorLocked(w) {
				return
			}
			writeErr(w, http.StatusUnauthorized, "login_failed")
			return
		}
		devID = a.store.state.Devices[devIdx].ID
		a.store.state.Devices[devIdx].LastSeenAt = &now
		a.store.state.Devices[devIdx].UpdatedAt = now
	}
	a.store.state.Users[idx].LastLoginAt = &now
	a.store.state.Users[idx].UpdatedAt = now
	token := a.createSessionLocked(sessionRoleClient, a.store.state.Users[idx].ID, devID)
	a.auditLocked(a.store.state.Users[idx].ID, "client", "client.login.success", devID, "")
	if !a.saveOrErrorLocked(w) {
		return
	}
	device := a.deviceByID(devID)
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "user": a.publicClientUserLocked(a.store.state.Users[idx]), "device": publicClientDevice(device), "security": publicClientSecurity(a.store.state.Users[idx], device)})
}

func (a *App) clientMe(w http.ResponseWriter, r *http.Request) {
	user, device, ok := a.requireClient(w, r)
	if !ok {
		return
	}
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"user": a.publicClientUserLocked(user), "device": publicClientDevice(device), "security": publicClientSecurity(user, device), "service": a.publicClientServiceLocked(user), "usage7d": a.usageDailyLocked(user.ID, 7)})
}

func (a *App) clientTopups(w http.ResponseWriter, r *http.Request) {
	_, _, ok := a.requireClient(w, r)
	if !ok {
		return
	}
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	items := make([]TokenTopup, 0)
	for _, topup := range a.store.state.TokenTopups {
		if topup.Enabled {
			items = append(items, topup)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Sort < items[j].Sort })
	public := make([]map[string]any, 0, len(items))
	for _, topup := range items {
		public = append(public, publicClientTopup(topup))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": public})
}

func (a *App) clientCreateRecharge(w http.ResponseWriter, r *http.Request) {
	user, _, ok := a.requireClient(w, r)
	if !ok {
		return
	}
	topupID, ok := readClientRechargeRequest(w, r)
	if !ok {
		return
	}
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	topupIdx := a.topupIndex(topupID)
	if topupIdx < 0 || !a.store.state.TokenTopups[topupIdx].Enabled {
		writeErr(w, http.StatusBadRequest, "topup_unavailable")
		return
	}
	topup := a.store.state.TokenTopups[topupIdx]
	if topup.PriceCents == 0 {
		writeErr(w, http.StatusBadRequest, "free_topup_view_only")
		return
	}
	now := time.Now().UTC()
	rr := RechargeRequest{
		ID: a.store.nextID("rch"), UserID: user.ID, TopupID: topup.ID, TopupName: topup.Name, PriceCents: topup.PriceCents, Tokens: topup.Tokens,
		Status: rechargePending, SubmittedAt: now, UpdatedAt: now,
		StatusTransitions: []RechargeStatusTransition{{Status: rechargePending, At: now, Action: "recharge.request", ActorRole: "client"}},
	}
	a.store.state.RechargeRequests = append(a.store.state.RechargeRequests, rr)
	a.auditLocked(user.ID, "client", "recharge.request", rr.ID, rr.TopupName)
	if !a.saveOrErrorLocked(w) {
		return
	}
	writeJSON(w, http.StatusCreated, publicClientRecharge(rr))
}

func (a *App) clientRecharges(w http.ResponseWriter, r *http.Request) {
	user, _, ok := a.requireClient(w, r)
	if !ok {
		return
	}
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	items := make([]map[string]any, 0)
	for _, rr := range a.store.state.RechargeRequests {
		if rr.UserID == user.ID {
			items = append(items, publicClientRecharge(rr))
		}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i]["submittedAt"].(time.Time).After(items[j]["submittedAt"].(time.Time))
	})
	writeJSON(w, http.StatusOK, listPayload(items, r))
}

func (a *App) clientUsage(w http.ResponseWriter, r *http.Request) {
	user, _, ok := a.requireClient(w, r)
	if !ok {
		return
	}
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	items := make([]map[string]any, 0)
	for _, rec := range a.store.state.UsageRecords {
		if rec.UserID == user.ID {
			items = append(items, publicClientUsage(rec))
		}
	}
	sortPublicRecordsByCreatedAtDesc(items)
	writeJSON(w, http.StatusOK, listPayload(items, r))
}

func (a *App) clientLedger(w http.ResponseWriter, r *http.Request) {
	user, _, ok := a.requireClient(w, r)
	if !ok {
		return
	}
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	items := make([]map[string]any, 0)
	for _, rec := range a.store.state.TokenLedgers {
		if rec.UserID == user.ID {
			items = append(items, publicClientTokenLedger(rec))
		}
	}
	sortPublicRecordsByCreatedAtDesc(items)
	writeJSON(w, http.StatusOK, listPayload(items, r))
}

func (a *App) clientLedgerSummary(w http.ResponseWriter, r *http.Request) {
	user, _, ok := a.requireClient(w, r)
	if !ok {
		return
	}
	days := intParam(r, "days", 30)
	if days < 1 {
		days = 30
	}
	if days > 90 {
		days = 90
	}
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	start := today.AddDate(0, 0, -(days - 1))
	items := make([]map[string]any, days)
	index := make(map[string]int, days)
	for i := 0; i < days; i++ {
		day := start.AddDate(0, 0, i)
		date := day.Format("2006-01-02")
		index[date] = i
		items[i] = map[string]any{"date": date, "incomeTokens": int64(0), "expenseTokens": int64(0), "netTokens": int64(0)}
	}
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	for _, rec := range a.store.state.TokenLedgers {
		if rec.UserID != user.ID || rec.CreatedAt.Before(start) {
			continue
		}
		date := rec.CreatedAt.UTC().Format("2006-01-02")
		i, ok := index[date]
		if !ok {
			continue
		}
		if rec.DeltaTokens >= 0 {
			items[i]["incomeTokens"] = items[i]["incomeTokens"].(int64) + rec.DeltaTokens
		} else {
			items[i]["expenseTokens"] = items[i]["expenseTokens"].(int64) - rec.DeltaTokens
		}
		items[i]["netTokens"] = items[i]["incomeTokens"].(int64) - items[i]["expenseTokens"].(int64)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "days": days})
}

func (a *App) clientDesktopUpdate(w http.ResponseWriter, r *http.Request) {
	_, _, ok := a.requireClient(w, r)
	if !ok {
		return
	}
	current := strings.TrimSpace(r.URL.Query().Get("currentVersion"))
	writeJSON(w, http.StatusOK, desktopUpdatePayload(current, time.Now().UTC()))
}

func desktopUpdatePayload(current string, checkedAt time.Time) map[string]any {
	current = strings.TrimSpace(current)
	if current == "" {
		current = "unknown"
	}
	latest := strings.TrimSpace(os.Getenv("CODEXPPP_DESKTOP_LATEST_VERSION"))
	if latest == "" {
		latest = current
	}
	available := versionGreater(latest, current)
	downloadURL := ""
	if available {
		downloadURL = safeHTTPURL(os.Getenv("CODEXPPP_DESKTOP_DOWNLOAD_URL"))
	}
	sha256Value := ""
	if available {
		sha256Value = strings.TrimSpace(os.Getenv("CODEXPPP_DESKTOP_DOWNLOAD_SHA256"))
	}
	notes := ""
	if available {
		notes = strings.TrimSpace(os.Getenv("CODEXPPP_DESKTOP_RELEASE_NOTES"))
	}
	return map[string]any{
		"currentVersion": current,
		"latestVersion":  latest,
		"available":      available,
		"downloadUrl":    downloadURL,
		"sha256":         sha256Value,
		"releaseNotes":   notes,
		"checkedAt":      checkedAt,
	}
}

func (a *App) clientLaunchPrepare(w http.ResponseWriter, r *http.Request) {
	user, device, ok := a.requireClient(w, r)
	if !ok {
		return
	}
	var req struct{}
	if !readStrictJSON(w, r, &req, "invalid_launch_prepare_request") {
		return
	}
	a.store.mu.Lock()
	if a.availableTokenBalanceLocked(user.ID) <= 0 {
		a.store.mu.Unlock()
		writeErr(w, http.StatusPaymentRequired, "token_not_available")
		return
	}
	routes := a.routeCandidatesLocked()
	if len(routes) == 0 {
		a.store.mu.Unlock()
		writeErr(w, http.StatusServiceUnavailable, "route_unavailable")
		return
	}
	providerToken, err := a.codexProviderSessionTokenLocked(user.ID, device.ID)
	if err != nil {
		a.store.mu.Unlock()
		writeErr(w, http.StatusInternalServerError, "key_generate_failed")
		return
	}
	if !a.saveOrErrorLocked(w) {
		a.store.mu.Unlock()
		return
	}
	a.store.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"launchState": "ready",
		"provider": map[string]any{
			"bearerToken": providerToken,
		},
		"diagnostics": map[string]string{"codexDetected": "不可用", "configWritten": "不可用", "lastFailure": ""},
	})
}

func (a *App) gatewayRun(w http.ResponseWriter, r *http.Request) {
	user, _, ok := a.requireClient(w, r)
	if !ok {
		return
	}
	defer r.Body.Close()
	body, ok := readLimitedBody(w, r.Body, maxGatewayBodyBytes)
	if !ok {
		return
	}
	a.gatewayRunForUser(w, r, user, body)
}

func (a *App) gatewayRunForUser(w http.ResponseWriter, r *http.Request, user User, body []byte) {
	requestBody, requestModel, bodyRequestID, err := sanitizeGatewayRequestBody(body)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_json")
		return
	}
	requestID := gatewayRequestID(r, bodyRequestID)
	a.store.mu.Lock()
	idx := a.userIndex(user.ID)
	if idx < 0 || a.store.state.Users[idx].Status != statusActive {
		a.store.mu.Unlock()
		writeErr(w, http.StatusUnauthorized, "login_failed")
		return
	}
	if status, payload, handled := a.existingGatewayResponseLocked(user.ID, requestID); handled {
		a.store.mu.Unlock()
		writeJSON(w, status, payload)
		return
	}
	a.store.mu.Unlock()
	if !a.allowGatewayRequest(w, r, user.ID) {
		return
	}
	a.store.mu.Lock()
	idx = a.userIndex(user.ID)
	if idx < 0 || a.store.state.Users[idx].Status != statusActive {
		a.store.mu.Unlock()
		writeErr(w, http.StatusUnauthorized, "login_failed")
		return
	}
	if status, payload, handled := a.existingGatewayResponseLocked(user.ID, requestID); handled {
		a.store.mu.Unlock()
		writeJSON(w, status, payload)
		return
	}
	availableBalance := a.availableTokenBalanceLocked(user.ID)
	if availableBalance <= 0 {
		a.store.mu.Unlock()
		writeErr(w, http.StatusPaymentRequired, "token_not_available")
		return
	}
	routes := a.routeCandidatesLocked()
	if len(routes) == 0 {
		a.store.mu.Unlock()
		writeErr(w, http.StatusServiceUnavailable, "route_unavailable")
		return
	}
	now := time.Now().UTC()
	if existingIdx := a.gatewayRequestIndexLocked(user.ID, requestID); existingIdx >= 0 {
		req := &a.store.state.GatewayRequests[existingIdx]
		req.Status = gatewayReserved
		req.ReservedTokens = availableBalance
		req.ChargedTokens = 0
		req.UsageRecordID = ""
		req.UpstreamStatus = 0
		req.Error = ""
		req.ResultText = ""
		req.UpdatedAt = now
	} else {
		a.store.state.GatewayRequests = append(a.store.state.GatewayRequests, GatewayRequest{ID: a.store.nextID("gw"), UserID: user.ID, RequestID: requestID, Status: gatewayReserved, ReservedTokens: availableBalance, CreatedAt: now, UpdatedAt: now})
	}
	if err := a.store.save(); err != nil {
		a.store.mu.Unlock()
		writeErr(w, http.StatusInternalServerError, "state_save_failed")
		return
	}
	a.store.mu.Unlock()

	var upstreamStatus int
	var upstreamPayload any
	var usage gatewayUsage
	var selected gatewayRoute
	var lastFailure gatewayRouteFailure
	var switchFrom gatewayRoute
	var switchReason string
	for _, route := range routes {
		status, payload, routeUsage, failure, ok := a.tryGatewayRoute(r.Context(), route, requestBody)
		if ok {
			upstreamStatus = status
			upstreamPayload = payload
			usage = routeUsage
			selected = route
			break
		}
		lastFailure = failure
		if failure.MarkUnavailable {
			if err := a.markRouteUnavailable(route, failure.Code, failure.UpstreamStatus); err != nil {
				lastFailure = gatewayRouteFailure{Code: "state_save_failed", HTTPStatus: http.StatusInternalServerError, TryNext: false}
				break
			}
		}
		if !failure.TryNext {
			break
		}
		switchFrom = route
		switchReason = failure.Code
	}
	if selected.Upstream.ID == "" {
		status := lastFailure.UpstreamStatus
		code := valueOr(lastFailure.Code, "route_unavailable")
		if err := a.failGatewayRequest(user.ID, requestID, status, code); err != nil {
			writeErr(w, http.StatusInternalServerError, "state_save_failed")
			return
		}
		writeGatewayFailure(w, lastFailure)
		return
	}
	model := valueOr(stringField(upstreamPayload, "model"), valueOr(requestModel, "codex"))
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	idx = a.userIndex(user.ID)
	if idx < 0 || a.store.state.Users[idx].Status != statusActive {
		a.markGatewayRequestFailedLocked(user.ID, requestID, upstreamStatus, "login_failed")
		if !a.saveOrErrorLocked(w) {
			return
		}
		writeErr(w, http.StatusUnauthorized, "login_failed")
		return
	}
	reqIdx := a.gatewayRequestIndexLocked(user.ID, requestID)
	if reqIdx < 0 || a.store.state.GatewayRequests[reqIdx].Status != gatewayReserved {
		writeErr(w, http.StatusConflict, "request_not_reserved")
		return
	}
	chargeTokens := usage.TotalTokens
	if reserved := a.store.state.GatewayRequests[reqIdx].ReservedTokens; reserved < chargeTokens {
		chargeTokens = reserved
	}
	if balance := a.store.state.Users[idx].TokenBalance; balance < chargeTokens {
		chargeTokens = balance
	}
	finishedAt := time.Now().UTC()
	rec := UsageRecord{ID: a.store.nextID("use"), UserID: user.ID, Model: model, InputTokens: usage.InputTokens, CachedInputTokens: usage.CachedInputTokens, OutputTokens: usage.OutputTokens, TotalTokens: usage.TotalTokens, CreatedAt: finishedAt}
	a.store.state.UsageRecords = append(a.store.state.UsageRecords, rec)
	if chargeTokens > 0 {
		source := "Codex token 用量"
		if chargeTokens < usage.TotalTokens {
			source = "Codex token 用量（余额扣至 0）"
		}
		if err := a.applyTokenDeltaLocked(user.ID, -chargeTokens, "debit", source); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	if switchFrom.Key.ID != "" && selected.Key.ID != "" && switchFrom.Key.ID != selected.Key.ID {
		a.auditLocked("system", "system", "gateway.route.switch", selected.Upstream.ID, fmt.Sprintf("from_key=%s to_key=%s reason=%s", switchFrom.Key.ID, selected.Key.ID, switchReason))
	}
	a.auditLocked(user.ID, "client", "gateway.usage.debit", requestID, fmt.Sprintf("model=%s usage_tokens=%d charged_tokens=%d", model, usage.TotalTokens, chargeTokens))
	result := resultField(upstreamPayload)
	req := &a.store.state.GatewayRequests[reqIdx]
	req.Status = gatewayCompleted
	req.ReservedTokens = 0
	req.ChargedTokens = chargeTokens
	req.UsageRecordID = rec.ID
	req.UpstreamStatus = upstreamStatus
	req.Error = ""
	req.ResultText = textFromAny(result)
	req.UpdatedAt = finishedAt
	if keyIdx := a.apiKeyIndex(selected.Key.ID); keyIdx >= 0 {
		a.store.state.APIKeys[keyIdx].LastUsedAt = &finishedAt
		a.store.state.APIKeys[keyIdx].UpdatedAt = finishedAt
	}
	if err := a.store.save(); err != nil {
		writeErr(w, http.StatusInternalServerError, "state_save_failed")
		return
	}
	writeJSON(w, http.StatusOK, gatewayRunResponse(requestID, rec, upstreamStatus, chargeTokens, false, result))
}

func (a *App) codexResponses(w http.ResponseWriter, r *http.Request) {
	user, _, ok := a.requireCodexClient(w, r)
	if !ok {
		return
	}
	defer r.Body.Close()
	body, ok := readLimitedBody(w, r.Body, maxGatewayBodyBytes)
	if !ok {
		return
	}
	stream := false
	var requestPayload map[string]any
	if err := json.Unmarshal(body, &requestPayload); err == nil {
		stream, _ = requestPayload["stream"].(bool)
	}
	capture := newCaptureResponse()
	a.gatewayRunForUser(capture, r, user, body)
	status := capture.status
	if status == 0 {
		status = http.StatusOK
	}
	var gatewayPayload map[string]any
	dec := json.NewDecoder(bytes.NewReader(capture.body.Bytes()))
	dec.UseNumber()
	if err := dec.Decode(&gatewayPayload); err != nil {
		writeErr(w, http.StatusBadGateway, "gateway_invalid_json")
		return
	}
	if status < 200 || status >= 300 {
		writeJSON(w, status, codexErrorPayloadFromGateway(gatewayPayload))
		return
	}
	response := codexResponsePayloadFromGateway(gatewayPayload)
	if stream {
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(codexResponsesSSE(response)))
		return
	}
	writeJSON(w, http.StatusOK, response)
}

type captureResponse struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func newCaptureResponse() *captureResponse {
	return &captureResponse{header: http.Header{}}
}

func (r *captureResponse) Header() http.Header {
	return r.header
}

func (r *captureResponse) WriteHeader(status int) {
	if r.status == 0 {
		r.status = status
	}
}

func (r *captureResponse) Write(body []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.body.Write(body)
}

type gatewayUsage struct {
	InputTokens       int64
	CachedInputTokens int64
	OutputTokens      int64
	TotalTokens       int64
}

type gatewayRoute struct {
	Key      APIKey
	Upstream UpstreamAccount
}

type gatewayRouteFailure struct {
	Code            string
	HTTPStatus      int
	UpstreamStatus  int
	TryNext         bool
	MarkUnavailable bool
}

func (a *App) tryGatewayRoute(ctx context.Context, route gatewayRoute, requestBody []byte) (int, any, gatewayUsage, gatewayRouteFailure, bool) {
	accessToken, err := a.decrypt(route.Upstream.AccessTokenCipher)
	if err != nil {
		return 0, nil, gatewayUsage{}, gatewayRouteFailure{Code: "secret_decrypt_failed", HTTPStatus: http.StatusServiceUnavailable, TryNext: true, MarkUnavailable: true}, false
	}
	credentials := codexProbeCredentials{
		AccessToken:      accessToken,
		ChatGPTAccountID: strings.TrimSpace(route.Upstream.ChatGPTAccountID),
		ChatGPTPlanType:  route.Upstream.SubscriptionTier,
	}
	if credentials.ChatGPTAccountID == "" {
		credentials.ChatGPTAccountID = chatGPTAccountIDFromAccessToken(accessToken)
	}
	result, err := codexAppServerRun(ctx, credentials, requestBody)
	if err != nil {
		return 0, nil, gatewayUsage{}, classifyCodexAppServerRunError(err), false
	}
	if result.Usage.TotalTokens <= 0 {
		return 0, nil, gatewayUsage{}, classifyCodexAppServerRunError(errors.New("codex_app_server_usage_missing")), false
	}
	status := result.Status
	if status == 0 {
		status = http.StatusOK
	}
	payload := map[string]any{
		"model":  valueOr(result.Model, "codex"),
		"usage":  appServerUsagePayload(result.Usage),
		"result": map[string]any{"text": result.Text},
	}
	return status, payload, result.Usage, gatewayRouteFailure{}, true
}

func classifyCodexAppServerRunError(err error) gatewayRouteFailure {
	code := err.Error()
	failure := gatewayRouteFailure{Code: code, HTTPStatus: http.StatusBadGateway, TryNext: true, MarkUnavailable: false}
	switch {
	case code == "codex_app_server_input_missing":
		failure.HTTPStatus = http.StatusBadRequest
		failure.TryNext = false
	case code == "codex_app_server_start_failed":
		failure.HTTPStatus = http.StatusServiceUnavailable
		failure.TryNext = false
	case code == "codex_app_server_usage_missing":
		failure.HTTPStatus = http.StatusBadGateway
		failure.TryNext = false
	case code == "codex_app_server_invalid_json":
		failure.HTTPStatus = http.StatusBadGateway
		failure.TryNext = false
	case strings.HasPrefix(code, "codex_app_server_http_"):
		rawStatus := strings.TrimPrefix(code, "codex_app_server_http_")
		status, _ := strconv.Atoi(rawStatus)
		failure = classifyUpstreamStatus(status)
	case strings.HasPrefix(code, "codex_app_server_"):
		failure.HTTPStatus = http.StatusBadGateway
	}
	return failure
}

func appServerUsagePayload(usage gatewayUsage) map[string]any {
	return map[string]any{
		"input_tokens":        usage.InputTokens,
		"cached_input_tokens": usage.CachedInputTokens,
		"output_tokens":       usage.OutputTokens,
		"total_tokens":        usage.TotalTokens,
		"inputTokens":         usage.InputTokens,
		"cachedInputTokens":   usage.CachedInputTokens,
		"outputTokens":        usage.OutputTokens,
		"totalTokens":         usage.TotalTokens,
	}
}

func classifyUpstreamStatus(status int) gatewayRouteFailure {
	failure := gatewayRouteFailure{HTTPStatus: http.StatusBadGateway, UpstreamStatus: status, Code: "upstream_failed", TryNext: true, MarkUnavailable: true}
	switch {
	case status == http.StatusPaymentRequired:
		failure.Code = "upstream_balance_unavailable"
		failure.HTTPStatus = http.StatusServiceUnavailable
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		failure.Code = "upstream_auth_unavailable"
		failure.HTTPStatus = http.StatusServiceUnavailable
	case status == http.StatusTooManyRequests:
		failure.Code = "upstream_limited"
		failure.HTTPStatus = http.StatusServiceUnavailable
	case status == http.StatusNotFound:
		failure.Code = "upstream_route_invalid"
		failure.HTTPStatus = http.StatusServiceUnavailable
	case status == http.StatusBadRequest || status == http.StatusUnprocessableEntity:
		failure.Code = "upstream_rejected_request"
		failure.TryNext = false
		failure.MarkUnavailable = false
	case status >= 500:
		failure.Code = "upstream_unavailable"
		failure.HTTPStatus = http.StatusServiceUnavailable
	}
	return failure
}

func writeGatewayFailure(w http.ResponseWriter, failure gatewayRouteFailure) {
	status := failure.HTTPStatus
	if status == 0 {
		status = http.StatusServiceUnavailable
	}
	code := valueOr(failure.Code, "route_unavailable")
	if status == http.StatusServiceUnavailable && failure.TryNext {
		code = "route_unavailable"
	}
	payload := map[string]any{"error": code}
	writeJSON(w, status, payload)
}

func gatewayRequestID(r *http.Request, fallback string) string {
	for _, key := range []string{"Idempotency-Key", "X-Request-ID", "X-Request-Id"} {
		if value := gatewayRequestIDFromValue(r.Header.Get(key)); value != "" {
			return value
		}
	}
	if value := gatewayRequestIDFromValue(fallback); value != "" {
		return value
	}
	return "auto_" + randomToken(18)
}

func gatewayRequestIDFromValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if isSafeGatewayRequestID(value) {
		return value
	}
	sum := sha256.Sum256([]byte(value))
	encoded := hex.EncodeToString(sum[:])
	return "req_" + encoded[:24]
}

func isSafeGatewayRequestID(value string) bool {
	if value == "" || len(value) > 80 {
		return false
	}
	for _, ch := range value {
		switch {
		case ch >= 'a' && ch <= 'z':
		case ch >= 'A' && ch <= 'Z':
		case ch >= '0' && ch <= '9':
		case ch == '_' || ch == '-' || ch == '.' || ch == ':':
		default:
			return false
		}
	}
	return true
}

func gatewayRunResponse(requestID string, rec UsageRecord, _ int, chargedTokens int64, idempotent bool, result any) map[string]any {
	return map[string]any{
		"requestId":     requestID,
		"usageRecord":   publicClientUsage(rec),
		"chargedTokens": chargedTokens,
		"idempotent":    idempotent,
		"result":        result,
	}
}

func codexResponsePayloadFromGateway(payload map[string]any) map[string]any {
	requestID := valueOr(stringField(payload, "requestId"), "codexppp")
	usage, _ := payload["usageRecord"].(map[string]any)
	model := valueOr(stringField(usage, "model"), "codex")
	text := textFromAny(payload["result"])
	return map[string]any{
		"id":          "resp_" + requestID,
		"object":      "response",
		"created_at":  time.Now().UTC().Unix(),
		"status":      "completed",
		"model":       model,
		"output_text": text,
		"output": []map[string]any{{
			"id":      "msg_" + requestID,
			"type":    "message",
			"status":  "completed",
			"role":    "assistant",
			"content": []map[string]any{{"type": "output_text", "text": text}},
		}},
		"usage": map[string]any{
			"input_tokens":        intField(usage, "inputTokens", "input_tokens"),
			"cached_input_tokens": intField(usage, "cachedInputTokens", "cached_input_tokens"),
			"output_tokens":       intField(usage, "outputTokens", "output_tokens"),
			"total_tokens":        intField(usage, "totalTokens", "total_tokens"),
		},
	}
}

func codexErrorPayloadFromGateway(payload map[string]any) map[string]any {
	code := stringField(payload, "error")
	if code == "" {
		code = "request_failed"
	}
	return map[string]any{"error": map[string]any{
		"message": publicCodexErrorMessage(code),
		"type":    "codexppp_error",
		"code":    code,
	}}
}

func writeCodexProviderError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, codexErrorPayloadFromGateway(map[string]any{"error": code}))
}

func publicCodexErrorMessage(code string) string {
	switch code {
	case "login_failed":
		return "登录失败，请重新登录"
	case "token_not_available":
		return "暂时无法继续使用，请刷新状态后重试"
	case "invalid_json", "invalid_gateway_request", "codex_app_server_input_missing", "upstream_rejected_request":
		return "请求暂时无法处理，请调整后重试"
	case "rate_limited":
		return "请求过于频繁，请稍后再试"
	default:
		return "服务暂时不可用，请稍后再试"
	}
}

func codexResponsesSSE(response map[string]any) string {
	id := stringField(response, "id")
	if id == "" {
		id = "resp"
	}
	text := stringField(response, "output_text")
	itemID := codexResponseFirstOutputItemID(response)
	if itemID == "" {
		itemID = "msg_" + strings.TrimPrefix(id, "resp_")
	}
	inProgressItem := map[string]any{"id": itemID, "type": "message", "status": "in_progress", "role": "assistant", "content": []any{}}
	completedItem := map[string]any{"id": itemID, "type": "message", "status": "completed", "role": "assistant", "content": []map[string]any{{"type": "output_text", "text": text}}}
	created := map[string]any{"type": "response.created", "response": map[string]any{"id": id, "object": "response", "status": "in_progress", "output": []any{}}}
	outputAdded := map[string]any{"type": "response.output_item.added", "response_id": id, "output_index": 0, "item": inProgressItem}
	partAdded := map[string]any{"type": "response.content_part.added", "response_id": id, "item_id": itemID, "output_index": 0, "content_index": 0, "part": map[string]any{"type": "output_text", "text": ""}}
	delta := map[string]any{"type": "response.output_text.delta", "response_id": id, "item_id": itemID, "output_index": 0, "content_index": 0, "delta": text}
	textDone := map[string]any{"type": "response.output_text.done", "response_id": id, "item_id": itemID, "output_index": 0, "content_index": 0, "text": text}
	partDone := map[string]any{"type": "response.content_part.done", "response_id": id, "item_id": itemID, "output_index": 0, "content_index": 0, "part": map[string]any{"type": "output_text", "text": text}}
	outputDone := map[string]any{"type": "response.output_item.done", "response_id": id, "output_index": 0, "item": completedItem}
	completed := map[string]any{"type": "response.completed", "response": response}
	return fmt.Sprintf(
		"event: response.created\ndata: %s\n\nevent: response.output_item.added\ndata: %s\n\nevent: response.content_part.added\ndata: %s\n\nevent: response.output_text.delta\ndata: %s\n\nevent: response.output_text.done\ndata: %s\n\nevent: response.content_part.done\ndata: %s\n\nevent: response.output_item.done\ndata: %s\n\nevent: response.completed\ndata: %s\n\ndata: [DONE]\n\n",
		mustJSONLine(created),
		mustJSONLine(outputAdded),
		mustJSONLine(partAdded),
		mustJSONLine(delta),
		mustJSONLine(textDone),
		mustJSONLine(partDone),
		mustJSONLine(outputDone),
		mustJSONLine(completed),
	)
}

func codexResponseFirstOutputItemID(response map[string]any) string {
	items, _ := response["output"].([]map[string]any)
	if len(items) > 0 {
		return stringField(items[0], "id")
	}
	rawItems, _ := response["output"].([]any)
	for _, raw := range rawItems {
		if item, ok := raw.(map[string]any); ok {
			if id := stringField(item, "id"); id != "" {
				return id
			}
		}
	}
	return ""
}

func mustJSONLine(payload any) string {
	raw, err := json.Marshal(payload)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func sanitizeGatewayRequestBody(body []byte) ([]byte, string, string, error) {
	var payload map[string]any
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&payload); err != nil {
		return nil, "", "", err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, "", "", errors.New("invalid_json")
	}
	model := stringField(payload, "model")
	requestID := firstStringField(payload, "requestId", "request_id", "idempotencyKey", "idempotency_key")
	for _, key := range []string{
		"usage", "inputTokens", "input_tokens", "cachedInputTokens", "cached_input_tokens", "outputTokens", "output_tokens", "totalTokens", "total_tokens",
		"requestId", "request_id", "idempotencyKey", "idempotency_key",
		"route", "routeId", "route_id", "apiKey", "api_key", "apiKeyId", "api_key_id", "upstream", "upstreamId", "upstream_id", "gatewayPath", "gateway_path",
		"proxy", "endpoint", "base_url", "baseUrl",
	} {
		delete(payload, key)
	}
	sanitized, err := json.Marshal(payload)
	if err != nil {
		return nil, "", "", err
	}
	return sanitized, model, requestID, nil
}

func extractGatewayUsage(payload any) (gatewayUsage, bool) {
	if payload == nil {
		return gatewayUsage{}, false
	}
	if m, ok := payload.(map[string]any); ok {
		if usage, ok := usageFromMap(m); ok {
			return usage, true
		}
		if usage, ok := usageFromTokenUsageMap(m); ok {
			return usage, true
		}
		for _, key := range []string{"usage", "response_usage"} {
			if nested, ok := m[key].(map[string]any); ok {
				if usage, ok := usageFromMap(nested); ok {
					return usage, true
				}
			}
		}
		if nested, ok := m["tokenUsage"].(map[string]any); ok {
			if usage, ok := usageFromTokenUsageMap(nested); ok {
				return usage, true
			}
		}
		for _, key := range []string{"params", "result", "response", "data"} {
			if usage, ok := extractGatewayUsage(m[key]); ok {
				return usage, true
			}
		}
	}
	if items, ok := payload.([]any); ok {
		for _, item := range items {
			if usage, ok := extractGatewayUsage(item); ok {
				return usage, true
			}
		}
	}
	return gatewayUsage{}, false
}

func usageFromTokenUsageMap(m map[string]any) (gatewayUsage, bool) {
	for _, key := range []string{"last", "total"} {
		if nested, ok := m[key].(map[string]any); ok {
			if usage, ok := usageFromMap(nested); ok {
				return usage, true
			}
		}
	}
	return gatewayUsage{}, false
}

func usageFromMap(m map[string]any) (gatewayUsage, bool) {
	input := intField(m, "input_tokens", "inputTokens", "prompt_tokens", "promptTokens", "input")
	cached := intField(m, "cached_input_tokens", "cachedInputTokens", "cached_tokens", "cachedTokens", "cachedInput")
	if details, ok := m["input_tokens_details"].(map[string]any); ok && cached == 0 {
		cached = intField(details, "cached_tokens", "cachedTokens")
	}
	if details, ok := m["inputTokensDetails"].(map[string]any); ok && cached == 0 {
		cached = intField(details, "cached_tokens", "cachedTokens")
	}
	output := intField(m, "output_tokens", "outputTokens", "completion_tokens", "completionTokens", "output")
	total := intField(m, "total_tokens", "totalTokens", "tokens", "total")
	if total <= 0 && (input > 0 || output > 0) {
		total = input + output
	}
	if total <= 0 && cached > 0 {
		total = cached
	}
	if total <= 0 || input < 0 || cached < 0 || output < 0 {
		return gatewayUsage{}, false
	}
	return gatewayUsage{InputTokens: input, CachedInputTokens: cached, OutputTokens: output, TotalTokens: total}, true
}

func intField(m map[string]any, keys ...string) int64 {
	for _, key := range keys {
		if value, ok := m[key]; ok {
			if parsed, ok := int64FromAny(value); ok {
				return parsed
			}
		}
	}
	return 0
}

func floatField(m map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		if value, ok := m[key]; ok {
			if parsed, ok := float64FromAny(value); ok {
				return parsed, true
			}
		}
	}
	return 0, false
}

func int64FromAny(value any) (int64, bool) {
	switch v := value.(type) {
	case json.Number:
		i, err := v.Int64()
		if err == nil {
			return i, true
		}
		f, err := strconv.ParseFloat(v.String(), 64)
		if err == nil {
			return int64(f), true
		}
	case float64:
		return int64(v), true
	case int64:
		return v, true
	case int:
		return int64(v), true
	}
	return 0, false
}

func float64FromAny(value any) (float64, bool) {
	switch v := value.(type) {
	case json.Number:
		f, err := strconv.ParseFloat(v.String(), 64)
		return f, err == nil
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int64:
		return float64(v), true
	case int:
		return float64(v), true
	}
	return 0, false
}

func stringField(payload any, key string) string {
	if m, ok := payload.(map[string]any); ok {
		if value, ok := m[key].(string); ok {
			return value
		}
	}
	return ""
}

func firstStringField(payload any, keys ...string) string {
	for _, key := range keys {
		if value := stringField(payload, key); value != "" {
			return value
		}
	}
	return ""
}

func nullableString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func resultField(payload any) any {
	if m, ok := payload.(map[string]any); ok {
		if result, ok := m["result"]; ok {
			return result
		}
		if text := firstNonEmptyText(stringField(m, "output_text"), stringField(m, "text"), stringField(m, "message")); text != "" {
			return map[string]any{"text": text}
		}
		for _, key := range []string{"output", "content"} {
			if value, ok := m[key]; ok {
				if text := strings.TrimSpace(textFromAny(value)); text != "" {
					return map[string]any{"text": text}
				}
			}
		}
	}
	return nil
}

func runCodexAppServerTurn(ctx context.Context, credentials codexProbeCredentials, requestBody []byte) (codexRunResult, error) {
	inputText, model, cwd, err := codexRunInput(requestBody)
	if err != nil {
		return codexRunResult{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	cmdName := valueOr(os.Getenv("CODEXPPP_CODEX_COMMAND"), "codex")
	cmd := exec.CommandContext(ctx, cmdName, "app-server", "--listen", "stdio://")
	cmd.Env = append(os.Environ(), "CODEX_ACCESS_TOKEN="+credentials.AccessToken)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return codexRunResult{}, fmt.Errorf("codex_app_server_start_failed")
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return codexRunResult{}, fmt.Errorf("codex_app_server_start_failed")
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return codexRunResult{}, fmt.Errorf("codex_app_server_start_failed")
	}
	defer func() {
		_ = stdin.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}()

	dec := json.NewDecoder(stdout)
	enc := json.NewEncoder(stdin)
	send := func(message map[string]any) error {
		if err := enc.Encode(message); err != nil {
			return fmt.Errorf("codex_app_server_write_failed")
		}
		return nil
	}
	respondToServerRequest := func(msg map[string]any) {
		if _, hasID := msg["id"]; !hasID || stringField(msg, "method") == "" {
			return
		}
		_ = send(map[string]any{"id": msg["id"], "result": map[string]any{"decision": "deny", "reason": "interactive approval is unavailable in the Codex+++ backend gateway"}})
	}
	respondToAuthRefresh := func(msg map[string]any) (bool, error) {
		if method := stringField(msg, "method"); method != "account/chatgptAuthTokens/refresh" {
			return false, nil
		}
		requestID, ok := msg["id"]
		if !ok {
			return true, nil
		}
		if err := send(map[string]any{"id": requestID, "result": codexChatGPTAuthTokensResult(credentials)}); err != nil {
			return true, fmt.Errorf("codex_app_server_write_failed")
		}
		return true, nil
	}
	state := &codexTurnState{}
	readResult := func(id int) (map[string]any, error) {
		for {
			var msg map[string]any
			if err := dec.Decode(&msg); err != nil {
				return nil, fmt.Errorf("codex_app_server_read_failed")
			}
			observeCodexTurnMessage(msg, state)
			if handled, err := respondToAuthRefresh(msg); handled || err != nil {
				if err != nil {
					return nil, err
				}
				continue
			}
			if msgID, ok := int64FromAny(msg["id"]); ok && msgID == int64(id) && stringField(msg, "method") == "" {
				if errObj, ok := msg["error"]; ok && errObj != nil {
					return nil, codexAppServerError(errObj)
				}
				result, _ := msg["result"].(map[string]any)
				if result == nil {
					result = map[string]any{}
				}
				return result, nil
			}
			respondToServerRequest(msg)
		}
	}

	if err := send(map[string]any{"id": 1, "method": "initialize", "params": map[string]any{"clientInfo": map[string]any{"name": "Codex+++", "version": "0.1.0"}, "capabilities": map[string]any{"experimentalApi": true}}}); err != nil {
		return codexRunResult{}, err
	}
	if _, err := readResult(1); err != nil {
		return codexRunResult{}, err
	}
	if err := send(map[string]any{"method": "initialized"}); err != nil {
		return codexRunResult{}, err
	}
	if strings.TrimSpace(credentials.ChatGPTAccountID) == "" {
		return codexRunResult{}, fmt.Errorf("codex_app_server_missing_chatgpt_account_id")
	}
	loginParams := codexChatGPTAuthTokensResult(credentials)
	loginParams["type"] = "chatgptAuthTokens"
	if err := send(map[string]any{"id": 2, "method": "account/login/start", "params": loginParams}); err != nil {
		return codexRunResult{}, err
	}
	if _, err := readResult(2); err != nil {
		return codexRunResult{}, err
	}
	threadParams := map[string]any{}
	if model != "" {
		threadParams["model"] = model
	}
	if cwd != "" {
		threadParams["cwd"] = cwd
	}
	if err := send(map[string]any{"id": 3, "method": "thread/start", "params": threadParams}); err != nil {
		return codexRunResult{}, err
	}
	threadResult, err := readResult(3)
	if err != nil {
		return codexRunResult{}, err
	}
	threadID := codexThreadID(threadResult)
	if threadID == "" {
		return codexRunResult{}, fmt.Errorf("codex_app_server_thread_missing")
	}
	turnParams := map[string]any{
		"threadId": threadID,
		"input":    []map[string]string{{"type": "text", "text": inputText}},
	}
	if model != "" {
		turnParams["model"] = model
	}
	if cwd != "" {
		turnParams["cwd"] = cwd
	}
	if err := send(map[string]any{"id": 4, "method": "turn/start", "params": turnParams}); err != nil {
		return codexRunResult{}, err
	}
	if _, err := readResult(4); err != nil {
		return codexRunResult{}, err
	}
	for {
		var msg map[string]any
		if err := dec.Decode(&msg); err != nil {
			return codexRunResult{}, fmt.Errorf("codex_app_server_read_failed")
		}
		observeCodexTurnMessage(msg, state)
		if handled, err := respondToAuthRefresh(msg); handled || err != nil {
			if err != nil {
				return codexRunResult{}, err
			}
			continue
		}
		respondToServerRequest(msg)
		method := stringField(msg, "method")
		if method == "turn/completed" {
			if err := codexTurnCompletionError(msg); err != nil {
				return codexRunResult{}, err
			}
			break
		}
	}
	if state.Usage.TotalTokens <= 0 {
		return codexRunResult{}, fmt.Errorf("codex_app_server_usage_missing")
	}
	return codexRunResult{Model: valueOr(state.Model, model), Text: strings.TrimSpace(state.Text.String()), Usage: state.Usage, Status: http.StatusOK}, nil
}

type codexTurnState struct {
	Model string
	Text  strings.Builder
	Usage gatewayUsage
}

func codexRunInput(body []byte) (string, string, string, error) {
	var payload map[string]any
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&payload); err != nil {
		return "", "", "", fmt.Errorf("codex_app_server_input_missing")
	}
	model := stringField(payload, "model")
	cwd := stringField(payload, "cwd")
	text := firstNonEmptyText(
		textFromAny(payload["input"]),
		textFromAny(payload["prompt"]),
		textFromAny(payload["message"]),
		textFromAny(payload["messages"]),
	)
	if strings.TrimSpace(text) == "" {
		return "", "", "", fmt.Errorf("codex_app_server_input_missing")
	}
	return strings.TrimSpace(text), model, cwd, nil
}

func firstNonEmptyText(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func textFromAny(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			if text := strings.TrimSpace(textFromAny(item)); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	case map[string]any:
		if text := firstNonEmptyText(stringField(v, "text"), stringField(v, "output_text"), stringField(v, "content")); text != "" {
			return text
		}
		if content, ok := v["content"]; ok {
			return textFromAny(content)
		}
		if message, ok := v["message"]; ok {
			return textFromAny(message)
		}
	}
	return ""
}

func codexThreadID(result map[string]any) string {
	if thread, ok := result["thread"].(map[string]any); ok {
		return stringField(thread, "id")
	}
	return firstStringField(result, "threadId", "id")
}

func observeCodexTurnMessage(msg map[string]any, state *codexTurnState) {
	if state == nil {
		return
	}
	if usage, ok := extractGatewayUsage(msg); ok {
		state.Usage = usage
	}
	if model := firstStringField(msg, "model"); model != "" {
		state.Model = model
	}
	params, _ := msg["params"].(map[string]any)
	if params == nil {
		return
	}
	if usage, ok := extractGatewayUsage(params); ok {
		state.Usage = usage
	}
	if model := firstStringField(params, "model"); model != "" {
		state.Model = model
	}
	method := stringField(msg, "method")
	if method == "item/agentMessage/delta" {
		if delta := firstNonEmptyText(stringField(params, "delta"), stringField(params, "text")); delta != "" {
			state.Text.WriteString(delta)
		}
	}
	if method == "item/completed" || method == "rawResponseItem/completed" {
		if item, ok := params["item"].(map[string]any); ok {
			if text := codexCompletedOutputText(item); text != "" && !strings.Contains(state.Text.String(), text) {
				if state.Text.Len() > 0 {
					state.Text.WriteString("\n")
				}
				state.Text.WriteString(text)
			}
			if model := firstStringField(item, "model"); model != "" {
				state.Model = model
			}
		}
	}
}

func codexCompletedOutputText(item map[string]any) string {
	if !codexItemLooksLikeAssistantOutput(item) {
		return ""
	}
	return strings.TrimSpace(textFromAny(item))
}

func codexItemLooksLikeAssistantOutput(item map[string]any) bool {
	role := strings.ToLower(firstNonEmptyText(stringField(item, "role"), stringField(item, "author")))
	if role == "assistant" || role == "codex" {
		return true
	}
	itemType := strings.ToLower(stringField(item, "type"))
	if strings.Contains(itemType, "assistant") || strings.Contains(itemType, "agent") {
		return true
	}
	return containsCodexOutputTextPart(item)
}

func containsCodexOutputTextPart(value any) bool {
	switch v := value.(type) {
	case []any:
		for _, item := range v {
			if containsCodexOutputTextPart(item) {
				return true
			}
		}
	case []map[string]any:
		for _, item := range v {
			if containsCodexOutputTextPart(item) {
				return true
			}
		}
	case map[string]any:
		if strings.EqualFold(stringField(v, "type"), "output_text") {
			return true
		}
		for _, key := range []string{"content", "item", "message", "parts"} {
			if containsCodexOutputTextPart(v[key]) {
				return true
			}
		}
	}
	return false
}

func codexTurnCompletionError(msg map[string]any) error {
	params, _ := msg["params"].(map[string]any)
	if params == nil {
		return nil
	}
	if errObj, ok := params["error"]; ok && errObj != nil {
		return codexAppServerError(errObj)
	}
	if turn, ok := params["turn"].(map[string]any); ok {
		if errObj, ok := turn["error"]; ok && errObj != nil {
			return codexAppServerError(errObj)
		}
		status := stringField(turn, "status")
		if status == "failed" || status == "interrupted" || status == "cancelled" {
			return fmt.Errorf("codex_app_server_turn_%s", status)
		}
	}
	return nil
}

func codexAppServerError(errObj any) error {
	if m, ok := errObj.(map[string]any); ok {
		if status := codexKnownErrorHTTPStatus(m); status > 0 {
			return fmt.Errorf("codex_app_server_http_%d", status)
		}
		if status := intField(m, "httpStatusCode", "status", "statusCode"); status > 0 {
			return fmt.Errorf("codex_app_server_http_%d", status)
		}
		if status := codexErrorInfoHTTPStatus(m["codexErrorInfo"]); status > 0 {
			return fmt.Errorf("codex_app_server_http_%d", status)
		}
		if data, ok := m["data"].(map[string]any); ok {
			if status := intField(data, "httpStatusCode", "status", "statusCode"); status > 0 {
				return fmt.Errorf("codex_app_server_http_%d", status)
			}
			if status := codexErrorInfoHTTPStatus(data["codexErrorInfo"]); status > 0 {
				return fmt.Errorf("codex_app_server_http_%d", status)
			}
		}
	}
	return fmt.Errorf("codex_app_server_error")
}

func codexKnownErrorHTTPStatus(value any) int {
	switch v := value.(type) {
	case string:
		return codexKnownErrorStringHTTPStatus(v)
	case map[string]any:
		for _, key := range []string{"code", "type", "status", "reason", "codexErrorInfo", "message"} {
			if status := codexKnownErrorHTTPStatus(v[key]); status > 0 {
				return status
			}
		}
		for _, nested := range v {
			switch nested.(type) {
			case map[string]any, []any:
				if status := codexKnownErrorHTTPStatus(nested); status > 0 {
					return status
				}
			}
		}
	case []any:
		for _, nested := range v {
			if status := codexKnownErrorHTTPStatus(nested); status > 0 {
				return status
			}
		}
	}
	return 0
}

func codexKnownErrorStringHTTPStatus(value string) int {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")
	switch {
	case normalized == "usagelimitexceeded",
		normalized == "usage_limit_exceeded",
		normalized == "insufficient_quota",
		normalized == "quota_exhausted",
		strings.Contains(normalized, "exceeded_your_current_quota"),
		strings.Contains(normalized, "quota") && strings.Contains(normalized, "exceeded"):
		return http.StatusPaymentRequired
	case normalized == "rate_limited",
		normalized == "rate_limit_exceeded",
		strings.Contains(normalized, "rate_limit_reached"),
		strings.Contains(normalized, "rate_limit"):
		return http.StatusTooManyRequests
	case normalized == "unauthorized" || normalized == "authentication_failed":
		return http.StatusUnauthorized
	case normalized == "permission_denied" || normalized == "forbidden":
		return http.StatusForbidden
	case normalized == "serveroverloaded" || normalized == "server_overloaded":
		return http.StatusServiceUnavailable
	case normalized == "internalservererror" || normalized == "internal_server_error":
		return http.StatusInternalServerError
	case normalized == "badrequest" || normalized == "bad_request" || normalized == "contextwindowexceeded" || normalized == "context_window_exceeded" || normalized == "cyberpolicy" || normalized == "cyber_policy":
		return http.StatusBadRequest
	}
	return 0
}

func codexErrorInfoHTTPStatus(info any) int {
	switch v := info.(type) {
	case string:
		if status := codexKnownErrorStringHTTPStatus(v); status > 0 {
			return status
		}
	case map[string]any:
		if status := intField(v, "httpStatusCode", "status", "statusCode"); status > 0 {
			return int(status)
		}
		for _, nested := range v {
			if status := codexErrorInfoHTTPStatus(nested); status > 0 {
				return status
			}
		}
	}
	return 0
}

func upstreamProbeFailureStatus(err error) string {
	if err == nil {
		return "check_failed"
	}
	code := err.Error()
	switch {
	case strings.Contains(code, "codex_app_server_missing_chatgpt_account_id"):
		return "missing_chatgpt_account_id"
	case strings.Contains(code, "codex_app_server_start_failed"):
		return "check_service_start_failed"
	case strings.Contains(code, "codex_app_server_read_failed"), strings.Contains(code, "codex_app_server_write_failed"):
		return "check_service_unavailable"
	case strings.Contains(code, "codex_app_server_http_401"), strings.Contains(code, "codex_app_server_http_403"):
		return "auth_failed"
	case strings.Contains(code, "codex_app_server_http_402"):
		return "usage_limit"
	case strings.Contains(code, "codex_app_server_http_429"):
		return "rate_limited"
	case strings.Contains(code, "codex_app_server_http_503"):
		return "service_overloaded"
	default:
		return "check_failed"
	}
}

func codexChatGPTAuthTokensResult(credentials codexProbeCredentials) map[string]any {
	return map[string]any{
		"accessToken":      credentials.AccessToken,
		"chatgptAccountId": credentials.ChatGPTAccountID,
		"chatgptPlanType":  nullableString(credentials.ChatGPTPlanType),
	}
}

func runCodexAppServerProbe(ctx context.Context, credentials codexProbeCredentials) (codexProbeResult, error) {
	cmdName := valueOr(os.Getenv("CODEXPPP_CODEX_COMMAND"), "codex")
	cmd := exec.CommandContext(ctx, cmdName, "app-server", "--listen", "stdio://")
	cmd.Env = os.Environ()
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return codexProbeResult{}, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return codexProbeResult{}, err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return codexProbeResult{}, fmt.Errorf("codex_app_server_start_failed")
	}
	defer func() {
		_ = stdin.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}()
	dec := json.NewDecoder(stdout)
	enc := json.NewEncoder(stdin)
	send := func(message map[string]any) error {
		return enc.Encode(message)
	}
	readResult := func(id int) (map[string]any, error) {
		for {
			var msg map[string]any
			if err := dec.Decode(&msg); err != nil {
				return nil, fmt.Errorf("codex_app_server_read_failed")
			}
			if method, _ := msg["method"].(string); method == "account/chatgptAuthTokens/refresh" {
				if requestID, ok := msg["id"]; ok {
					if err := send(map[string]any{"id": requestID, "result": codexChatGPTAuthTokensResult(credentials)}); err != nil {
						return nil, fmt.Errorf("codex_app_server_write_failed")
					}
				}
				continue
			}
			msgID, ok := int64FromAny(msg["id"])
			if !ok || msgID != int64(id) {
				continue
			}
			if errObj, ok := msg["error"]; ok && errObj != nil {
				return nil, codexAppServerError(errObj)
			}
			result, _ := msg["result"].(map[string]any)
			if result == nil {
				result = map[string]any{}
			}
			return result, nil
		}
	}
	if err := send(map[string]any{"id": 1, "method": "initialize", "params": map[string]any{"clientInfo": map[string]any{"name": "Codex+++", "version": "0.1.0"}, "capabilities": map[string]any{"experimentalApi": true}}}); err != nil {
		return codexProbeResult{}, fmt.Errorf("codex_app_server_write_failed")
	}
	if _, err := readResult(1); err != nil {
		return codexProbeResult{}, err
	}
	if err := send(map[string]any{"method": "initialized"}); err != nil {
		return codexProbeResult{}, fmt.Errorf("codex_app_server_write_failed")
	}
	if strings.TrimSpace(credentials.ChatGPTAccountID) == "" {
		return codexProbeResult{}, fmt.Errorf("codex_app_server_missing_chatgpt_account_id")
	}
	loginParams := codexChatGPTAuthTokensResult(credentials)
	loginParams["type"] = "chatgptAuthTokens"
	if err := send(map[string]any{"id": 2, "method": "account/login/start", "params": loginParams}); err != nil {
		return codexProbeResult{}, fmt.Errorf("codex_app_server_write_failed")
	}
	if _, err := readResult(2); err != nil {
		return codexProbeResult{}, err
	}
	if err := send(map[string]any{"id": 3, "method": "account/read", "params": map[string]any{"refreshToken": false}}); err != nil {
		return codexProbeResult{}, fmt.Errorf("codex_app_server_write_failed")
	}
	accountResult, err := readResult(3)
	if err != nil {
		return codexProbeResult{}, err
	}
	if err := send(map[string]any{"id": 4, "method": "account/rateLimits/read"}); err != nil {
		return codexProbeResult{}, fmt.Errorf("codex_app_server_write_failed")
	}
	rateResult, err := readResult(4)
	if err != nil {
		return codexProbeResult{}, err
	}
	if err := send(map[string]any{"id": 5, "method": "account/usage/read"}); err != nil {
		return codexProbeResult{}, fmt.Errorf("codex_app_server_write_failed")
	}
	usageResult, err := readResult(5)
	if err != nil {
		return codexProbeResult{}, err
	}
	return parseCodexProbeResult(accountResult, rateResult, usageResult), nil
}

func parseCodexProbeResult(accountResult, rateResult, usageResult map[string]any) codexProbeResult {
	var out codexProbeResult
	if account, ok := accountResult["account"].(map[string]any); ok {
		out.AccountType = stringField(account, "type")
		out.Email = stringField(account, "email")
		out.PlanType = stringField(account, "planType")
	}
	out.RateLimitReachedType = findRateLimitReachedType(rateResult)
	if bucket := primaryCodexRateLimitBucket(rateResult); bucket != nil {
		if primary, ok := bucket["primary"].(map[string]any); ok {
			if used, ok := floatField(primary, "usedPercent"); ok {
				out.RateLimitUsedPercent = &used
			}
			if resetsAt, ok := importValueField(primary, "resetsAt"); ok {
				if parsed, err := parseImportExpiresAt(resetsAt); err == nil {
					out.RateLimitResetsAt = parsed
				}
			}
		} else {
			if used, ok := floatField(bucket, "usedPercent"); ok {
				out.RateLimitUsedPercent = &used
			}
			if resetsAt, ok := importValueField(bucket, "resetsAt"); ok {
				if parsed, err := parseImportExpiresAt(resetsAt); err == nil {
					out.RateLimitResetsAt = parsed
				}
			}
		}
		out.CreditBalance, out.CreditBalanceLabel = parseCreditBalance(bucket["credits"])
	}
	if buckets, ok := usageResult["dailyUsageBuckets"].([]any); ok && len(buckets) > 0 {
		if latest, ok := buckets[len(buckets)-1].(map[string]any); ok {
			out.UsageTokens = intField(latest, "tokens")
		}
	}
	return out
}

func primaryCodexRateLimitBucket(rateResult map[string]any) map[string]any {
	if byID, ok := rateResult["rateLimitsByLimitId"].(map[string]any); ok {
		if codex, ok := byID["codex"].(map[string]any); ok {
			return codex
		}
		for _, raw := range byID {
			if bucket, ok := raw.(map[string]any); ok {
				return bucket
			}
		}
	}
	if limits, ok := rateResult["rateLimits"].(map[string]any); ok {
		return limits
	}
	return nil
}

func parseCreditBalance(value any) (*float64, string) {
	if value == nil {
		return nil, ""
	}
	if numeric, ok := float64FromAny(value); ok {
		return &numeric, strconv.FormatFloat(numeric, 'f', -1, 64)
	}
	credits, ok := value.(map[string]any)
	if !ok {
		return nil, ""
	}
	label := firstStringField(credits, "display", "label", "formatted", "formattedBalance", "text")
	for _, key := range []string{"remaining", "available", "balance", "remainingCredits", "availableCredits", "creditBalance", "amount", "value"} {
		if raw, ok := credits[key]; ok {
			if numeric, ok := float64FromAny(raw); ok {
				if label == "" {
					label = strconv.FormatFloat(numeric, 'f', -1, 64)
				}
				return &numeric, label
			}
		}
	}
	return nil, label
}

func findRateLimitReachedType(rateResult map[string]any) string {
	if byID, ok := rateResult["rateLimitsByLimitId"].(map[string]any); ok {
		if codex, ok := byID["codex"].(map[string]any); ok {
			if value := stringField(codex, "rateLimitReachedType"); value != "" {
				return value
			}
		}
		for _, raw := range byID {
			if bucket, ok := raw.(map[string]any); ok {
				if value := stringField(bucket, "rateLimitReachedType"); value != "" {
					return value
				}
			}
		}
	}
	if limits, ok := rateResult["rateLimits"].(map[string]any); ok {
		return stringField(limits, "rateLimitReachedType")
	}
	return ""
}

func (a *App) requireAdmin(w http.ResponseWriter, r *http.Request, allowPasswordChange bool) (Admin, bool) {
	token := bearerToken(r)
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	session, ok := a.sessionLocked(token, sessionRoleAdmin)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "login_failed")
		return Admin{}, false
	}
	admin := a.adminByID(session.SubjectID)
	if admin.ID == "" {
		writeErr(w, http.StatusUnauthorized, "login_failed")
		return Admin{}, false
	}
	if admin.MustChangePassword && !allowPasswordChange {
		writeErr(w, http.StatusConflict, "password_change_required")
		return Admin{}, false
	}
	return admin, true
}

func (a *App) requireClient(w http.ResponseWriter, r *http.Request) (User, Device, bool) {
	user, device, ok := a.clientFromRequest(r)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "login_failed")
		return User{}, Device{}, false
	}
	return user, device, true
}

func (a *App) clientFromRequest(r *http.Request) (User, Device, bool) {
	token := bearerToken(r)
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	session, ok := a.sessionLocked(token, sessionRoleClient)
	if !ok {
		return User{}, Device{}, false
	}
	return a.userDeviceFromSessionLocked(session)
}

func (a *App) codexClientFromRequest(r *http.Request) (User, Device, bool) {
	token := bearerToken(r)
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	session, ok := a.sessionLocked(token, sessionRoleCodexProvider)
	if !ok {
		return User{}, Device{}, false
	}
	return a.userDeviceFromSessionLocked(session)
}

func (a *App) userDeviceFromSessionLocked(session Session) (User, Device, bool) {
	user := a.userByID(session.SubjectID)
	device := a.deviceByID(session.DeviceID)
	if user.ID == "" || device.ID == "" || user.Status != statusActive || device.Status != statusActive {
		return User{}, Device{}, false
	}
	return user, device, true
}

func (a *App) sessionLocked(token, role string) (Session, bool) {
	now := time.Now().UTC()
	for _, session := range a.store.state.Sessions {
		if session.Token == token && session.Role == role && session.ExpiresAt.After(now) {
			return session, true
		}
	}
	return Session{}, false
}

func (a *App) createSessionLocked(role, subjectID, deviceID string) string {
	token := randomToken(32)
	a.createSessionWithTokenLocked(token, role, subjectID, deviceID)
	return token
}

func (a *App) createSessionWithTokenLocked(token, role, subjectID, deviceID string) {
	now := time.Now().UTC()
	a.store.state.Sessions = append(a.store.state.Sessions, Session{Token: token, Role: role, SubjectID: subjectID, DeviceID: deviceID, CreatedAt: now, ExpiresAt: now.Add(30 * 24 * time.Hour)})
}

func (a *App) codexProviderSessionTokenLocked(userID, deviceID string) (string, error) {
	now := time.Now().UTC()
	kept := a.store.state.Sessions[:0]
	reusableToken := ""
	for _, session := range a.store.state.Sessions {
		if session.Role == sessionRoleCodexProvider && session.SubjectID == userID && session.DeviceID == deviceID {
			if reusableToken == "" && session.ExpiresAt.After(now) && isSub2APIStyleKey(session.Token) {
				reusableToken = session.Token
				kept = append(kept, session)
			}
			continue
		}
		kept = append(kept, session)
	}
	a.store.state.Sessions = kept
	if reusableToken != "" {
		return reusableToken, nil
	}
	token, err := generateSub2APIKey()
	if err != nil {
		return "", err
	}
	a.createSessionWithTokenLocked(token, sessionRoleCodexProvider, userID, deviceID)
	return token, nil
}

func (a *App) applyTokenDeltaLocked(userID string, delta int64, typ, source string) error {
	idx := a.userIndex(userID)
	if idx < 0 {
		return errors.New("user_not_found")
	}
	next := a.store.state.Users[idx].TokenBalance + delta
	if next < 0 {
		return errors.New("token_not_available")
	}
	a.store.state.Users[idx].TokenBalance = next
	a.store.state.Users[idx].UpdatedAt = time.Now().UTC()
	a.store.state.TokenLedgers = append(a.store.state.TokenLedgers, TokenLedger{ID: a.store.nextID("led"), UserID: userID, Type: typ, DeltaTokens: delta, BalanceAfter: next, Source: source, CreatedAt: time.Now().UTC()})
	return nil
}

func (a *App) availableTokenBalanceLocked(userID string) int64 {
	user := a.userByID(userID)
	if user.ID == "" {
		return 0
	}
	available := user.TokenBalance
	for _, req := range a.store.state.GatewayRequests {
		if req.UserID == userID && req.Status == gatewayReserved {
			available -= req.ReservedTokens
		}
	}
	if available < 0 {
		return 0
	}
	return available
}

func (a *App) gatewayRequestIndexLocked(userID, requestID string) int {
	for i, req := range a.store.state.GatewayRequests {
		if req.UserID == userID && req.RequestID == requestID {
			return i
		}
	}
	return -1
}

func (a *App) usageRecordByIDLocked(id string) UsageRecord {
	for _, rec := range a.store.state.UsageRecords {
		if rec.ID == id {
			return rec
		}
	}
	return UsageRecord{}
}

func (a *App) existingGatewayResponseLocked(userID, requestID string) (int, any, bool) {
	existingIdx := a.gatewayRequestIndexLocked(userID, requestID)
	if existingIdx < 0 {
		return 0, nil, false
	}
	existing := a.store.state.GatewayRequests[existingIdx]
	switch existing.Status {
	case gatewayCompleted:
		rec := a.usageRecordByIDLocked(existing.UsageRecordID)
		if rec.ID == "" {
			return http.StatusInternalServerError, apiError{Error: "idempotency_record_corrupt"}, true
		}
		var result any
		if existing.ResultText != "" {
			result = map[string]any{"text": existing.ResultText}
		}
		return http.StatusOK, gatewayRunResponse(requestID, rec, existing.UpstreamStatus, existing.ChargedTokens, true, result), true
	case gatewayReserved:
		return http.StatusConflict, apiError{Error: "request_in_progress"}, true
	default:
		return 0, nil, false
	}
}

func (a *App) allowGatewayRequest(w http.ResponseWriter, r *http.Request, userID string) bool {
	if a.gatewayLimiter == nil || a.gatewayRateLimit <= 0 {
		return true
	}
	window := a.gatewayRateWindow
	if window <= 0 {
		window = time.Minute
	}
	allowed, err := a.gatewayLimiter.Allow(r.Context(), "codexppp:gateway:user:"+userID, a.gatewayRateLimit, window)
	if err != nil {
		writeErr(w, http.StatusServiceUnavailable, "rate_limiter_unavailable")
		return false
	}
	if !allowed {
		writeErr(w, http.StatusTooManyRequests, "rate_limited")
		return false
	}
	return true
}

func (a *App) failGatewayRequest(userID, requestID string, upstreamStatus int, errCode string) error {
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	a.markGatewayRequestFailedLocked(userID, requestID, upstreamStatus, errCode)
	return a.store.save()
}

func (a *App) markGatewayRequestFailedLocked(userID, requestID string, upstreamStatus int, errCode string) {
	idx := a.gatewayRequestIndexLocked(userID, requestID)
	if idx < 0 {
		return
	}
	req := &a.store.state.GatewayRequests[idx]
	released := req.ReservedTokens
	req.Status = gatewayFailed
	req.ReservedTokens = 0
	req.UpstreamStatus = upstreamStatus
	req.Error = errCode
	req.UpdatedAt = time.Now().UTC()
	a.auditLocked(userID, "client", "gateway.request.release", requestID, fmt.Sprintf("released_tokens=%d error=%s upstream_status=%d", released, errCode, upstreamStatus))
}

func (a *App) markRouteUnavailable(route gatewayRoute, reason string, upstreamStatus int) error {
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	a.markRouteUnavailableLocked(route, reason, upstreamStatus)
	return a.store.save()
}

func (a *App) markRouteUnavailableLocked(route gatewayRoute, reason string, upstreamStatus int) {
	idx := a.upstreamIndex(route.Upstream.ID)
	if idx < 0 {
		return
	}
	up := &a.store.state.UpstreamAccounts[idx]
	now := time.Now().UTC()
	if reason == "upstream_balance_unavailable" || reason == "upstream_limited" {
		up.BalanceStatus = "unavailable"
	} else {
		up.RiskStatus = "unavailable"
	}
	up.LastCheckedAt = &now
	up.UpdatedAt = now
	keyID := valueOr(route.Key.ID, "auto")
	detail := fmt.Sprintf("key=%s reason=%s upstream_status=%d", keyID, reason, upstreamStatus)
	a.auditLocked("system", "system", "gateway.route.unavailable", up.ID, detail)
}

func (a *App) routeCandidatesLocked() []gatewayRoute {
	routes := make([]gatewayRoute, 0)
	if len(a.store.state.APIKeys) == 0 {
		for _, up := range a.store.state.UpstreamAccounts {
			if upstreamIsAvailable(up) {
				routes = append(routes, gatewayRoute{
					Key:      APIKey{UpstreamAccountID: up.ID, Status: statusActive, CreatedAt: up.CreatedAt},
					Upstream: up,
				})
			}
		}
		return routes
	}
	for _, key := range a.store.state.APIKeys {
		if key.Status != statusActive {
			continue
		}
		up := a.upstreamByID(key.UpstreamAccountID)
		if up.ID == "" {
			continue
		}
		if upstreamIsAvailable(up) {
			routes = append(routes, gatewayRoute{Key: key, Upstream: up})
		}
	}
	sort.SliceStable(routes, func(i, j int) bool {
		left := routes[i].Key.LastUsedAt
		right := routes[j].Key.LastUsedAt
		if left == nil && right == nil {
			return routes[i].Key.CreatedAt.Before(routes[j].Key.CreatedAt)
		}
		if left == nil {
			return true
		}
		if right == nil {
			return false
		}
		if left.Equal(*right) {
			return routes[i].Key.CreatedAt.Before(routes[j].Key.CreatedAt)
		}
		return left.Before(*right)
	})
	return routes
}

func (a *App) publicUserLocked(user User) map[string]any {
	return map[string]any{
		"id": user.ID, "account": user.Account, "status": user.Status, "tokenBalance": user.TokenBalance,
		"recentRechargeStatus": a.recentRechargeStatusLocked(user.ID), "lastLoginAt": user.LastLoginAt, "createdAt": user.CreatedAt,
	}
}

func (a *App) publicClientUserLocked(user User) map[string]any {
	return map[string]any{
		"account":              user.Account,
		"tokenBalance":         user.TokenBalance,
		"recentRechargeStatus": a.recentRechargeStatusLocked(user.ID),
	}
}

func publicClientDevice(device Device) map[string]any {
	return map[string]any{
		"status": availabilityLabel(device.Status),
	}
}

func publicClientSecurity(user User, device Device) map[string]any {
	return map[string]any{
		"accountStatus": availabilityLabel(user.Status),
		"deviceStatus":  availabilityLabel(device.Status),
		"sessionStatus": "可用",
	}
}

func (a *App) publicClientServiceLocked(user User) map[string]any {
	routes := a.routeCandidatesLocked()
	available := user.Status == statusActive && a.availableTokenBalanceLocked(user.ID) > 0 && len(routes) > 0
	return map[string]any{"status": availabilityLabel(boolStatus(available))}
}

func boolStatus(value bool) string {
	if value {
		return statusActive
	}
	return statusDisabled
}

func (a *App) publicAdminDeviceLocked(device Device) map[string]any {
	user := a.userByID(device.UserID)
	return map[string]any{
		"id":          device.ID,
		"userId":      device.UserID,
		"userAccount": user.Account,
		"name":        device.Name,
		"status":      device.Status,
		"lastSeenAt":  device.LastSeenAt,
		"createdAt":   device.CreatedAt,
	}
}

func (a *App) publicAdminUsageLocked(rec UsageRecord) map[string]any {
	user := a.userByID(rec.UserID)
	userAccount := user.Account
	if userAccount == "" {
		userAccount = rec.UserID
	}
	return map[string]any{
		"id":                rec.ID,
		"userId":            rec.UserID,
		"userAccount":       userAccount,
		"model":             rec.Model,
		"inputTokens":       rec.InputTokens,
		"cachedInputTokens": rec.CachedInputTokens,
		"outputTokens":      rec.OutputTokens,
		"totalTokens":       rec.TotalTokens,
		"createdAt":         rec.CreatedAt,
	}
}

func (a *App) publicAdminAuditLocked(item AuditLog) map[string]any {
	actorAccount := ""
	switch item.ActorRole {
	case "admin":
		actorAccount = a.adminByID(item.ActorID).Account
	case "client":
		actorAccount = a.userByID(item.ActorID).Account
	case "system":
		actorAccount = "system"
	}
	return map[string]any{
		"id":           item.ID,
		"actorId":      item.ActorID,
		"actorRole":    item.ActorRole,
		"actorAccount": actorAccount,
		"action":       item.Action,
		"targetId":     item.TargetID,
		"detail":       item.Detail,
		"createdAt":    item.CreatedAt,
	}
}

func availabilityLabel(status string) string {
	if status == statusActive {
		return "可用"
	}
	return "不可用"
}

func (a *App) publicAdminTokenLedgerLocked(rec TokenLedger) map[string]any {
	user := a.userByID(rec.UserID)
	userAccount := user.Account
	if userAccount == "" {
		userAccount = rec.UserID
	}
	label := "消耗"
	if rec.DeltaTokens >= 0 {
		label = "收入"
	}
	return map[string]any{
		"id":           rec.ID,
		"userId":       rec.UserID,
		"userAccount":  userAccount,
		"type":         rec.Type,
		"typeLabel":    label,
		"deltaTokens":  rec.DeltaTokens,
		"balanceAfter": rec.BalanceAfter,
		"source":       publicAdminLedgerSource(rec),
		"createdAt":    rec.CreatedAt,
	}
}

func publicAdminLedgerSource(rec TokenLedger) string {
	source := strings.TrimSpace(rec.Source)
	switch rec.Type {
	case "recharge":
		if source == "" || containsSensitiveLedgerText(source) {
			return "Token 充值项"
		}
		return source
	case "debit":
		if source == "" || containsSensitiveLedgerText(source) {
			return "Codex token 用量"
		}
		if strings.HasPrefix(source, "Codex token 用量") {
			return source
		}
		return "Codex token 用量"
	case "adjustment":
		if source == "" || containsSensitiveLedgerText(source) {
			return "管理员调整"
		}
		if strings.HasPrefix(source, "管理员调整") {
			return source
		}
		return "管理员调整：" + source
	default:
		return "系统记录"
	}
}

func publicClientTokenLedger(rec TokenLedger) map[string]any {
	label := "消耗"
	if rec.DeltaTokens >= 0 {
		label = "收入"
	}
	return map[string]any{"type": rec.Type, "typeLabel": label, "deltaTokens": rec.DeltaTokens, "balanceAfter": rec.BalanceAfter, "source": publicClientLedgerSource(rec), "createdAt": rec.CreatedAt}
}

func publicClientLedgerSource(rec TokenLedger) string {
	switch rec.Type {
	case "recharge":
		return "Token 充值项"
	case "debit":
		return "Codex token 用量"
	case "adjustment":
		return publicClientAdjustmentSource(rec.Source)
	default:
		return "系统记录"
	}
}

func publicClientAdjustmentSource(source string) string {
	const label = "管理员调整"
	remark := strings.TrimSpace(source)
	remark = strings.TrimPrefix(remark, label)
	remark = strings.TrimSpace(remark)
	remark = strings.TrimPrefix(remark, "：")
	remark = strings.TrimPrefix(remark, ":")
	remark = strings.TrimSpace(remark)
	if remark == "" || containsSensitiveLedgerText(remark) {
		return label
	}
	return label + "：" + remark
}

func containsSensitiveLedgerText(value string) bool {
	lower := strings.ToLower(value)
	for _, part := range []string{
		"internal route", "c:\\users", "token=", "key=", "secret", "upstream",
		"proxy", "endpoint", "base_url", "api key", "authorization", "bearer",
		"access_token", "refresh_token",
	} {
		if strings.Contains(lower, part) {
			return true
		}
	}
	return false
}

func publicClientUsage(rec UsageRecord) map[string]any {
	return map[string]any{
		"model":             rec.Model,
		"inputTokens":       rec.InputTokens,
		"cachedInputTokens": rec.CachedInputTokens,
		"outputTokens":      rec.OutputTokens,
		"totalTokens":       rec.TotalTokens,
		"createdAt":         rec.CreatedAt,
	}
}

func validTopupPayload(w http.ResponseWriter, topup TokenTopup) bool {
	if strings.TrimSpace(topup.Name) == "" {
		writeErr(w, http.StatusBadRequest, "invalid_topup_name")
		return false
	}
	if topup.PriceCents < 0 {
		writeErr(w, http.StatusBadRequest, "invalid_topup_price")
		return false
	}
	if topup.Tokens < 0 {
		writeErr(w, http.StatusBadRequest, "invalid_topup_tokens")
		return false
	}
	if topup.PriceCents > 0 && topup.Tokens == 0 {
		writeErr(w, http.StatusBadRequest, "invalid_topup_tokens")
		return false
	}
	return true
}

func publicClientTopup(topup TokenTopup) map[string]any {
	return map[string]any{"id": topup.ID, "name": topup.Name, "priceCents": topup.PriceCents, "tokens": topup.Tokens}
}

func publicClientRecharge(rr RechargeRequest) map[string]any {
	label := rechargeStatusLabel(rr.Status)
	return map[string]any{"topupName": rr.TopupName, "status": rr.Status, "statusLabel": label, "submittedAt": rr.SubmittedAt, "confirmedAt": rr.ConfirmedAt}
}

func (a *App) publicRechargeLocked(rr RechargeRequest) map[string]any {
	user := a.userByID(rr.UserID)
	label := rechargeStatusLabel(rr.Status)
	return map[string]any{"id": rr.ID, "userId": rr.UserID, "userAccount": user.Account, "topupId": rr.TopupID, "topupName": rr.TopupName, "priceCents": rr.PriceCents, "tokens": rr.Tokens, "status": rr.Status, "statusLabel": label, "submittedAt": rr.SubmittedAt, "confirmedAt": rr.ConfirmedAt}
}

func (a *App) publicAdminRechargeLocked(rr RechargeRequest) map[string]any {
	out := a.publicRechargeLocked(rr)
	out["statusTransitions"] = ensureRechargeTransitions(rr)
	return out
}

func (a *App) recentRechargeStatusLocked(userID string) string {
	var latest *RechargeRequest
	for i := range a.store.state.RechargeRequests {
		rr := &a.store.state.RechargeRequests[i]
		if rr.UserID != userID {
			continue
		}
		if latest == nil || rr.SubmittedAt.After(latest.SubmittedAt) || rr.SubmittedAt.Equal(latest.SubmittedAt) {
			latest = rr
		}
	}
	if latest == nil {
		return "无"
	}
	return rechargeStatusLabel(latest.Status)
}

func rechargeStatusLabel(status string) string {
	switch status {
	case rechargePending:
		return "等待管理员确认"
	case rechargeApproved:
		return "已确认"
	case rechargeRejected:
		return "已拒绝"
	case rechargeCancelled:
		return "已取消"
	default:
		return "未知"
	}
}

func (a *App) usageDailyLocked(userID string, days int) []map[string]any {
	now := time.Now().UTC()
	out := make([]map[string]any, 0, days)
	for i := days - 1; i >= 0; i-- {
		day := now.AddDate(0, 0, -i)
		start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
		end := start.Add(24 * time.Hour)
		var total int64
		for _, rec := range a.store.state.UsageRecords {
			if rec.UserID == userID && !rec.CreatedAt.Before(start) && rec.CreatedAt.Before(end) {
				total += rec.TotalTokens
			}
		}
		out = append(out, map[string]any{"date": start.Format("2006-01-02"), "tokens": total})
	}
	return out
}

func (a *App) auditLocked(actorID, role, action, targetID, detail string) {
	a.store.state.AuditLogs = append(a.store.state.AuditLogs, AuditLog{ID: a.store.nextID("aud"), ActorID: actorID, ActorRole: role, Action: action, TargetID: sanitizeAuditText(targetID), Detail: sanitizeAuditText(detail), CreatedAt: time.Now().UTC()})
}

func sanitizeAuditText(value string) string {
	if containsSensitiveAuditText(value) {
		return "redacted"
	}
	return value
}

func containsSensitiveAuditText(value string) bool {
	lower := strings.ToLower(value)
	for _, part := range []string{
		"token=",
		"access token",
		"refresh token",
		"access_token=",
		"refresh_token=",
		"accesstoken=",
		"refreshtoken=",
		"authorization:",
		"authorization=",
		"bearer ",
		"api key",
		"apikey",
		"api-key",
		"secret",
		"password",
		"credential=",
		"c:\\users",
		"proxy=",
		"endpoint=",
		"base_url=",
		"baseurl=",
		"gateway_url=",
		"gatewayurl=",
	} {
		if strings.Contains(lower, part) {
			return true
		}
	}
	return false
}

func (a *App) adminIndex(id string) int {
	for i, admin := range a.store.state.Admins {
		if admin.ID == id {
			return i
		}
	}
	return -1
}

func (a *App) adminByID(id string) Admin {
	for _, admin := range a.store.state.Admins {
		if admin.ID == id {
			return admin
		}
	}
	return Admin{}
}

func (a *App) userIndex(id string) int {
	for i, user := range a.store.state.Users {
		if user.ID == id {
			return i
		}
	}
	return -1
}

func (a *App) userByID(id string) User {
	for _, user := range a.store.state.Users {
		if user.ID == id {
			return user
		}
	}
	return User{}
}

func (a *App) deviceByID(id string) Device {
	for _, device := range a.store.state.Devices {
		if device.ID == id {
			return device
		}
	}
	return Device{}
}

func (a *App) deviceIndex(id string) int {
	for i, device := range a.store.state.Devices {
		if device.ID == id {
			return i
		}
	}
	return -1
}

func (a *App) deviceByUserAndFingerprint(userID, fingerprint string) int {
	for i, device := range a.store.state.Devices {
		if device.UserID == userID && device.Fingerprint == fingerprint {
			return i
		}
	}
	return -1
}

func (a *App) topupIndex(id string) int {
	for i, topup := range a.store.state.TokenTopups {
		if topup.ID == id {
			return i
		}
	}
	return -1
}

func (a *App) rechargeIndex(id string) int {
	for i, rr := range a.store.state.RechargeRequests {
		if rr.ID == id {
			return i
		}
	}
	return -1
}

func (a *App) upstreamIndex(id string) int {
	for i, up := range a.store.state.UpstreamAccounts {
		if up.ID == id {
			return i
		}
	}
	return -1
}

func (a *App) upstreamByID(id string) UpstreamAccount {
	for _, up := range a.store.state.UpstreamAccounts {
		if up.ID == id {
			return up
		}
	}
	return UpstreamAccount{}
}

func (a *App) apiKeyIndex(id string) int {
	for i, key := range a.store.state.APIKeys {
		if key.ID == id {
			return i
		}
	}
	return -1
}

func publicAdmin(admin Admin) map[string]any {
	return map[string]any{"id": admin.ID, "account": admin.Account, "mustChangePassword": admin.MustChangePassword, "createdAt": admin.CreatedAt}
}

func upstreamIsAvailable(up UpstreamAccount) bool {
	return up.Status == statusActive && upstreamAccountIsAvailable(up)
}

func upstreamAccountIsAvailable(up UpstreamAccount) bool {
	return strings.EqualFold(up.BalanceStatus, "available") && strings.EqualFold(up.RiskStatus, "available")
}

func upstreamAvailabilityStatus(up UpstreamAccount) string {
	if upstreamAccountIsAvailable(up) {
		return "available"
	}
	return "unavailable"
}

func upstreamRateLimitRemainingPercent(up UpstreamAccount) *float64 {
	if up.RateLimitUsedPercent == nil {
		return nil
	}
	remaining := 100 - *up.RateLimitUsedPercent
	if remaining < 0 {
		remaining = 0
	}
	if remaining > 100 {
		remaining = 100
	}
	return &remaining
}

func publicUpstream(up UpstreamAccount) map[string]any {
	return map[string]any{
		"id":                        up.ID,
		"name":                      up.Name,
		"group":                     up.Group,
		"credentialType":            up.CredentialType,
		"tokenType":                 up.TokenType,
		"chatgptAccountId":          up.ChatGPTAccountID,
		"expiresAt":                 up.ExpiresAt,
		"email":                     up.Email,
		"subscriptionTier":          up.SubscriptionTier,
		"entitlementStatus":         up.EntitlementStatus,
		"enabled":                   up.Status == statusActive,
		"availabilityStatus":        upstreamAvailabilityStatus(up),
		"usageTokens":               up.UsageTokens,
		"rateLimitUsedPercent":      up.RateLimitUsedPercent,
		"rateLimitRemainingPercent": upstreamRateLimitRemainingPercent(up),
		"rateLimitResetsAt":         up.RateLimitResetsAt,
		"creditBalance":             up.CreditBalance,
		"creditBalanceLabel":        up.CreditBalanceLabel,
		"lastCheckedAt":             up.LastCheckedAt,
		"createdAt":                 up.CreatedAt,
	}
}

func (a *App) publicAPIKeyLocked(key APIKey) map[string]any {
	up := a.upstreamByID(key.UpstreamAccountID)
	routeAvailable := false
	if up.ID != "" {
		routeAvailable = key.Status == statusActive && upstreamIsAvailable(up)
	}
	return map[string]any{
		"id":                  key.ID,
		"upstreamAccountId":   key.UpstreamAccountID,
		"upstreamAccountName": up.Name,
		"status":              key.Status,
		"routeAvailable":      routeAvailable,
		"lastUsedAt":          key.LastUsedAt,
		"createdAt":           key.CreatedAt,
	}
}

func (a *App) encrypt(plain string) (string, error) {
	block, err := aes.NewCipher(a.secretKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

func (a *App) decrypt(cipherText string) (string, error) {
	if cipherText == "" {
		return "", nil
	}
	raw, err := base64.StdEncoding.DecodeString(cipherText)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(a.secretKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("ciphertext_too_short")
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func (a *App) fingerprint(value string) string {
	mac := hmac.New(sha256.New, a.secretKey)
	_, _ = mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))[:16]
}

func makePasswordHash(password string) (string, string) {
	saltBytes := make([]byte, 16)
	_, _ = rand.Read(saltBytes)
	salt := base64.StdEncoding.EncodeToString(saltBytes)
	return salt, hashPassword(password, salt)
}

func hashPassword(password, salt string) string {
	sum := sha256.Sum256([]byte(salt + "\x00" + password))
	return hex.EncodeToString(sum[:])
}

func verifyPassword(password, salt, expected string) bool {
	return hmac.Equal([]byte(hashPassword(password, salt)), []byte(expected))
}

func hashString(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func deriveKey(value string) []byte {
	sum := sha256.Sum256([]byte(value))
	return sum[:]
}

func backendSecret(rawSecret, databaseURL string) (string, error) {
	secret := strings.TrimSpace(rawSecret)
	usingPersistentDatabase := strings.TrimSpace(databaseURL) != ""
	if usingPersistentDatabase {
		if isDefaultDeploymentSecret(secret) {
			return "", errors.New("CODEXPPP_SECRET must be set to a non-default value when CODEXPPP_DATABASE_URL is set")
		}
		return secret, nil
	}
	if secret == "" {
		return defaultDevSecret, nil
	}
	return secret, nil
}

func isDefaultDeploymentSecret(secret string) bool {
	switch strings.ToLower(strings.TrimSpace(secret)) {
	case "", defaultDevSecret, "replace-with-a-long-random-secret", "<set-a-long-random-secret>", "set-a-long-random-secret", "change-me", "changeme":
		return true
	default:
		return false
	}
}

type RedisRateLimiter struct {
	client *redis.Client
}

func redisRateLimiterFromEnv() (*RedisRateLimiter, error) {
	addr := strings.TrimSpace(os.Getenv("CODEXPPP_REDIS_ADDR"))
	if addr == "" {
		return nil, nil
	}
	db, err := redisDBFromEnv(os.Getenv("CODEXPPP_REDIS_DB"))
	if err != nil {
		return nil, err
	}
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: os.Getenv("CODEXPPP_REDIS_PASSWORD"),
		DB:       db,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis_unavailable: %w", err)
	}
	return &RedisRateLimiter{client: client}, nil
}

func (r *RedisRateLimiter) Allow(ctx context.Context, key string, limit int64, window time.Duration) (bool, error) {
	if r == nil || r.client == nil || limit <= 0 {
		return true, nil
	}
	count, err := r.client.Incr(ctx, key).Result()
	if err != nil {
		return false, err
	}
	if count == 1 {
		if err := r.client.Expire(ctx, key, window).Err(); err != nil {
			return false, err
		}
	}
	return count <= limit, nil
}

func gatewayRateLimitFromEnv(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultGatewayRateLimitPerMinute, nil
	}
	limit, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || limit < 0 {
		return 0, errors.New("CODEXPPP_GATEWAY_RATE_LIMIT_PER_MINUTE must be zero or a positive integer")
	}
	return limit, nil
}

func redisDBFromEnv(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	db, err := strconv.Atoi(raw)
	if err != nil || db < 0 {
		return 0, errors.New("CODEXPPP_REDIS_DB must be zero or a positive integer")
	}
	return db, nil
}

func randomToken(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func generateSub2APIKey() (string, error) {
	b := make([]byte, sub2APIKeyRandomBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return codexProviderKeyPrefix + hex.EncodeToString(b), nil
}

func isSub2APIStyleKey(token string) bool {
	if len(token) != len(codexProviderKeyPrefix)+sub2APIKeyRandomBytes*2 {
		return false
	}
	if !strings.HasPrefix(token, codexProviderKeyPrefix) {
		return false
	}
	for _, c := range token[len(codexProviderKeyPrefix):] {
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') {
			continue
		}
		return false
	}
	return true
}

func bearerToken(r *http.Request) string {
	value := r.Header.Get("Authorization")
	if strings.HasPrefix(value, "Bearer ") {
		return strings.TrimPrefix(value, "Bearer ")
	}
	return ""
}

func readStrictJSON(w http.ResponseWriter, r *http.Request, dst any, errorCode string, allowedFields ...string) bool {
	defer r.Body.Close()
	raw, ok := readStrictJSONMap(w, r.Body)
	if !ok {
		return false
	}
	if raw == nil {
		writeErr(w, http.StatusBadRequest, errorCode)
		return false
	}
	allowed := make(map[string]struct{}, len(allowedFields))
	for _, field := range allowedFields {
		allowed[field] = struct{}{}
	}
	for key := range raw {
		if _, ok := allowed[key]; !ok {
			writeErr(w, http.StatusBadRequest, errorCode)
			return false
		}
	}
	body, err := json.Marshal(raw)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_json")
		return false
	}
	if err := json.Unmarshal(body, dst); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_json")
		return false
	}
	return true
}

func readStrictJSONLocked(w http.ResponseWriter, r *http.Request, dst any, errorCode string, allowedFields ...string) bool {
	return readStrictJSON(w, r, dst, errorCode, allowedFields...)
}

func readEmptyJSON(w http.ResponseWriter, r *http.Request, errorCode string) bool {
	defer r.Body.Close()
	body, ok := readLimitedBody(w, r.Body, maxJSONBodyBytes)
	if !ok {
		return false
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return true
	}
	raw, ok := readStrictJSONMap(w, bytes.NewReader(body))
	if !ok {
		return false
	}
	if raw == nil || len(raw) != 0 {
		writeErr(w, http.StatusBadRequest, errorCode)
		return false
	}
	return true
}

func readStrictJSONMap(w http.ResponseWriter, r io.Reader) (map[string]json.RawMessage, bool) {
	body, ok := readLimitedBody(w, r, maxJSONBodyBytes)
	if !ok {
		return nil, false
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var raw map[string]json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_json")
		return nil, false
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		writeErr(w, http.StatusBadRequest, "invalid_json")
		return nil, false
	}
	return raw, true
}

func readLimitedBody(w http.ResponseWriter, r io.Reader, limit int64) ([]byte, bool) {
	limited := &io.LimitedReader{R: r, N: limit + 1}
	body, err := io.ReadAll(limited)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_json")
		return nil, false
	}
	if int64(len(body)) > limit {
		writeErr(w, http.StatusBadRequest, "invalid_json")
		return nil, false
	}
	return body, true
}

type adminUpstreamRequest struct {
	Name              string     `json:"name"`
	Group             string     `json:"group"`
	CredentialType    string     `json:"credentialType"`
	AccessToken       string     `json:"accessToken"`
	RefreshToken      string     `json:"refreshToken"`
	TokenType         string     `json:"tokenType"`
	ChatGPTAccountID  string     `json:"chatgptAccountId"`
	ExpiresAt         *time.Time `json:"expiresAt"`
	Email             string     `json:"email"`
	SubscriptionTier  string     `json:"subscriptionTier"`
	EntitlementStatus string     `json:"entitlementStatus"`
}

func readAdminUpstreamRequest(w http.ResponseWriter, r *http.Request) (adminUpstreamRequest, bool) {
	defer r.Body.Close()
	raw, ok := readStrictJSONMap(w, r.Body)
	if !ok {
		return adminUpstreamRequest{}, false
	}
	allowed := map[string]struct{}{
		"name":              {},
		"group":             {},
		"credentialType":    {},
		"accessToken":       {},
		"refreshToken":      {},
		"tokenType":         {},
		"chatgptAccountId":  {},
		"expiresAt":         {},
		"email":             {},
		"subscriptionTier":  {},
		"entitlementStatus": {},
	}
	for key := range raw {
		if _, ok := allowed[key]; !ok {
			writeErr(w, http.StatusBadRequest, "invalid_upstream_request")
			return adminUpstreamRequest{}, false
		}
	}
	body, err := json.Marshal(raw)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_json")
		return adminUpstreamRequest{}, false
	}
	var req adminUpstreamRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_json")
		return adminUpstreamRequest{}, false
	}
	req, err = normalizeAdminUpstreamRequest(req)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_upstream_request")
		return adminUpstreamRequest{}, false
	}
	return req, true
}

func readAdminUpstreamImportRequest(w http.ResponseWriter, r *http.Request) ([]adminUpstreamRequest, bool) {
	defer r.Body.Close()
	body, ok := readLimitedBody(w, r.Body, maxJSONBodyBytes)
	if !ok {
		return nil, false
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var payload any
	if err := dec.Decode(&payload); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_json")
		return nil, false
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		writeErr(w, http.StatusBadRequest, "invalid_json")
		return nil, false
	}
	reqs, err := parseAdminUpstreamImportPayload(payload)
	if err != nil || len(reqs) == 0 {
		writeErr(w, http.StatusBadRequest, "invalid_upstream_import_request")
		return nil, false
	}
	return reqs, true
}

func parseAdminUpstreamImportPayload(payload any) ([]adminUpstreamRequest, error) {
	if accounts, ok := payload.([]any); ok {
		return parseAdminUpstreamImportAccounts(accounts, "")
	}
	obj, ok := payload.(map[string]any)
	if !ok {
		return nil, errors.New("upstream_import_must_be_object")
	}
	defaultGroup := importStringField(obj, "group", "account_group", "accountGroup")
	if rawAccounts, ok := obj["accounts"]; ok {
		accounts, ok := rawAccounts.([]any)
		if !ok {
			return nil, errors.New("upstream_import_accounts_must_be_array")
		}
		return parseAdminUpstreamImportAccounts(accounts, defaultGroup)
	}
	req, err := parseAdminUpstreamImportAccount(obj, 1, defaultGroup)
	if err != nil {
		return nil, err
	}
	return []adminUpstreamRequest{req}, nil
}

func parseAdminUpstreamImportAccounts(accounts []any, defaultGroup string) ([]adminUpstreamRequest, error) {
	if len(accounts) == 0 {
		return nil, errors.New("upstream_import_accounts_empty")
	}
	reqs := make([]adminUpstreamRequest, 0, len(accounts))
	for i, item := range accounts {
		req, err := parseAdminUpstreamImportAccount(item, i+1, defaultGroup)
		if err != nil {
			return nil, err
		}
		reqs = append(reqs, req)
	}
	return reqs, nil
}

func parseAdminUpstreamImportAccount(item any, index int, defaultGroup string) (adminUpstreamRequest, error) {
	obj, ok := item.(map[string]any)
	if !ok {
		return adminUpstreamRequest{}, errors.New("upstream_import_account_must_be_object")
	}
	credentials, hasCredentials := importMapField(obj, "credentials")
	if !hasCredentials {
		credentials = obj
	}
	name := importStringField(obj, "name", "accountName", "account_name")
	if name == "" {
		name = importStringField(credentials, "name", "email")
	}
	if name == "" {
		name = fmt.Sprintf("Codex account %d", index)
	}
	group := importStringField(obj, "group", "account_group", "accountGroup")
	if group == "" {
		group = defaultGroup
	}
	credentialType := importNestedStringField(credentials, obj, hasCredentials, "credentialType", "credential_type")
	if credentialType == "" {
		credentialType = "oauth"
	}
	expiresAt, err := importNestedExpiresAt(credentials, obj, hasCredentials)
	if err != nil {
		return adminUpstreamRequest{}, err
	}
	req := adminUpstreamRequest{
		Name:              name,
		Group:             group,
		CredentialType:    credentialType,
		AccessToken:       importNestedStringField(credentials, obj, hasCredentials, "access_token", "accessToken"),
		RefreshToken:      importNestedStringField(credentials, obj, hasCredentials, "refresh_token", "refreshToken"),
		TokenType:         importNestedStringField(credentials, obj, hasCredentials, "token_type", "tokenType"),
		ChatGPTAccountID:  importNestedStringField(credentials, obj, hasCredentials, "chatgpt_account_id", "chatgptAccountId", "chatgpt_user_id", "chatgptUserId", "organization_id", "organizationId"),
		ExpiresAt:         expiresAt,
		Email:             importNestedStringField(credentials, obj, hasCredentials, "email"),
		SubscriptionTier:  importNestedStringField(credentials, obj, hasCredentials, "plan_type", "planType", "subscription_tier", "subscriptionTier"),
		EntitlementStatus: importNestedStringField(credentials, obj, hasCredentials, "entitlement_status", "entitlementStatus"),
	}
	if req.TokenType == "" {
		req.TokenType = "Bearer"
	}
	return normalizeAdminUpstreamRequest(req)
}

func normalizeAdminUpstreamRequest(req adminUpstreamRequest) (adminUpstreamRequest, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Group = strings.TrimSpace(req.Group)
	req.AccessToken = strings.TrimSpace(req.AccessToken)
	req.RefreshToken = strings.TrimSpace(req.RefreshToken)
	req.CredentialType = strings.TrimSpace(req.CredentialType)
	req.TokenType = strings.TrimSpace(req.TokenType)
	req.ChatGPTAccountID = strings.TrimSpace(req.ChatGPTAccountID)
	req.Email = strings.TrimSpace(req.Email)
	req.SubscriptionTier = strings.TrimSpace(req.SubscriptionTier)
	req.EntitlementStatus = strings.TrimSpace(req.EntitlementStatus)
	if req.ChatGPTAccountID == "" {
		req.ChatGPTAccountID = chatGPTAccountIDFromAccessToken(req.AccessToken)
	}
	if req.AccessToken == "" {
		return adminUpstreamRequest{}, errors.New("upstream_access_token_required")
	}
	if req.CredentialType != "" && req.CredentialType != "oauth" {
		return adminUpstreamRequest{}, errors.New("upstream_credential_type_unsupported")
	}
	return req, nil
}

func chatGPTAccountIDFromAccessToken(accessToken string) string {
	parts := strings.Split(accessToken, ".")
	if len(parts) < 2 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		payload, err = base64.URLEncoding.DecodeString(parts[1])
	}
	if err != nil {
		return ""
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ""
	}
	if auth, ok := claims["https://api.openai.com/auth"].(map[string]any); ok {
		if id := firstStringField(auth, "chatgpt_account_id", "chatgptAccountId", "user_id", "userId", "poid", "organization_id", "organizationId"); id != "" {
			return id
		}
	}
	return firstStringField(claims, "chatgpt_account_id", "chatgptAccountId", "user_id", "userId")
}

func importMapField(obj map[string]any, name string) (map[string]any, bool) {
	value, ok := obj[name]
	if !ok {
		return nil, false
	}
	nested, ok := value.(map[string]any)
	return nested, ok
}

func importNestedStringField(primary, secondary map[string]any, useSecondary bool, names ...string) string {
	if value := importStringField(primary, names...); value != "" {
		return value
	}
	if useSecondary {
		return importStringField(secondary, names...)
	}
	return ""
}

func importStringField(obj map[string]any, names ...string) string {
	for _, name := range names {
		value, ok := obj[name]
		if !ok {
			continue
		}
		switch v := value.(type) {
		case string:
			if text := strings.TrimSpace(v); text != "" {
				return text
			}
		case json.Number:
			if text := strings.TrimSpace(v.String()); text != "" {
				return text
			}
		}
	}
	return ""
}

func importNestedExpiresAt(primary, secondary map[string]any, useSecondary bool) (*time.Time, error) {
	if value, ok := importValueField(primary, "expires_at", "expiresAt", "expires"); ok {
		expiresAt, err := parseImportExpiresAt(value)
		if err != nil || expiresAt != nil {
			return expiresAt, err
		}
	}
	if useSecondary {
		if value, ok := importValueField(secondary, "expires_at", "expiresAt", "expires"); ok {
			return parseImportExpiresAt(value)
		}
	}
	return nil, nil
}

func importValueField(obj map[string]any, names ...string) (any, bool) {
	for _, name := range names {
		value, ok := obj[name]
		if ok && value != nil {
			return value, true
		}
	}
	return nil, false
}

func parseImportExpiresAt(value any) (*time.Time, error) {
	switch v := value.(type) {
	case nil:
		return nil, nil
	case json.Number:
		return parseImportExpiresAtString(v.String())
	case string:
		return parseImportExpiresAtString(v)
	case float64:
		return importUnixTimeFromFloat(v), nil
	case float32:
		return importUnixTimeFromFloat(float64(v)), nil
	case int:
		return importUnixTime(int64(v)), nil
	case int64:
		return importUnixTime(v), nil
	case int32:
		return importUnixTime(int64(v)), nil
	default:
		return nil, errors.New("upstream_import_expires_at_invalid")
	}
}

func parseImportExpiresAtString(value string) (*time.Time, error) {
	text := strings.TrimSpace(value)
	if text == "" {
		return nil, nil
	}
	if n, err := strconv.ParseInt(text, 10, 64); err == nil {
		return importUnixTime(n), nil
	}
	if f, err := strconv.ParseFloat(text, 64); err == nil {
		return importUnixTimeFromFloat(f), nil
	}
	if t, err := time.Parse(time.RFC3339, text); err == nil {
		tt := t.UTC()
		return &tt, nil
	}
	if t, err := time.Parse("2006-01-02 15:04:05", text); err == nil {
		tt := t.UTC()
		return &tt, nil
	}
	return nil, errors.New("upstream_import_expires_at_invalid")
}

func importUnixTime(value int64) *time.Time {
	if value > 1_000_000_000_000 || value < -1_000_000_000_000 {
		sec := value / 1000
		nsec := (value % 1000) * int64(time.Millisecond)
		t := time.Unix(sec, nsec).UTC()
		return &t
	}
	t := time.Unix(value, 0).UTC()
	return &t
}

func importUnixTimeFromFloat(value float64) *time.Time {
	if value > 1_000_000_000_000 || value < -1_000_000_000_000 {
		value = value / 1000
	}
	sec := int64(value)
	nsec := int64((value - float64(sec)) * 1_000_000_000)
	t := time.Unix(sec, nsec).UTC()
	return &t
}

func readAdminTopupRequest(w http.ResponseWriter, r *http.Request) (TokenTopup, bool) {
	defer r.Body.Close()
	raw, ok := readStrictJSONMap(w, r.Body)
	if !ok {
		return TokenTopup{}, false
	}
	allowed := map[string]struct{}{
		"name":        {},
		"priceCents":  {},
		"tokens":      {},
		"enabled":     {},
		"sort":        {},
		"description": {},
	}
	for key := range raw {
		if _, ok := allowed[key]; !ok {
			writeErr(w, http.StatusBadRequest, "invalid_topup_request")
			return TokenTopup{}, false
		}
	}
	body, err := json.Marshal(raw)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_json")
		return TokenTopup{}, false
	}
	var topup TokenTopup
	if err := json.Unmarshal(body, &topup); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_json")
		return TokenTopup{}, false
	}
	topup.Name = strings.TrimSpace(topup.Name)
	topup.Description = strings.TrimSpace(topup.Description)
	return topup, true
}

type adminAPIKeyCreateRequest struct {
	UpstreamAccountID string `json:"upstreamAccountId"`
}

func readAdminAPIKeyCreateRequest(w http.ResponseWriter, r *http.Request) (adminAPIKeyCreateRequest, bool) {
	defer r.Body.Close()
	raw, ok := readStrictJSONMap(w, r.Body)
	if !ok {
		return adminAPIKeyCreateRequest{}, false
	}
	if len(raw) != 1 {
		writeErr(w, http.StatusBadRequest, "invalid_api_key_request")
		return adminAPIKeyCreateRequest{}, false
	}
	value, ok := raw["upstreamAccountId"]
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid_api_key_request")
		return adminAPIKeyCreateRequest{}, false
	}
	var req adminAPIKeyCreateRequest
	if err := json.Unmarshal(value, &req.UpstreamAccountID); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_api_key_request")
		return adminAPIKeyCreateRequest{}, false
	}
	return req, true
}

type adminAPIKeyStatusRequest struct {
	Status string `json:"status"`
}

func readAdminAPIKeyStatusRequest(w http.ResponseWriter, r *http.Request) (adminAPIKeyStatusRequest, bool) {
	defer r.Body.Close()
	raw, ok := readStrictJSONMap(w, r.Body)
	if !ok {
		return adminAPIKeyStatusRequest{}, false
	}
	if len(raw) != 1 {
		writeErr(w, http.StatusBadRequest, "invalid_api_key_status_request")
		return adminAPIKeyStatusRequest{}, false
	}
	value, ok := raw["status"]
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid_api_key_status_request")
		return adminAPIKeyStatusRequest{}, false
	}
	var req adminAPIKeyStatusRequest
	if err := json.Unmarshal(value, &req.Status); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_api_key_status_request")
		return adminAPIKeyStatusRequest{}, false
	}
	return req, true
}

func readClientRechargeRequest(w http.ResponseWriter, r *http.Request) (string, bool) {
	defer r.Body.Close()
	raw, ok := readStrictJSONMap(w, r.Body)
	if !ok {
		return "", false
	}
	if len(raw) != 1 {
		writeErr(w, http.StatusBadRequest, "invalid_recharge_request")
		return "", false
	}
	value, ok := raw["topupId"]
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid_recharge_request")
		return "", false
	}
	var topupID string
	if err := json.Unmarshal(value, &topupID); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_recharge_request")
		return "", false
	}
	return topupID, true
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, apiError{Error: msg})
}

func (a *App) saveOrErrorLocked(w http.ResponseWriter) bool {
	if err := a.store.save(); err != nil {
		writeErr(w, http.StatusInternalServerError, "state_save_failed")
		return false
	}
	return true
}

func paginate[T any](items []T, r *http.Request) []T {
	page, size := paginationParams(r)
	start := (page - 1) * size
	if start >= len(items) {
		return []T{}
	}
	end := start + size
	if end > len(items) {
		end = len(items)
	}
	return items[start:end]
}

func paginationParams(r *http.Request) (int, int) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 10
	}
	if size > 100 {
		size = 100
	}
	return page, size
}

func intParam(r *http.Request, key string, fallback int) int {
	value, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil {
		return fallback
	}
	return value
}

func versionGreater(latest, current string) bool {
	cmp, ok := compareNumericVersions(latest, current)
	return ok && cmp > 0
}

func compareNumericVersions(a, b string) (int, bool) {
	left, ok := numericVersionParts(a)
	if !ok {
		return 0, false
	}
	right, ok := numericVersionParts(b)
	if !ok {
		return 0, false
	}
	maxLen := len(left)
	if len(right) > maxLen {
		maxLen = len(right)
	}
	for i := 0; i < maxLen; i++ {
		l, r := 0, 0
		if i < len(left) {
			l = left[i]
		}
		if i < len(right) {
			r = right[i]
		}
		if l > r {
			return 1, true
		}
		if l < r {
			return -1, true
		}
	}
	return 0, true
}

func numericVersionParts(raw string) ([]int, bool) {
	value := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(raw), "v"))
	if cut := strings.IndexAny(value, "-+"); cut >= 0 {
		value = value[:cut]
	}
	if value == "" {
		return nil, false
	}
	pieces := strings.Split(value, ".")
	parts := make([]int, 0, len(pieces))
	for _, piece := range pieces {
		if piece == "" {
			return nil, false
		}
		n, err := strconv.Atoi(piece)
		if err != nil || n < 0 {
			return nil, false
		}
		parts = append(parts, n)
	}
	return parts, true
}

func safeHTTPURL(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return ""
	}
	return parsed.String()
}

func utcDayBounds(t time.Time) (time.Time, time.Time) {
	utc := t.UTC()
	start := time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
	return start, start.Add(24 * time.Hour)
}

func sortPublicRecordsByCreatedAtDesc(items []map[string]any) {
	sort.SliceStable(items, func(i, j int) bool {
		left, _ := items[i]["createdAt"].(time.Time)
		right, _ := items[j]["createdAt"].(time.Time)
		if !left.Equal(right) {
			return left.After(right)
		}
		return publicRecordIDNumber(items[i]) > publicRecordIDNumber(items[j])
	})
}

func publicRecordIDNumber(item map[string]any) int64 {
	id, _ := item["id"].(string)
	idx := strings.LastIndex(id, "_")
	if idx < 0 || idx == len(id)-1 {
		return 0
	}
	n, err := strconv.ParseInt(id[idx+1:], 10, 64)
	if err != nil {
		return 0
	}
	return n
}

func listPayload[T any](items []T, r *http.Request) map[string]any {
	page, size := paginationParams(r)
	total := len(items)
	totalPages := 0
	if total > 0 {
		totalPages = (total + size - 1) / size
	}
	return map[string]any{"items": paginate(items, r), "page": page, "size": size, "total": total, "totalPages": totalPages}
}

func valueOr(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func corsOriginsFromEnv(value string) (map[string]struct{}, error) {
	origins := map[string]struct{}{}
	for _, origin := range defaultCORSOrigins() {
		normalized, err := normalizeCORSOrigin(origin)
		if err != nil {
			return nil, err
		}
		origins[normalized] = struct{}{}
	}
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		normalized, err := normalizeCORSOrigin(item)
		if err != nil {
			return nil, fmt.Errorf("invalid CODEXPPP_CLIENT_ORIGINS value %q: %w", item, err)
		}
		origins[normalized] = struct{}{}
	}
	return origins, nil
}

func defaultCORSOrigins() []string {
	return []string{
		"tauri://localhost",
		"http://tauri.localhost",
		"https://tauri.localhost",
		"http://localhost:1420",
		"http://127.0.0.1:1420",
		"http://localhost:5173",
		"http://127.0.0.1:5173",
	}
}

func normalizeCORSOrigin(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "null" {
		return value, nil
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("origin must include scheme and host")
	}
	if parsed.User != nil {
		return "", fmt.Errorf("origin must not include user info")
	}
	return strings.ToLower(parsed.Scheme) + "://" + strings.ToLower(parsed.Host), nil
}

func normalizeCORSRequestOrigin(value string) (string, error) {
	normalized, err := normalizeCORSOrigin(value)
	if err != nil || normalized == "null" {
		return normalized, err
	}
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return "", err
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", fmt.Errorf("request origin must not include a path")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("request origin must not include query or fragment")
	}
	return normalized, nil
}

func allowedCORSOrigin(r *http.Request, allowedOrigins map[string]struct{}, origin string) (string, bool) {
	normalized, err := normalizeCORSRequestOrigin(origin)
	if err != nil {
		return "", false
	}
	if _, ok := allowedOrigins[normalized]; ok {
		return normalized, true
	}
	if sameRequestOrigin(r, normalized) {
		return normalized, true
	}
	return "", false
}

func sameRequestOrigin(r *http.Request, origin string) bool {
	if origin == "null" {
		return false
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return false
	}
	return strings.EqualFold(parsed.Host, r.Host)
}

func listenAddrFromEnv(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultListenAddr
	}
	return value
}

func listenDisplayURL(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "http://localhost" + addr
	}
	if strings.HasPrefix(addr, "0.0.0.0:") {
		return "http://localhost:" + strings.TrimPrefix(addr, "0.0.0.0:")
	}
	if strings.HasPrefix(addr, "[::]:") {
		return "http://localhost:" + strings.TrimPrefix(addr, "[::]:")
	}
	return "http://" + addr
}
