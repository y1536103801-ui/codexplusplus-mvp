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
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"mime"
	"net"
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
	"unicode"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"
)

//go:embed web/admin/* web/site/*
var adminFiles embed.FS

//go:embed migrations/*.sql
var migrationFiles embed.FS

const defaultDevSecret = "codexppp-dev-secret-change-me"
const defaultListenAddr = "127.0.0.1:8787"
const defaultGatewayRateLimitPerMinute int64 = 120
const maxJSONBodyBytes int64 = 1 << 20
const maxGatewayEncodedBodyBytes int64 = 1 << 30
const maxGatewayDecodedBodyBytes int64 = 1 << 30
const maxCodexResponseBodyBytes int64 = 64 << 20
const maxStoredCodexResponseBodyBytes int64 = 2 << 20
const clientInteropHeader = "X-CodexPPP-Interop-Major"
const clientInteropMajor = "1"
const codexOAuthSessionTTL = 15 * time.Minute
const upstreamAccessTokenRefreshWindow = 10 * time.Minute
const gatewayClientLeaseTTL = 2 * time.Minute
const clientPresenceTTL = 2 * time.Minute
const legacyClientPresenceDeviceID = "legacy"
const gatewaySessionRouteTTL = 30 * 24 * time.Hour
const gatewayUpstreamLeaseTTL = 90 * time.Second
const gatewayUpstreamLeaseRenewInterval = 30 * time.Second
const defaultGatewayUpstreamConcurrency int64 = 2
const defaultGatewayUpstreamUserLimit int64 = 2
const defaultGatewayUploadConcurrency = 2
const defaultGatewayUploadUserLimit = 1
const gatewayDefaultOutputReservation int64 = 4096
const gatewayMaximumOutputReservation int64 = 32768
const gatewayMaximumRequestReservation int64 = 131072
const defaultCodexResponsesURL = "https://chatgpt.com/backend-api/codex/responses"
const maxUpstreamRemarkRunes = 500
const maxSiteOrdersPerHour = 5
const sessionTokenHashPrefix = "sha256:"
const maxAdminLoginAttempts = 10
const adminLoginAttemptWindow = 15 * time.Minute
const gatewayIdempotencyRetention = 30 * 24 * time.Hour
const gatewayReplayBodyRetention = 6 * time.Hour
const operationalCleanupInterval = time.Hour
const auditLogRetention = 180 * 24 * time.Hour

const (
	statusActive   = "active"
	statusDisabled = "disabled"

	upstreamAuthPending     = "pending_authorization"
	upstreamAuthAuthorizing = "authorizing"
	upstreamAuthAction      = "action_required"
	upstreamAuthAuthorized  = "authorized"
	upstreamAuthFailed      = "failed"

	codexDefaultModel = "gpt-5.5"

	sessionRoleAdmin       = "admin"
	sessionRoleClient      = "client"
	codexProviderKeyPrefix = "sk-"
	sub2APIKeyRandomBytes  = 32

	rechargePending   = "pending"
	rechargeApproved  = "approved"
	rechargeRejected  = "rejected"
	rechargeCancelled = "cancelled"

	accountOrderPending   = "pending"
	accountOrderContacted = "contacted"
	accountOrderFulfilled = "fulfilled"
	accountOrderRejected  = "rejected"

	gatewayReserved  = "reserved"
	gatewayCompleted = "completed"
	gatewayFailed    = "failed"
)

var errRouteUnavailable = errors.New("route_unavailable")

type App struct {
	store                  *Store
	secretKey              []byte
	corsOrigins            map[string]struct{}
	gatewayLimiter         GatewayRateLimiter
	gatewayRateLimit       int64
	gatewayRateWindow      time.Duration
	gatewayRuntime         GatewayRuntime
	gatewayInstanceID      string
	upstreamLimit          int64
	upstreamUserLimit      int64
	gatewayUploadLimit     int
	gatewayUploadUserLimit int
	gatewayUploadMu        sync.Mutex
	gatewayUploadTotal     int
	gatewayUploads         map[string]int
	gatewayInFlight        map[string]struct{}
	gatewayActive          map[string]map[string]int
	gatewayLeases          map[string]map[string]time.Time
	clientRuntimes         map[string]map[string]clientRuntimeLease
	clientPresence         map[string]map[string]time.Time
	siteOrderMu            sync.Mutex
	siteOrderAttempts      map[string][]time.Time
	adminLoginMu           sync.Mutex
	adminLoginAttempts     map[string][]time.Time
	upstreamCheckMu        sync.Mutex
	upstreamCheckStateMu   sync.Mutex
	upstreamChecking       map[string]struct{}
	oauthMu                sync.Mutex
	oauthSessions          map[string]codexOAuthSession
	oauthLogins            map[string]*codexDeviceCodeLogin
}

type clientRuntimeLease struct {
	ExpiresAt      time.Time
	DesktopVersion string
	CodexVersion   string
}

type codexOAuthSession struct {
	State           string
	AdminID         string
	Status          string
	Error           string
	Imported        int
	Updated         int
	KeyPreview      string
	AccountName     string
	AuthMethod      string
	VerificationURL string
	UserCode        string
	LoginID         string
	UpstreamID      string
	CreatedAt       time.Time
	ExpiresAt       time.Time
}

type upstreamImportResult struct {
	Imported int              `json:"imported"`
	Updated  int              `json:"updated"`
	Pending  int              `json:"pending"`
	Items    []map[string]any `json:"items"`
	APIKeys  []map[string]any `json:"apiKeys"`
}

type GatewayRateLimiter interface {
	Allow(ctx context.Context, key string, limit int64, window time.Duration) (bool, error)
}

type GatewayRuntime interface {
	SessionRoute(ctx context.Context, key string) (string, error)
	RememberSessionRoute(ctx context.Context, key, upstreamID string, ttl time.Duration) error
	AcquireUpstream(ctx context.Context, upstreamID, leaseID string, limit int64, ttl time.Duration) (bool, error)
	RenewUpstream(ctx context.Context, upstreamID, leaseID string, ttl time.Duration) (bool, error)
	ReleaseUpstream(ctx context.Context, upstreamID, leaseID string) error
	AcquireLock(ctx context.Context, key, owner string, ttl time.Duration) (bool, error)
	ReleaseLock(ctx context.Context, key, owner string) error
	Ping(ctx context.Context) error
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
var codexResponsesRun = runCodexResponsesRequestSource
var codexResponsesStreamRun = runCodexResponsesStreamingRequestSource
var codexDeviceCodeLoginStart = startCodexDeviceCodeLogin
var codexBrowserLoginStart = startCodexBrowserLogin

type codexResponsesResult struct {
	Status      int
	Header      http.Header
	Body        []byte
	Payload     any
	Usage       gatewayUsage
	ContentType string
}

type codexResponseCapture struct {
	Result codexResponsesResult
}

type codexResponseCaptureContextKey struct{}
type codexStreamTargetContextKey struct{}

type codexStreamTarget struct {
	Writer  http.ResponseWriter
	Started bool
}

type codexDeviceCodeLogin struct {
	AuthMethod      string
	VerificationURL string
	UserCode        string
	LoginID         string
	ExpectedState   string
	Callback        func(context.Context, string) error
	Wait            func(context.Context) (string, error)
	Cleanup         func()
}

type Store struct {
	mu          sync.Mutex
	path        string
	databaseURL string
	db          *sql.DB
	writerConn  *sql.Conn
	state       State
}

type State struct {
	NextID           int64             `json:"nextId"`
	Admins           []Admin           `json:"admins"`
	Users            []User            `json:"users"`
	Devices          []Device          `json:"devices"`
	TokenTopups      []TokenTopup      `json:"tokenTopups"`
	AccountOrders    []AccountOrder    `json:"accountOrders"`
	RechargeRequests []RechargeRequest `json:"rechargeRequests"`
	TokenLedgers     []TokenLedger     `json:"tokenLedgers"`
	UpstreamAccounts []UpstreamAccount `json:"upstreamAccounts"`
	APIKeys          []APIKey          `json:"apiKeys"`
	ClientAccessKeys []ClientAccessKey `json:"clientAccessKeys"`
	UsageRecords     []UsageRecord     `json:"usageRecords"`
	GatewayRequests  []GatewayRequest  `json:"gatewayRequests"`
	GatewaySessions  []GatewaySession  `json:"gatewaySessions,omitempty"`
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

type AccountOrder struct {
	ID          string     `json:"id"`
	BuyerCipher string     `json:"buyerCipher"`
	TopupID     string     `json:"topupId"`
	TopupName   string     `json:"topupName"`
	PriceCents  int64      `json:"priceCents"`
	Tokens      int64      `json:"tokens"`
	Status      string     `json:"status"`
	UserID      string     `json:"userId,omitempty"`
	AdminRemark string     `json:"adminRemark,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	FulfilledAt *time.Time `json:"fulfilledAt,omitempty"`
}

type accountOrderBuyer struct {
	Contact          string `json:"contact"`
	PreferredAccount string `json:"preferredAccount"`
	Remark           string `json:"remark"`
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
	ID                     string     `json:"id"`
	Name                   string     `json:"name"`
	Group                  string     `json:"group"`
	Remark                 string     `json:"remark,omitempty"`
	CredentialType         string     `json:"credentialType"`
	SourceType             string     `json:"sourceType,omitempty"`
	AuthorizationStatus    string     `json:"authorizationStatus,omitempty"`
	AccessTokenCipher      string     `json:"accessTokenCipher,omitempty"`
	RefreshTokenCipher     string     `json:"refreshTokenCipher,omitempty"`
	AuthJSONCipher         string     `json:"authJsonCipher,omitempty"`
	PasswordCipher         string     `json:"passwordCipher,omitempty"`
	LastAuthorizationError string     `json:"lastAuthorizationError,omitempty"`
	TokenType              string     `json:"tokenType,omitempty"`
	ChatGPTAccountID       string     `json:"chatgptAccountId,omitempty"`
	ExpiresAt              *time.Time `json:"expiresAt,omitempty"`
	Email                  string     `json:"email,omitempty"`
	SubscriptionTier       string     `json:"subscriptionTier,omitempty"`
	EntitlementStatus      string     `json:"entitlementStatus,omitempty"`
	Status                 string     `json:"status"`
	BalanceStatus          string     `json:"balanceStatus"`
	RiskStatus             string     `json:"riskStatus"`
	UsageTokens            int64      `json:"usageTokens,omitempty"`
	RateLimitUsedPercent   *float64   `json:"rateLimitUsedPercent,omitempty"`
	RateLimitResetsAt      *time.Time `json:"rateLimitResetsAt,omitempty"`
	CreditBalance          *float64   `json:"creditBalance,omitempty"`
	CreditBalanceLabel     string     `json:"creditBalanceLabel,omitempty"`
	LastCheckedAt          *time.Time `json:"lastCheckedAt,omitempty"`
	CredentialFingerprint  string     `json:"credentialFingerprint,omitempty"`
	CreatedAt              time.Time  `json:"createdAt"`
	UpdatedAt              time.Time  `json:"updatedAt"`
}

type APIKey struct {
	ID                string `json:"id"`
	KeyCipher         string `json:"keyCipher,omitempty"`
	KeyHash           string `json:"keyHash"`
	PublicPrefix      string `json:"publicPrefix"`
	UpstreamAccountID string `json:"upstreamAccountId"`
	// UserID is retained only to migrate credentials created before client
	// access keys were separated from account-pool route keys.
	UserID     string     `json:"userId,omitempty"`
	Status     string     `json:"status"`
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
}

type ClientAccessKey struct {
	ID           string     `json:"id"`
	KeyCipher    string     `json:"keyCipher,omitempty"`
	KeyHash      string     `json:"keyHash"`
	PublicPrefix string     `json:"publicPrefix"`
	UserID       string     `json:"userId"`
	DeviceID     string     `json:"deviceId,omitempty"`
	Status       string     `json:"status"`
	LastUsedAt   *time.Time `json:"lastUsedAt,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

type UsageRecord struct {
	ID                string    `json:"id"`
	UserID            string    `json:"userId"`
	UpstreamAccountID string    `json:"upstreamAccountId,omitempty"`
	APIKeyID          string    `json:"apiKeyId,omitempty"`
	ClientAccessKeyID string    `json:"clientAccessKeyId,omitempty"`
	SessionID         string    `json:"sessionId,omitempty"`
	Model             string    `json:"model"`
	InputTokens       int64     `json:"inputTokens"`
	CachedInputTokens int64     `json:"cachedInputTokens"`
	OutputTokens      int64     `json:"outputTokens"`
	TotalTokens       int64     `json:"totalTokens"`
	CreatedAt         time.Time `json:"createdAt"`
}

type usageAnalyticsFilter struct {
	AccountIDs []string
	From       time.Time
	To         time.Time
	FromDate   string
	ToDate     string
	GroupBy    string
	Metric     string
	Timezone   string
	MinTasks   *int64
	MaxTasks   *int64
	MinTokens  *int64
	MaxTokens  *int64
}

type usageAnalyticsAccumulator struct {
	AccountID         string
	BucketStart       time.Time
	InputTokens       int64
	CachedInputTokens int64
	OutputTokens      int64
	TotalTokens       int64
	RecordCount       int64
	FallbackRecords   int64
	Tasks             map[string]struct{}
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
	ResultBody     string    `json:"resultBody,omitempty"`
	ResultType     string    `json:"resultType,omitempty"`
	ResultHeaders  string    `json:"resultHeaders,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// GatewaySession persists a stable Codex task-to-upstream affinity. SessionKey
// is a SHA-256 digest rather than the raw desktop task identifier.
type GatewaySession struct {
	UserID            string    `json:"userId"`
	SessionKey        string    `json:"sessionKey"`
	UpstreamAccountID string    `json:"upstreamAccountId"`
	ExpiresAt         time.Time `json:"expiresAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
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
	upstreamLimit, err := gatewayUpstreamConcurrencyFromEnv(os.Getenv("CODEXPPP_GATEWAY_UPSTREAM_CONCURRENCY"))
	if err != nil {
		log.Fatal(err)
	}
	upstreamUserLimit, err := gatewayUpstreamUserLimitFromEnv(os.Getenv("CODEXPPP_GATEWAY_UPSTREAM_USER_LIMIT"))
	if err != nil {
		log.Fatal(err)
	}
	uploadLimit, err := gatewayPositiveIntFromEnv(os.Getenv("CODEXPPP_GATEWAY_UPLOAD_CONCURRENCY"), defaultGatewayUploadConcurrency, "CODEXPPP_GATEWAY_UPLOAD_CONCURRENCY")
	if err != nil {
		log.Fatal(err)
	}
	uploadUserLimit, err := gatewayPositiveIntFromEnv(os.Getenv("CODEXPPP_GATEWAY_UPLOAD_USER_LIMIT"), defaultGatewayUploadUserLimit, "CODEXPPP_GATEWAY_UPLOAD_USER_LIMIT")
	if err != nil {
		log.Fatal(err)
	}
	corsOrigins, err := corsOriginsFromEnv(os.Getenv("CODEXPPP_CLIENT_ORIGINS"))
	if err != nil {
		log.Fatal(err)
	}
	app := &App{
		store:                  store,
		secretKey:              deriveKey(secret),
		corsOrigins:            corsOrigins,
		gatewayLimiter:         limiter,
		gatewayRateLimit:       rateLimit,
		gatewayRateWindow:      time.Minute,
		gatewayRuntime:         limiter,
		gatewayInstanceID:      randomToken(12),
		upstreamLimit:          upstreamLimit,
		upstreamUserLimit:      upstreamUserLimit,
		gatewayUploadLimit:     uploadLimit,
		gatewayUploadUserLimit: uploadUserLimit,
		gatewayUploads:         map[string]int{},
		gatewayInFlight:        map[string]struct{}{},
		gatewayActive:          map[string]map[string]int{},
		gatewayLeases:          map[string]map[string]time.Time{},
		clientRuntimes:         map[string]map[string]clientRuntimeLease{},
		clientPresence:         map[string]map[string]time.Time{},
		siteOrderAttempts:      map[string][]time.Time{},
		adminLoginAttempts:     map[string][]time.Time{},
		upstreamChecking:       map[string]struct{}{},
		oauthSessions:          map[string]codexOAuthSession{},
		oauthLogins:            map[string]*codexDeviceCodeLogin{},
	}
	cleanupGatewayTempFiles()
	if err := app.initializeRuntimeState(); err != nil {
		log.Fatal(err)
	}
	app.startDailyUpstreamCheckScheduler(context.Background())
	app.startOperationalCleanupScheduler(context.Background())

	addr := listenAddrFromEnv(os.Getenv("CODEXPPP_ADDR"))
	log.Printf("Codex+++ backend listening on %s", listenDisplayURL(addr))
	log.Fatal(http.ListenAndServe(addr, app.routes()))
}

func (a *App) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", a.siteStatic)
	mux.HandleFunc("/admin", a.adminStatic)
	mux.HandleFunc("/admin/", a.adminStatic)
	mux.HandleFunc("/api/health", a.healthAPI)
	mux.HandleFunc("/api/ready", a.readyAPI)
	mux.HandleFunc("/api/site/", a.siteAPI)
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

func (a *App) readyAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusNotFound, "not_found")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if err := a.store.ping(ctx); err != nil {
		writeErr(w, http.StatusServiceUnavailable, "not_ready")
		return
	}
	if a.gatewayRuntime != nil {
		if err := a.gatewayRuntime.Ping(ctx); err != nil {
			writeErr(w, http.StatusServiceUnavailable, "not_ready")
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
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
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key, X-Request-ID, X-Request-Id, X-CodexPPP-Interop-Major")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
	w.Header().Set("Access-Control-Max-Age", "600")
}

func (a *App) adminStatic(w http.ResponseWriter, r *http.Request) {
	path := "web/admin/index.html"
	if r.URL.Path != "/admin" && r.URL.Path != "/admin/" {
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

func (a *App) siteStatic(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "/index.html" {
		http.NotFound(w, r)
		return
	}
	data, err := adminFiles.ReadFile("web/site/index.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; connect-src 'self'; img-src 'self' data:; base-uri 'none'; form-action 'self'; frame-ancestors 'none'")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
	w.Header().Set("X-Content-Type-Options", "nosniff")
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

func (s *Store) ping(ctx context.Context) error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.PingContext(ctx)
}

func (s *Store) openPostgres() error {
	db, err := sql.Open("pgx", s.databaseURL)
	if err != nil {
		return err
	}
	s.db = db
	succeeded := false
	defer func() {
		if !succeeded {
			_ = s.Close()
		}
	}()
	// Startup loads every durable table after applying migrations. Keep enough
	// headroom for managed PostgreSQL or an SSH-tunnelled recovery connection;
	// the readiness endpoint still uses its own short operational deadline.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return err
	}
	if err := s.applyMigrations(ctx); err != nil {
		return err
	}
	if err := s.acquirePostgresWriterLock(ctx); err != nil {
		return err
	}
	if err := s.loadPostgres(ctx); err != nil {
		return err
	}
	if s.state.NextID == 0 {
		s.state.NextID = 1
	}
	s.normalizeRechargeTransitions()
	changed := false
	if len(s.state.TokenTopups) == 0 {
		s.seedDefaultsLocked()
		changed = true
	}
	if changed {
		if err := s.savePostgres(ctx); err != nil {
			return err
		}
	}
	succeeded = true
	return nil
}

func (s *Store) acquirePostgresWriterLock(ctx context.Context) error {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	var acquired bool
	if err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1, $2)`, int32(1129268293), int32(1481658448)).Scan(&acquired); err != nil {
		_ = conn.Close()
		return err
	}
	if !acquired {
		_ = conn.Close()
		return errors.New("another Codex+++ backend writer is already connected to this database")
	}
	s.writerConn = conn
	return nil
}

func (s *Store) Close() error {
	if s == nil {
		return nil
	}
	if s.writerConn != nil {
		_ = s.writerConn.Close()
		s.writerConn = nil
	}
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *Store) applyMigrations(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, checksum TEXT NOT NULL, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		return err
	}
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
		checksum := hashString(string(sqlBytes))
		var storedChecksum string
		err = s.db.QueryRowContext(ctx, `SELECT checksum FROM schema_migrations WHERE version=$1`, path).Scan(&storedChecksum)
		if err == nil {
			if storedChecksum != checksum {
				return fmt.Errorf("migration checksum mismatch: %s", path)
			}
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, string(sqlBytes)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", path, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version,checksum) VALUES ($1,$2)`, path, checksum); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %s: %w", path, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", path, err)
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
	if err := scanRows(ctx, s.db, `SELECT id, buyer_cipher, topup_id, topup_name, price_cents, tokens, status, user_id, admin_remark, created_at, updated_at, fulfilled_at FROM account_orders ORDER BY created_at`, func(rows *sql.Rows) error {
		var item AccountOrder
		var fulfilled sql.NullTime
		if err := rows.Scan(&item.ID, &item.BuyerCipher, &item.TopupID, &item.TopupName, &item.PriceCents, &item.Tokens, &item.Status, &item.UserID, &item.AdminRemark, &item.CreatedAt, &item.UpdatedAt, &fulfilled); err != nil {
			return err
		}
		item.FulfilledAt = nullableTimePtr(fulfilled)
		state.AccountOrders = append(state.AccountOrders, item)
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
	if err := scanRows(ctx, s.db, `SELECT id, name, account_group, remark, credential_type, source_type, authorization_status, access_token_cipher, refresh_token_cipher, auth_json_cipher, password_cipher, last_authorization_error, token_type, chatgpt_account_id, expires_at, email, subscription_tier, entitlement_status, status, balance_status, risk_status, usage_tokens, rate_limit_used_percent, rate_limit_resets_at, credit_balance, credit_balance_label, last_checked_at, credential_fingerprint, created_at, updated_at FROM upstream_accounts ORDER BY created_at`, func(rows *sql.Rows) error {
		var item UpstreamAccount
		var expires, rateLimitResetsAt, lastChecked sql.NullTime
		var rateLimitUsedPercent, creditBalance sql.NullFloat64
		if err := rows.Scan(&item.ID, &item.Name, &item.Group, &item.Remark, &item.CredentialType, &item.SourceType, &item.AuthorizationStatus, &item.AccessTokenCipher, &item.RefreshTokenCipher, &item.AuthJSONCipher, &item.PasswordCipher, &item.LastAuthorizationError, &item.TokenType, &item.ChatGPTAccountID, &expires, &item.Email, &item.SubscriptionTier, &item.EntitlementStatus, &item.Status, &item.BalanceStatus, &item.RiskStatus, &item.UsageTokens, &rateLimitUsedPercent, &rateLimitResetsAt, &creditBalance, &item.CreditBalanceLabel, &lastChecked, &item.CredentialFingerprint, &item.CreatedAt, &item.UpdatedAt); err != nil {
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
	if err := scanRows(ctx, s.db, `SELECT id, key_cipher, key_hash, public_prefix, upstream_account_id, user_id, status, last_used_at, created_at, updated_at FROM api_keys ORDER BY created_at`, func(rows *sql.Rows) error {
		var item APIKey
		var lastUsed sql.NullTime
		if err := rows.Scan(&item.ID, &item.KeyCipher, &item.KeyHash, &item.PublicPrefix, &item.UpstreamAccountID, &item.UserID, &item.Status, &lastUsed, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return err
		}
		item.LastUsedAt = nullableTimePtr(lastUsed)
		state.APIKeys = append(state.APIKeys, item)
		return nil
	}); err != nil {
		return err
	}
	if err := scanRows(ctx, s.db, `SELECT id, key_cipher, key_hash, public_prefix, user_id, device_id, status, last_used_at, created_at, updated_at FROM client_access_keys ORDER BY created_at`, func(rows *sql.Rows) error {
		var item ClientAccessKey
		var lastUsed sql.NullTime
		if err := rows.Scan(&item.ID, &item.KeyCipher, &item.KeyHash, &item.PublicPrefix, &item.UserID, &item.DeviceID, &item.Status, &lastUsed, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return err
		}
		item.LastUsedAt = nullableTimePtr(lastUsed)
		state.ClientAccessKeys = append(state.ClientAccessKeys, item)
		return nil
	}); err != nil {
		return err
	}
	if err := scanRows(ctx, s.db, `SELECT id, user_id, upstream_account_id, api_key_id, client_access_key_id, session_id, model, input_tokens, cached_input_tokens, output_tokens, total_tokens, created_at FROM usage_records ORDER BY created_at`, func(rows *sql.Rows) error {
		var item UsageRecord
		if err := rows.Scan(&item.ID, &item.UserID, &item.UpstreamAccountID, &item.APIKeyID, &item.ClientAccessKeyID, &item.SessionID, &item.Model, &item.InputTokens, &item.CachedInputTokens, &item.OutputTokens, &item.TotalTokens, &item.CreatedAt); err != nil {
			return err
		}
		state.UsageRecords = append(state.UsageRecords, item)
		return nil
	}); err != nil {
		return err
	}
	// Raw Responses payloads are intentionally excluded from the resident state.
	// PostgreSQL keeps the short replay window and serves a body only when a
	// matching completed request is actually retried.
	if err := scanRows(ctx, s.db, `SELECT id, user_id, request_id, status, reserved_tokens, charged_tokens, usage_record_id, upstream_status, error, result_text, result_type, result_headers, created_at, updated_at FROM idempotency_records ORDER BY created_at`, func(rows *sql.Rows) error {
		var item GatewayRequest
		if err := rows.Scan(&item.ID, &item.UserID, &item.RequestID, &item.Status, &item.ReservedTokens, &item.ChargedTokens, &item.UsageRecordID, &item.UpstreamStatus, &item.Error, &item.ResultText, &item.ResultType, &item.ResultHeaders, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return err
		}
		state.GatewayRequests = append(state.GatewayRequests, item)
		return nil
	}); err != nil {
		return err
	}
	if err := scanRows(ctx, s.db, `SELECT user_id, session_key, upstream_account_id, expires_at, updated_at FROM gateway_session_routes ORDER BY user_id, session_key`, func(rows *sql.Rows) error {
		var item GatewaySession
		if err := rows.Scan(&item.UserID, &item.SessionKey, &item.UpstreamAccountID, &item.ExpiresAt, &item.UpdatedAt); err != nil {
			return err
		}
		state.GatewaySessions = append(state.GatewaySessions, item)
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
	// Users and idempotency rows are updated in place. Deleting them as part of
	// the legacy snapshot writer would either violate the user foreign key or
	// force every stored Responses body through a full table rewrite on each
	// unrelated administrator mutation.
	if _, err := tx.ExecContext(ctx, `CREATE TEMP TABLE snapshot_user_ids (id TEXT PRIMARY KEY) ON COMMIT DROP`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `CREATE TEMP TABLE snapshot_gateway_requests (user_id TEXT NOT NULL, request_id TEXT NOT NULL, PRIMARY KEY (user_id, request_id)) ON COMMIT DROP`); err != nil {
		return err
	}
	for _, item := range s.state.Users {
		if _, err := tx.ExecContext(ctx, `INSERT INTO snapshot_user_ids (id) VALUES ($1)`, item.ID); err != nil {
			return err
		}
	}
	for _, item := range s.state.GatewayRequests {
		if _, err := tx.ExecContext(ctx, `INSERT INTO snapshot_gateway_requests (user_id, request_id) VALUES ($1,$2)`, item.UserID, item.RequestID); err != nil {
			return err
		}
	}
	for _, table := range []string{"sessions", "audit_logs", "gateway_session_routes", "usage_records", "client_access_keys", "api_keys", "upstream_accounts", "token_ledgers", "recharge_requests", "account_orders", "devices", "admins", "token_topups", "app_meta"} {
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
		if _, err := tx.ExecContext(ctx, `INSERT INTO users (id, account, password_salt, password_hash, status, token_balance, last_login_at, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT (id) DO UPDATE SET account=EXCLUDED.account,password_salt=EXCLUDED.password_salt,password_hash=EXCLUDED.password_hash,status=EXCLUDED.status,token_balance=EXCLUDED.token_balance,last_login_at=EXCLUDED.last_login_at,created_at=EXCLUDED.created_at,updated_at=EXCLUDED.updated_at`, item.ID, item.Account, item.PasswordSalt, item.PasswordHash, item.Status, item.TokenBalance, item.LastLoginAt, item.CreatedAt, item.UpdatedAt); err != nil {
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
	for _, item := range s.state.AccountOrders {
		if _, err := tx.ExecContext(ctx, `INSERT INTO account_orders (id, buyer_cipher, topup_id, topup_name, price_cents, tokens, status, user_id, admin_remark, created_at, updated_at, fulfilled_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, item.ID, item.BuyerCipher, item.TopupID, item.TopupName, item.PriceCents, item.Tokens, item.Status, item.UserID, item.AdminRemark, item.CreatedAt, item.UpdatedAt, item.FulfilledAt); err != nil {
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
		if _, err := tx.ExecContext(ctx, `INSERT INTO upstream_accounts (id, name, account_group, remark, credential_type, source_type, authorization_status, access_token_cipher, refresh_token_cipher, auth_json_cipher, password_cipher, last_authorization_error, token_type, chatgpt_account_id, expires_at, email, subscription_tier, entitlement_status, status, balance_status, risk_status, usage_tokens, rate_limit_used_percent, rate_limit_resets_at, credit_balance, credit_balance_label, last_checked_at, credential_fingerprint, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30)`, item.ID, item.Name, item.Group, item.Remark, item.CredentialType, item.SourceType, item.AuthorizationStatus, item.AccessTokenCipher, item.RefreshTokenCipher, item.AuthJSONCipher, item.PasswordCipher, item.LastAuthorizationError, item.TokenType, item.ChatGPTAccountID, item.ExpiresAt, item.Email, item.SubscriptionTier, item.EntitlementStatus, item.Status, item.BalanceStatus, item.RiskStatus, item.UsageTokens, item.RateLimitUsedPercent, item.RateLimitResetsAt, item.CreditBalance, item.CreditBalanceLabel, item.LastCheckedAt, item.CredentialFingerprint, item.CreatedAt, item.UpdatedAt); err != nil {
			return err
		}
	}
	for _, item := range s.state.APIKeys {
		if _, err := tx.ExecContext(ctx, `INSERT INTO api_keys (id, key_cipher, key_hash, public_prefix, upstream_account_id, user_id, status, last_used_at, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, item.ID, item.KeyCipher, item.KeyHash, item.PublicPrefix, item.UpstreamAccountID, item.UserID, item.Status, item.LastUsedAt, item.CreatedAt, item.UpdatedAt); err != nil {
			return err
		}
	}
	for _, item := range s.state.ClientAccessKeys {
		if _, err := tx.ExecContext(ctx, `INSERT INTO client_access_keys (id, key_cipher, key_hash, public_prefix, user_id, device_id, status, last_used_at, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, item.ID, item.KeyCipher, item.KeyHash, item.PublicPrefix, item.UserID, item.DeviceID, item.Status, item.LastUsedAt, item.CreatedAt, item.UpdatedAt); err != nil {
			return err
		}
	}
	for _, item := range s.state.UsageRecords {
		if _, err := tx.ExecContext(ctx, `INSERT INTO usage_records (id, user_id, upstream_account_id, api_key_id, client_access_key_id, session_id, model, input_tokens, cached_input_tokens, output_tokens, total_tokens, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, item.ID, item.UserID, item.UpstreamAccountID, item.APIKeyID, item.ClientAccessKeyID, item.SessionID, item.Model, item.InputTokens, item.CachedInputTokens, item.OutputTokens, item.TotalTokens, item.CreatedAt); err != nil {
			return err
		}
	}
	for _, item := range s.state.GatewayRequests {
		if _, err := tx.ExecContext(ctx, `INSERT INTO idempotency_records (id, user_id, request_id, status, reserved_tokens, charged_tokens, usage_record_id, upstream_status, error, result_text, result_body, result_type, result_headers, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15) ON CONFLICT (user_id, request_id) DO UPDATE SET status=EXCLUDED.status,reserved_tokens=EXCLUDED.reserved_tokens,charged_tokens=EXCLUDED.charged_tokens,usage_record_id=EXCLUDED.usage_record_id,upstream_status=EXCLUDED.upstream_status,error=EXCLUDED.error,result_text=EXCLUDED.result_text,result_body=CASE WHEN EXCLUDED.status=$16 AND EXCLUDED.result_body='' THEN idempotency_records.result_body ELSE EXCLUDED.result_body END,result_type=EXCLUDED.result_type,result_headers=EXCLUDED.result_headers,created_at=EXCLUDED.created_at,updated_at=EXCLUDED.updated_at`, item.ID, item.UserID, item.RequestID, item.Status, item.ReservedTokens, item.ChargedTokens, item.UsageRecordID, item.UpstreamStatus, item.Error, item.ResultText, item.ResultBody, item.ResultType, item.ResultHeaders, item.CreatedAt, item.UpdatedAt, gatewayCompleted); err != nil {
			return err
		}
	}
	for _, item := range s.state.GatewaySessions {
		if _, err := tx.ExecContext(ctx, `INSERT INTO gateway_session_routes (user_id, session_key, upstream_account_id, expires_at, updated_at) VALUES ($1,$2,$3,$4,$5)`, item.UserID, item.SessionKey, item.UpstreamAccountID, item.ExpiresAt, item.UpdatedAt); err != nil {
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
	if _, err := tx.ExecContext(ctx, `DELETE FROM idempotency_records AS item WHERE NOT EXISTS (SELECT 1 FROM snapshot_gateway_requests AS keep WHERE keep.user_id=item.user_id AND keep.request_id=item.request_id)`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM users AS item WHERE NOT EXISTS (SELECT 1 FROM snapshot_user_ids AS keep WHERE keep.id=item.id)`); err != nil {
		return err
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

func (a *App) initializeRuntimeState() error {
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	now := time.Now().UTC()
	if a.gatewayInFlight == nil {
		a.gatewayInFlight = map[string]struct{}{}
	}
	changed := a.normalizeUpstreamAuthorizationStateLocked()
	keysChanged, err := a.normalizeAccountPoolAPIKeysLocked(now)
	if err != nil {
		return err
	}
	if keysChanged {
		changed = true
	}
	clientKeysChanged, err := a.normalizeClientAccessKeysLocked(now)
	if err != nil {
		return err
	}
	if clientKeysChanged {
		changed = true
	}
	if a.removeLegacyCodexProviderSessionsLocked() {
		changed = true
	}
	if a.normalizeSessionTokensLocked() {
		changed = true
	}
	if a.pruneOperationalHistoryLocked(now) {
		changed = true
	}
	if a.normalizeGatewaySessionsLocked(now) {
		changed = true
	}
	if a.releaseInterruptedGatewayRequestsLocked(now) {
		changed = true
	}
	if changed {
		return a.store.save()
	}
	return nil
}

func (a *App) pruneOperationalHistoryLocked(now time.Time) bool {
	changed := false
	activeSessions := a.store.state.Sessions[:0]
	for _, session := range a.store.state.Sessions {
		if session.ExpiresAt.After(now) {
			activeSessions = append(activeSessions, session)
		} else {
			changed = true
		}
	}
	a.store.state.Sessions = activeSessions

	idempotencyCutoff := now.Add(-gatewayIdempotencyRetention)
	requests := a.store.state.GatewayRequests[:0]
	for _, request := range a.store.state.GatewayRequests {
		if request.Status == gatewayReserved || request.UpdatedAt.After(idempotencyCutoff) {
			requests = append(requests, request)
		} else {
			changed = true
		}
	}
	a.store.state.GatewayRequests = requests

	auditCutoff := now.Add(-auditLogRetention)
	audits := a.store.state.AuditLogs[:0]
	for _, audit := range a.store.state.AuditLogs {
		if audit.CreatedAt.After(auditCutoff) {
			audits = append(audits, audit)
		} else {
			changed = true
		}
	}
	a.store.state.AuditLogs = audits
	return changed
}

func (a *App) normalizeSessionTokensLocked() bool {
	changed := false
	for i := range a.store.state.Sessions {
		token := strings.TrimSpace(a.store.state.Sessions[i].Token)
		if token == "" || strings.HasPrefix(token, sessionTokenHashPrefix) {
			continue
		}
		a.store.state.Sessions[i].Token = sessionTokenDigest(token)
		changed = true
	}
	return changed
}

func (a *App) normalizeGatewaySessionsLocked(now time.Time) bool {
	if len(a.store.state.GatewaySessions) == 0 {
		return false
	}
	latest := make(map[string]GatewaySession, len(a.store.state.GatewaySessions))
	for _, session := range a.store.state.GatewaySessions {
		if session.UserID == "" || session.SessionKey == "" || session.UpstreamAccountID == "" || !session.ExpiresAt.After(now) {
			continue
		}
		if a.userByID(session.UserID).ID == "" || a.upstreamByID(session.UpstreamAccountID).ID == "" {
			continue
		}
		key := session.UserID + "\x00" + session.SessionKey
		if current, ok := latest[key]; !ok || session.UpdatedAt.After(current.UpdatedAt) {
			latest[key] = session
		}
	}
	normalized := make([]GatewaySession, 0, len(latest))
	for _, session := range latest {
		normalized = append(normalized, session)
	}
	sort.Slice(normalized, func(i, j int) bool {
		if normalized[i].UserID == normalized[j].UserID {
			return normalized[i].SessionKey < normalized[j].SessionKey
		}
		return normalized[i].UserID < normalized[j].UserID
	})
	changed := len(normalized) != len(a.store.state.GatewaySessions)
	if !changed {
		for i := range normalized {
			if normalized[i] != a.store.state.GatewaySessions[i] {
				changed = true
				break
			}
		}
	}
	if changed {
		a.store.state.GatewaySessions = normalized
	}
	return changed
}

func (a *App) releaseInterruptedGatewayRequestsLocked(now time.Time) bool {
	changed := false
	for i := range a.store.state.GatewayRequests {
		req := &a.store.state.GatewayRequests[i]
		if req.Status != gatewayReserved {
			continue
		}
		released := req.ReservedTokens
		req.Status = gatewayFailed
		req.ReservedTokens = 0
		req.Error = "backend_restarted"
		req.UpdatedAt = now
		a.auditLocked("system", "system", "gateway.request.recovered", req.RequestID, fmt.Sprintf("user_id=%s released_tokens=%d reason=backend_restarted", req.UserID, released))
		changed = true
	}
	return changed
}

func (a *App) normalizeClientAccessKeysLocked(now time.Time) (bool, error) {
	existing := make(map[string]struct{}, len(a.store.state.ClientAccessKeys))
	for _, key := range a.store.state.ClientAccessKeys {
		if strings.TrimSpace(key.KeyHash) != "" {
			existing[key.KeyHash] = struct{}{}
		}
	}
	changed := false
	for i := range a.store.state.APIKeys {
		routeKey := &a.store.state.APIKeys[i]
		userID := strings.TrimSpace(routeKey.UserID)
		if userID == "" {
			continue
		}
		user := a.userByID(userID)
		if _, ok := existing[routeKey.KeyHash]; user.ID != "" && !ok && strings.TrimSpace(routeKey.KeyHash) != "" && strings.TrimSpace(routeKey.KeyCipher) != "" {
			createdAt := routeKey.CreatedAt
			if createdAt.IsZero() {
				createdAt = now
			}
			updatedAt := routeKey.UpdatedAt
			if updatedAt.IsZero() {
				updatedAt = now
			}
			a.store.state.ClientAccessKeys = append(a.store.state.ClientAccessKeys, ClientAccessKey{
				ID:           a.store.nextID("cak"),
				KeyCipher:    routeKey.KeyCipher,
				KeyHash:      routeKey.KeyHash,
				PublicPrefix: routeKey.PublicPrefix,
				UserID:       userID,
				Status:       routeKey.Status,
				LastUsedAt:   routeKey.LastUsedAt,
				CreatedAt:    createdAt,
				UpdatedAt:    updatedAt,
			})
			existing[routeKey.KeyHash] = struct{}{}
		}
		// The old credential remains valid as a user-scoped gateway key, while
		// the account-pool route receives a fresh internal-only identifier.
		raw, err := generateSub2APIKey()
		if err != nil {
			return false, err
		}
		cipherText, err := a.encrypt(raw)
		if err != nil {
			return false, err
		}
		routeKey.KeyCipher = cipherText
		routeKey.KeyHash = hashString(raw)
		routeKey.PublicPrefix = raw[:10]
		routeKey.UserID = ""
		routeKey.UpdatedAt = now
		changed = true
	}
	return changed, nil
}

func (a *App) normalizeUpstreamAuthorizationStateLocked() bool {
	changed := false
	for i := range a.store.state.UpstreamAccounts {
		up := &a.store.state.UpstreamAccounts[i]
		if strings.TrimSpace(up.AuthorizationStatus) == "" {
			if strings.TrimSpace(up.AccessTokenCipher) == "" {
				up.AuthorizationStatus = upstreamAuthPending
				up.Status = statusDisabled
				up.BalanceStatus = "unavailable"
				up.RiskStatus = "unavailable"
			} else {
				up.AuthorizationStatus = upstreamAuthAuthorized
			}
			changed = true
		}
		if strings.TrimSpace(up.SourceType) == "" {
			up.SourceType = "legacy"
			changed = true
		}
	}
	return changed
}

func (a *App) normalizeAccountPoolAPIKeysLocked(now time.Time) (bool, error) {
	if len(a.store.state.UpstreamAccounts) == 0 {
		return false, nil
	}
	referenced := make(map[string]struct{}, len(a.store.state.APIKeys))
	changed := false
	for i := range a.store.state.APIKeys {
		key := &a.store.state.APIKeys[i]
		if key.UpstreamAccountID != "" {
			referenced[key.UpstreamAccountID] = struct{}{}
		}
		if strings.TrimSpace(key.KeyCipher) == "" {
			raw, err := generateSub2APIKey()
			if err != nil {
				return false, err
			}
			cipherText, err := a.encrypt(raw)
			if err != nil {
				return false, err
			}
			key.KeyCipher = cipherText
			key.KeyHash = hashString(raw)
			key.PublicPrefix = raw[:10]
			key.UpdatedAt = now
			changed = true
		}
	}
	for _, up := range a.store.state.UpstreamAccounts {
		if up.ID == "" || up.AuthorizationStatus != upstreamAuthAuthorized || strings.TrimSpace(up.AccessTokenCipher) == "" {
			continue
		}
		if _, ok := referenced[up.ID]; ok {
			continue
		}
		key, err := a.newAPIKeyForUpstreamLocked(up.ID, now)
		if err != nil {
			return false, err
		}
		a.store.state.APIKeys = append(a.store.state.APIKeys, key)
		referenced[up.ID] = struct{}{}
		changed = true
	}
	return changed, nil
}

func (a *App) removeLegacyCodexProviderSessionsLocked() bool {
	kept := a.store.state.Sessions[:0]
	changed := false
	for _, session := range a.store.state.Sessions {
		if session.Role == "codex" {
			changed = true
			continue
		}
		kept = append(kept, session)
	}
	a.store.state.Sessions = kept
	return changed
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
	for _, item := range state.AccountOrders {
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
	for _, item := range state.ClientAccessKeys {
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

func (a *App) siteAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	path := strings.TrimPrefix(r.URL.Path, "/api/site")
	switch {
	case path == "/config" && r.Method == http.MethodGet:
		a.siteConfig(w)
	case path == "/orders" && r.Method == http.MethodPost:
		a.siteCreateAccountOrder(w, r)
	default:
		writeErr(w, http.StatusNotFound, "not_found")
	}
}

func (a *App) siteConfig(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "public, max-age=60")
	a.store.mu.Lock()
	plans := make([]map[string]any, 0)
	for _, topup := range a.store.state.TokenTopups {
		if !topup.Enabled || topup.PriceCents <= 0 || topup.Tokens <= 0 {
			continue
		}
		plans = append(plans, map[string]any{
			"id": topup.ID, "name": topup.Name, "priceCents": topup.PriceCents,
			"tokens": topup.Tokens, "description": topup.Description, "sort": topup.Sort,
		})
	}
	a.store.mu.Unlock()
	sort.SliceStable(plans, func(i, j int) bool {
		return intFromAny(plans[i]["sort"]) < intFromAny(plans[j]["sort"])
	})
	for _, plan := range plans {
		delete(plan, "sort")
	}
	version := strings.TrimSpace(os.Getenv("CODEXPPP_DESKTOP_LATEST_VERSION"))
	if version == "" {
		version = "0.1.60"
	}
	macVersion := strings.TrimSpace(os.Getenv("CODEXPPP_MACOS_DESKTOP_LATEST_VERSION"))
	writeJSON(w, http.StatusOK, map[string]any{
		"product":        "Codex+++",
		"version":        version,
		"downloadUrl":    safeHTTPURL(os.Getenv("CODEXPPP_DESKTOP_DOWNLOAD_URL")),
		"downloadSha256": strings.TrimSpace(os.Getenv("CODEXPPP_DESKTOP_DOWNLOAD_SHA256")),
		"downloads": map[string]any{
			"windows": map[string]any{
				"version": version,
				"url":     safeHTTPURL(os.Getenv("CODEXPPP_DESKTOP_DOWNLOAD_URL")),
				"sha256":  strings.TrimSpace(os.Getenv("CODEXPPP_DESKTOP_DOWNLOAD_SHA256")),
			},
			"macos": map[string]any{
				"version": macVersion,
				"url":     safeHTTPURL(os.Getenv("CODEXPPP_MACOS_DESKTOP_DOWNLOAD_URL")),
				"sha256":  strings.TrimSpace(os.Getenv("CODEXPPP_MACOS_DESKTOP_DOWNLOAD_SHA256")),
			},
		},
		"purchaseMode": "manual_review",
		"plans":        plans,
	})
}

func intFromAny(value any) int {
	switch number := value.(type) {
	case int:
		return number
	case int64:
		return int(number)
	case float64:
		return int(number)
	default:
		return 0
	}
}

func (a *App) siteCreateAccountOrder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TopupID          string `json:"topupId"`
		Contact          string `json:"contact"`
		PreferredAccount string `json:"preferredAccount"`
		Remark           string `json:"remark"`
		Website          string `json:"website"`
	}
	if !readStrictJSON(w, r, &req, "invalid_account_order_request", "topupId", "contact", "preferredAccount", "remark", "website") {
		return
	}
	req.TopupID = strings.TrimSpace(req.TopupID)
	req.Contact = strings.TrimSpace(req.Contact)
	req.PreferredAccount = strings.TrimSpace(req.PreferredAccount)
	req.Remark = strings.TrimSpace(req.Remark)
	if req.Website != "" || req.TopupID == "" || len([]rune(req.Contact)) < 3 || len([]rune(req.Contact)) > 120 || len([]rune(req.PreferredAccount)) > 64 || len([]rune(req.Remark)) > 300 {
		writeErr(w, http.StatusBadRequest, "invalid_account_order_request")
		return
	}
	if !a.allowSiteOrder(r, time.Now().UTC()) {
		writeErr(w, http.StatusTooManyRequests, "too_many_purchase_requests")
		return
	}
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	idx := a.topupIndex(req.TopupID)
	if idx < 0 || !a.store.state.TokenTopups[idx].Enabled || a.store.state.TokenTopups[idx].PriceCents <= 0 || a.store.state.TokenTopups[idx].Tokens <= 0 {
		writeErr(w, http.StatusBadRequest, "account_order_plan_unavailable")
		return
	}
	buyerJSON, err := json.Marshal(accountOrderBuyer{Contact: req.Contact, PreferredAccount: req.PreferredAccount, Remark: req.Remark})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "account_order_create_failed")
		return
	}
	buyerCipher, err := a.encrypt(string(buyerJSON))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "account_order_create_failed")
		return
	}
	topup := a.store.state.TokenTopups[idx]
	now := time.Now().UTC()
	order := AccountOrder{
		ID: a.store.nextID("ord"), BuyerCipher: buyerCipher, TopupID: topup.ID, TopupName: topup.Name,
		PriceCents: topup.PriceCents, Tokens: topup.Tokens, Status: accountOrderPending,
		CreatedAt: now, UpdatedAt: now,
	}
	a.store.state.AccountOrders = append(a.store.state.AccountOrders, order)
	if !a.saveOrErrorLocked(w) {
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"orderId": order.ID, "status": order.Status,
		"message": "购买申请已提交，工作人员将通过您填写的联系方式确认收款与账号发放。",
	})
}

func (a *App) allowSiteOrder(r *http.Request, now time.Time) bool {
	key := clientRequestAddress(r)
	a.siteOrderMu.Lock()
	defer a.siteOrderMu.Unlock()
	if a.siteOrderAttempts == nil {
		a.siteOrderAttempts = map[string][]time.Time{}
	}
	cutoff := now.Add(-time.Hour)
	if len(a.siteOrderAttempts) >= 10000 {
		for attemptKey, attempts := range a.siteOrderAttempts {
			kept := attempts[:0]
			for _, attemptedAt := range attempts {
				if attemptedAt.After(cutoff) {
					kept = append(kept, attemptedAt)
				}
			}
			if len(kept) == 0 {
				delete(a.siteOrderAttempts, attemptKey)
			} else {
				a.siteOrderAttempts[attemptKey] = kept
			}
		}
		if _, known := a.siteOrderAttempts[key]; !known && len(a.siteOrderAttempts) >= 10000 {
			return false
		}
	}
	recent := a.siteOrderAttempts[key][:0]
	for _, attemptedAt := range a.siteOrderAttempts[key] {
		if attemptedAt.After(cutoff) {
			recent = append(recent, attemptedAt)
		}
	}
	if len(recent) >= maxSiteOrdersPerHour {
		a.siteOrderAttempts[key] = recent
		return false
	}
	a.siteOrderAttempts[key] = append(recent, now)
	return true
}

func (a *App) allowAdminLoginAttempt(ctx context.Context, account, address string, now time.Time) (bool, error) {
	key := hashString(strings.ToLower(strings.TrimSpace(account)) + "\x00" + address)
	if a.gatewayLimiter != nil {
		return a.gatewayLimiter.Allow(ctx, "codexppp:admin:login:"+key, maxAdminLoginAttempts, adminLoginAttemptWindow)
	}
	a.adminLoginMu.Lock()
	defer a.adminLoginMu.Unlock()
	if a.adminLoginAttempts == nil {
		a.adminLoginAttempts = map[string][]time.Time{}
	}
	cutoff := now.Add(-adminLoginAttemptWindow)
	recent := a.adminLoginAttempts[key][:0]
	for _, attemptedAt := range a.adminLoginAttempts[key] {
		if attemptedAt.After(cutoff) {
			recent = append(recent, attemptedAt)
		}
	}
	if len(recent) >= maxAdminLoginAttempts {
		a.adminLoginAttempts[key] = recent
		return false, nil
	}
	a.adminLoginAttempts[key] = append(recent, now)
	return true, nil
}

func clientRequestAddress(r *http.Request) string {
	if r == nil {
		return "unknown"
	}
	remote := strings.TrimSpace(r.RemoteAddr)
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	parsed := net.ParseIP(strings.Trim(host, "[]"))
	if parsed != nil && parsed.IsLoopback() {
		if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
			return realIP
		}
		if forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0]); forwarded != "" {
			return forwarded
		}
	}
	if host == "" {
		return "unknown"
	}
	return host
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
	case path == "/account-orders" && r.Method == http.MethodGet:
		a.adminAccountOrders(w, r)
	case strings.HasPrefix(path, "/account-orders/"):
		a.adminAccountOrderAction(w, r, strings.TrimPrefix(path, "/account-orders/"))
	case path == "/recharges" && r.Method == http.MethodGet:
		a.adminRecharges(w, r)
	case strings.HasPrefix(path, "/recharges/"):
		a.adminRechargeAction(w, r, strings.TrimPrefix(path, "/recharges/"))
	case path == "/upstreams" && r.Method == http.MethodGet:
		a.adminUpstreams(w, r)
	case path == "/upstreams" && r.Method == http.MethodPost:
		a.adminCreateUpstream(w, r)
	case path == "/upstreams/export" && r.Method == http.MethodPost:
		a.adminExportUpstreams(w, r)
	case path == "/upstreams/import" && r.Method == http.MethodPost:
		a.adminImportUpstreams(w, r)
	case path == "/codex-oauth/start" && r.Method == http.MethodPost:
		a.adminStartCodexOAuth(w, r)
	case path == "/codex-oauth/callback" && r.Method == http.MethodPost:
		a.adminCodexOAuthCallback(w, r)
	case path == "/codex-oauth/cancel" && r.Method == http.MethodPost:
		a.adminCancelCodexOAuth(w, r)
	case path == "/codex-oauth/status" && r.Method == http.MethodGet:
		a.adminCodexOAuthStatus(w, r)
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
	case path == "/usage/analytics" && r.Method == http.MethodGet:
		a.adminUsageAnalytics(w, r)
	case path == "/ledger" && r.Method == http.MethodGet:
		a.adminLedger(w, r)
	case path == "/audit" && r.Method == http.MethodGet:
		a.adminAudit(w, r)
	default:
		writeErr(w, http.StatusNotFound, "not_found")
	}
}

func (a *App) clientAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(clientInteropHeader, clientInteropMajor)
	if !requireClientInterop(w, r) {
		return
	}
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
	case path == "/launch/heartbeat" && r.Method == http.MethodPost:
		a.clientLaunchHeartbeat(w, r)
	case path == "/launch/stop" && r.Method == http.MethodPost:
		a.clientLaunchStop(w, r)
	case path == "/presence/heartbeat" && r.Method == http.MethodPost:
		a.clientPresenceHeartbeat(w, r)
	case path == "/presence/stop" && r.Method == http.MethodPost:
		a.clientPresenceStop(w, r)
	default:
		writeErr(w, http.StatusNotFound, "not_found")
	}
}

func requireClientInterop(w http.ResponseWriter, r *http.Request) bool {
	if strings.TrimSpace(r.Header.Get(clientInteropHeader)) == clientInteropMajor {
		return true
	}
	writeErr(w, http.StatusUpgradeRequired, "client_version_incompatible")
	return false
}

func (a *App) codexAPI(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/codex")
	switch {
	case (path == "/v1/models" || path == "/v1/models/") && r.Method == http.MethodGet:
		user, _, _, ok := a.requireCodexClient(w, r)
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

func (a *App) requireCodexClient(w http.ResponseWriter, r *http.Request) (User, Device, ClientAccessKey, bool) {
	user, device, accessKey, ok := a.codexClientFromRequest(r)
	if !ok {
		writeCodexProviderError(w, http.StatusUnauthorized, "login_failed")
		return User{}, Device{}, ClientAccessKey{}, false
	}
	return user, device, accessKey, true
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
	allowed, err := a.allowAdminLoginAttempt(r.Context(), req.Account, clientRequestAddress(r), time.Now().UTC())
	if err != nil {
		writeErr(w, http.StatusServiceUnavailable, "admin_login_unavailable")
		return
	}
	if !allowed {
		writeErr(w, http.StatusTooManyRequests, "login_rate_limited")
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
	var pending, pendingOrders, available int
	var todayTokens int64
	start, end := utcDayBounds(time.Now())
	for _, rr := range a.store.state.RechargeRequests {
		if rr.Status == rechargePending {
			pending++
		}
	}
	for _, order := range a.store.state.AccountOrders {
		if order.Status == accountOrderPending || order.Status == accountOrderContacted {
			pendingOrders++
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
	writeJSON(w, http.StatusOK, map[string]any{"pendingRecharges": pending, "pendingAccountOrders": pendingOrders, "availableUpstreams": available, "todayTokens": todayTokens})
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

func (a *App) adminAccountOrders(w http.ResponseWriter, r *http.Request) {
	_, ok := a.requireAdmin(w, r, false)
	if !ok {
		return
	}
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	if status != "" && !validAccountOrderStatus(status) {
		writeErr(w, http.StatusBadRequest, "invalid_account_order_status")
		return
	}
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	items := make([]map[string]any, 0)
	for _, order := range a.store.state.AccountOrders {
		if status != "" && order.Status != status {
			continue
		}
		items = append(items, a.publicAdminAccountOrderLocked(order))
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i]["createdAt"].(time.Time).After(items[j]["createdAt"].(time.Time))
	})
	writeJSON(w, http.StatusOK, listPayload(items, r))
}

func (a *App) adminAccountOrderAction(w http.ResponseWriter, r *http.Request, rest string) {
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
		Status      string `json:"status"`
		UserAccount string `json:"userAccount"`
		AdminRemark string `json:"adminRemark"`
	}
	if !readStrictJSON(w, r, &req, "invalid_account_order_request", "status", "userAccount", "adminRemark") {
		return
	}
	req.Status = strings.TrimSpace(req.Status)
	req.UserAccount = strings.TrimSpace(req.UserAccount)
	req.AdminRemark = strings.TrimSpace(req.AdminRemark)
	if !validAccountOrderStatus(req.Status) || req.Status == accountOrderPending || len([]rune(req.AdminRemark)) > 300 {
		writeErr(w, http.StatusBadRequest, "invalid_account_order_status")
		return
	}
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	idx := a.accountOrderIndex(parts[0])
	if idx < 0 {
		writeErr(w, http.StatusNotFound, "account_order_not_found")
		return
	}
	order := &a.store.state.AccountOrders[idx]
	if order.Status == accountOrderFulfilled || order.Status == accountOrderRejected {
		writeErr(w, http.StatusConflict, "account_order_already_closed")
		return
	}
	now := time.Now().UTC()
	switch req.Status {
	case accountOrderContacted:
		order.Status = accountOrderContacted
	case accountOrderRejected:
		order.Status = accountOrderRejected
	case accountOrderFulfilled:
		userID := ""
		for _, user := range a.store.state.Users {
			if strings.EqualFold(user.Account, req.UserAccount) && user.Status == statusActive {
				userID = user.ID
				break
			}
		}
		if userID == "" {
			writeErr(w, http.StatusBadRequest, "account_order_user_not_found")
			return
		}
		if err := a.applyTokenDeltaLocked(userID, order.Tokens, "recharge", "官网购买："+order.TopupName); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		order.Status = accountOrderFulfilled
		order.UserID = userID
		order.FulfilledAt = &now
		a.auditLocked(admin.ID, "admin", "account_order.fulfill", order.ID, fmt.Sprintf("user=%s tokens=%d", userID, order.Tokens))
	}
	order.AdminRemark = req.AdminRemark
	order.UpdatedAt = now
	if req.Status != accountOrderFulfilled {
		a.auditLocked(admin.ID, "admin", "account_order.status", order.ID, req.Status)
	}
	if !a.saveOrErrorLocked(w) {
		return
	}
	writeJSON(w, http.StatusOK, a.publicAdminAccountOrderLocked(*order))
}

func validAccountOrderStatus(status string) bool {
	return status == accountOrderPending || status == accountOrderContacted || status == accountOrderFulfilled || status == accountOrderRejected
}

func (a *App) publicAdminAccountOrderLocked(order AccountOrder) map[string]any {
	buyer := accountOrderBuyer{}
	if raw, err := a.decrypt(order.BuyerCipher); err == nil {
		_ = json.Unmarshal([]byte(raw), &buyer)
	}
	userAccount := ""
	if order.UserID != "" {
		userAccount = a.userByID(order.UserID).Account
	}
	return map[string]any{
		"id": order.ID, "contact": buyer.Contact, "preferredAccount": buyer.PreferredAccount,
		"remark": buyer.Remark, "topupId": order.TopupID, "topupName": order.TopupName,
		"priceCents": order.PriceCents, "tokens": order.Tokens, "status": order.Status,
		"userAccount": userAccount, "adminRemark": order.AdminRemark, "createdAt": order.CreatedAt,
		"updatedAt": order.UpdatedAt, "fulfilledAt": order.FulfilledAt,
	}
}

func (a *App) accountOrderIndex(id string) int {
	for i := range a.store.state.AccountOrders {
		if a.store.state.AccountOrders[i].ID == id {
			return i
		}
	}
	return -1
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
	authorization := strings.TrimSpace(r.URL.Query().Get("authorization"))
	if authorization != "" && authorization != "authorized" && authorization != "pending" {
		writeErr(w, http.StatusBadRequest, "invalid_upstream_authorization_filter")
		return
	}
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	items := make([]map[string]any, 0)
	for _, up := range a.store.state.UpstreamAccounts {
		isAvailable := upstreamIsAvailable(up)
		if available == "true" && !isAvailable {
			continue
		}
		if authorization == "authorized" && up.AuthorizationStatus != upstreamAuthAuthorized {
			continue
		}
		if authorization == "pending" && up.AuthorizationStatus == upstreamAuthAuthorized {
			continue
		}
		items = append(items, a.publicAdminUpstreamLocked(up))
	}
	payload := listPayload(items, r)
	now := time.Now().UTC()
	activeClients, unassignedClients := a.activeClientAccountsLocked(now)
	payload["onlineClientAccounts"] = a.onlineClientAccountsLocked(now)
	payload["activeClientAccounts"] = activeClients
	payload["unassignedClientAccounts"] = unassignedClients
	writeJSON(w, http.StatusOK, payload)
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
	if up.AuthorizationStatus == upstreamAuthAuthorized {
		key, err := a.newAPIKeyForUpstreamLocked(up.ID, now)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "key_generate_failed")
			return
		}
		a.store.state.APIKeys = append(a.store.state.APIKeys, key)
		a.auditLocked(admin.ID, "admin", "api_key.create", key.ID, up.ID)
	}
	if !a.saveOrErrorLocked(w) {
		return
	}
	writeJSON(w, http.StatusCreated, publicUpstream(up))
}

func (a *App) adminExportUpstreams(w http.ResponseWriter, r *http.Request) {
	admin, ok := a.requireAdmin(w, r, false)
	if !ok {
		return
	}
	if !a.adminSessionIsRecent(r, 15*time.Minute) {
		writeErr(w, http.StatusForbidden, "reauth_required")
		return
	}
	if !readEmptyJSON(w, r, "invalid_upstream_export_request") {
		return
	}
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	accounts := make([]map[string]any, 0, len(a.store.state.UpstreamAccounts))
	for _, up := range a.store.state.UpstreamAccounts {
		account, err := a.exportUpstreamAccountLocked(up)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		accounts = append(accounts, account)
	}
	now := time.Now().UTC()
	a.auditLocked(admin.ID, "admin", "upstream.export", "account_pool", fmt.Sprintf("accounts=%d", len(accounts)))
	if !a.saveOrErrorLocked(w) {
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="codexppp-account-pool-%s.json"`, now.Format("20060102-150405")))
	writeJSON(w, http.StatusOK, map[string]any{
		"format":     "codexppp-account-pool",
		"version":    1,
		"exportedAt": now,
		"accounts":   accounts,
	})
}

func (a *App) adminSessionIsRecent(r *http.Request, maximumAge time.Duration) bool {
	token := bearerToken(r)
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	session, ok := a.sessionLocked(token, sessionRoleAdmin)
	return ok && time.Since(session.CreatedAt) <= maximumAge
}

func (a *App) exportUpstreamAccountLocked(up UpstreamAccount) (map[string]any, error) {
	base := map[string]any{
		"name":              up.Name,
		"group":             up.Group,
		"remark":            up.Remark,
		"sourceType":        up.SourceType,
		"email":             up.Email,
		"subscriptionTier":  up.SubscriptionTier,
		"entitlementStatus": up.EntitlementStatus,
	}
	if up.ExpiresAt != nil {
		base["expiresAt"] = up.ExpiresAt
	}
	if strings.TrimSpace(up.PasswordCipher) != "" {
		password, err := a.decrypt(up.PasswordCipher)
		if err != nil {
			return nil, errors.New("secret_decrypt_failed")
		}
		base["credentialType"] = "email_password"
		base["password"] = password
		return base, nil
	}
	if strings.TrimSpace(up.AuthJSONCipher) != "" {
		raw, err := a.decrypt(up.AuthJSONCipher)
		if err != nil {
			return nil, errors.New("secret_decrypt_failed")
		}
		dec := json.NewDecoder(strings.NewReader(raw))
		dec.UseNumber()
		var authJSON map[string]any
		if err := dec.Decode(&authJSON); err != nil {
			return nil, errors.New("upstream_export_auth_json_invalid")
		}
		for key, value := range base {
			authJSON[key] = value
		}
		return authJSON, nil
	}
	accessToken, err := a.decrypt(up.AccessTokenCipher)
	if err != nil {
		return nil, errors.New("secret_decrypt_failed")
	}
	refreshToken := ""
	if strings.TrimSpace(up.RefreshTokenCipher) != "" {
		refreshToken, err = a.decrypt(up.RefreshTokenCipher)
		if err != nil {
			return nil, errors.New("secret_decrypt_failed")
		}
	}
	base["credentialType"] = "oauth"
	base["tokenType"] = valueOr(up.TokenType, "Bearer")
	base["chatgptAccountId"] = up.ChatGPTAccountID
	base["accessToken"] = accessToken
	base["refreshToken"] = refreshToken
	return base, nil
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
	result, err := a.importUpstreamRequestsLocked(admin, reqs, time.Now().UTC())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !a.saveOrErrorLocked(w) {
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (a *App) importUpstreamRequestsLocked(admin Admin, reqs []adminUpstreamRequest, now time.Time) (upstreamImportResult, error) {
	result := upstreamImportResult{
		Items:   make([]map[string]any, 0, len(reqs)),
		APIKeys: make([]map[string]any, 0, len(reqs)),
	}
	for _, req := range reqs {
		if idx := a.upstreamDuplicateIndexLocked(req); idx >= 0 {
			up := &a.store.state.UpstreamAccounts[idx]
			if err := a.applyUpstreamRequestLocked(up, req, now); err != nil {
				return upstreamImportResult{}, errors.New("secret_encrypt_failed")
			}
			result.Updated++
			a.auditLocked(admin.ID, "admin", "upstream.import.update", up.ID, up.Name)
			result.Items = append(result.Items, publicUpstream(*up))
			if up.AuthorizationStatus == upstreamAuthAuthorized {
				key, createdKey, err := a.ensureAPIKeyForUpstreamLocked(up.ID, now)
				if err != nil {
					return upstreamImportResult{}, errors.New("key_generate_failed")
				}
				if createdKey {
					a.auditLocked(admin.ID, "admin", "api_key.create", key.ID, up.ID)
				}
				result.APIKeys = append(result.APIKeys, a.publicAPIKeyLocked(key))
			} else {
				result.Pending++
			}
			continue
		}
		up, err := a.newUpstreamFromRequestLocked(req, now)
		if err != nil {
			return upstreamImportResult{}, errors.New("secret_encrypt_failed")
		}
		a.store.state.UpstreamAccounts = append(a.store.state.UpstreamAccounts, up)
		a.auditLocked(admin.ID, "admin", "upstream.import", up.ID, up.Name)
		result.Imported++
		result.Items = append(result.Items, publicUpstream(up))
		if up.AuthorizationStatus == upstreamAuthAuthorized {
			key, err := a.newAPIKeyForUpstreamLocked(up.ID, now)
			if err != nil {
				return upstreamImportResult{}, errors.New("key_generate_failed")
			}
			a.store.state.APIKeys = append(a.store.state.APIKeys, key)
			a.auditLocked(admin.ID, "admin", "api_key.create", key.ID, up.ID)
			result.APIKeys = append(result.APIKeys, a.publicAPIKeyLocked(key))
		} else {
			result.Pending++
		}
	}
	return result, nil
}

func (a *App) adminStartCodexOAuth(w http.ResponseWriter, r *http.Request) {
	admin, ok := a.requireAdmin(w, r, false)
	if !ok {
		return
	}
	req, ok := readCodexOAuthStartRequest(w, r)
	if !ok {
		return
	}
	if req.UpstreamID != "" {
		a.store.mu.Lock()
		exists := a.upstreamIndex(req.UpstreamID) >= 0
		a.store.mu.Unlock()
		if !exists {
			writeErr(w, http.StatusNotFound, "upstream_not_found")
			return
		}
	}
	var login *codexDeviceCodeLogin
	var err error
	if req.Method == "browser" {
		login, err = codexBrowserLoginStart(context.Background(), codexOAuthSessionTTL)
	} else {
		login, err = codexDeviceCodeLoginStart(context.Background(), codexOAuthSessionTTL)
	}
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	session := a.createCodexOAuthSession(admin.ID, req.UpstreamID, login)
	if req.UpstreamID != "" {
		a.store.mu.Lock()
		if idx := a.upstreamIndex(req.UpstreamID); idx >= 0 {
			up := &a.store.state.UpstreamAccounts[idx]
			if up.AuthorizationStatus != upstreamAuthAuthorized || strings.TrimSpace(up.AccessTokenCipher) == "" {
				up.AuthorizationStatus = upstreamAuthAction
				up.LastAuthorizationError = ""
				up.UpdatedAt = time.Now().UTC()
			}
		}
		if err := a.store.save(); err != nil {
			a.store.mu.Unlock()
			if login.Cleanup != nil {
				login.Cleanup()
			}
			a.oauthMu.Lock()
			delete(a.oauthSessions, session.State)
			delete(a.oauthLogins, session.State)
			a.oauthMu.Unlock()
			writeErr(w, http.StatusInternalServerError, "store_save_failed")
			return
		}
		a.store.mu.Unlock()
	}
	if login.Wait != nil {
		go a.completeCodexDeviceCodeLogin(session.State, login)
	}
	writeJSON(w, http.StatusOK, publicCodexOAuthSession(session))
}

type codexOAuthStartRequest struct {
	Method     string `json:"method"`
	UpstreamID string `json:"upstreamId"`
}

func readCodexOAuthStartRequest(w http.ResponseWriter, r *http.Request) (codexOAuthStartRequest, bool) {
	defer r.Body.Close()
	dec := json.NewDecoder(io.LimitReader(r.Body, maxJSONBodyBytes+1))
	dec.DisallowUnknownFields()
	var req codexOAuthStartRequest
	if err := dec.Decode(&req); err != nil && err != io.EOF {
		writeErr(w, http.StatusBadRequest, "invalid_codex_oauth_start_request")
		return codexOAuthStartRequest{}, false
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		writeErr(w, http.StatusBadRequest, "invalid_codex_oauth_start_request")
		return codexOAuthStartRequest{}, false
	}
	req.Method = strings.ToLower(strings.TrimSpace(req.Method))
	req.UpstreamID = strings.TrimSpace(req.UpstreamID)
	if req.Method == "" {
		req.Method = "device_code"
	}
	if req.Method != "device_code" && req.Method != "browser" {
		writeErr(w, http.StatusBadRequest, "invalid_codex_oauth_start_request")
		return codexOAuthStartRequest{}, false
	}
	return req, true
}

func (a *App) adminCodexOAuthStatus(w http.ResponseWriter, r *http.Request) {
	admin, ok := a.requireAdmin(w, r, false)
	if !ok {
		return
	}
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	if state == "" {
		writeErr(w, http.StatusBadRequest, "invalid_codex_oauth_status_request")
		return
	}
	session, ok := a.codexOAuthSessionSnapshot(state)
	if !ok || session.AdminID != admin.ID {
		writeErr(w, http.StatusNotFound, "codex_oauth_session_not_found")
		return
	}
	writeJSON(w, http.StatusOK, publicCodexOAuthSession(session))
}

func (a *App) adminCodexOAuthCallback(w http.ResponseWriter, r *http.Request) {
	admin, ok := a.requireAdmin(w, r, false)
	if !ok {
		return
	}
	var req struct {
		State       string `json:"state"`
		CallbackURL string `json:"callbackUrl"`
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, maxJSONBodyBytes+1))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_codex_oauth_callback_request")
		return
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		writeErr(w, http.StatusBadRequest, "invalid_codex_oauth_callback_request")
		return
	}
	req.State = strings.TrimSpace(req.State)
	req.CallbackURL = strings.TrimSpace(req.CallbackURL)
	a.oauthMu.Lock()
	a.ensureOAuthSessionsLocked()
	session, exists := a.oauthSessions[req.State]
	login := a.oauthLogins[req.State]
	a.oauthMu.Unlock()
	if !exists || session.AdminID != admin.ID {
		writeErr(w, http.StatusNotFound, "codex_oauth_session_not_found")
		return
	}
	if time.Now().UTC().After(session.ExpiresAt) {
		writeErr(w, http.StatusGone, "codex_oauth_session_expired")
		return
	}
	if session.AuthMethod != "app_server_browser" || login == nil || login.Callback == nil {
		writeErr(w, http.StatusBadRequest, "codex_oauth_callback_not_supported")
		return
	}
	if err := login.Callback(r.Context(), req.CallbackURL); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	a.updateCodexOAuthSession(req.State, func(s *codexOAuthSession) {
		if s.Status == "pending" {
			s.Status = "callback_received"
		}
	})
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "callback_received", "state": req.State})
}

func (a *App) adminCancelCodexOAuth(w http.ResponseWriter, r *http.Request) {
	admin, ok := a.requireAdmin(w, r, false)
	if !ok {
		return
	}
	var req struct {
		State string `json:"state"`
	}
	if !readStrictJSON(w, r, &req, "invalid_codex_oauth_cancel_request", "state") {
		return
	}
	req.State = strings.TrimSpace(req.State)
	a.oauthMu.Lock()
	a.ensureOAuthSessionsLocked()
	session, exists := a.oauthSessions[req.State]
	if !exists || session.AdminID != admin.ID {
		a.oauthMu.Unlock()
		writeErr(w, http.StatusNotFound, "codex_oauth_session_not_found")
		return
	}
	login := a.oauthLogins[req.State]
	if session.Status != "imported" {
		session.Status = "cancelled"
		session.Error = ""
		a.oauthSessions[req.State] = session
		delete(a.oauthLogins, req.State)
	}
	a.oauthMu.Unlock()
	if login != nil && login.Cleanup != nil {
		login.Cleanup()
	}
	if session.UpstreamID != "" && session.Status != "imported" {
		a.store.mu.Lock()
		if idx := a.upstreamIndex(session.UpstreamID); idx >= 0 {
			up := &a.store.state.UpstreamAccounts[idx]
			if up.AuthorizationStatus != upstreamAuthAuthorized || strings.TrimSpace(up.AccessTokenCipher) == "" {
				up.AuthorizationStatus = upstreamAuthPending
				up.LastAuthorizationError = ""
				up.Status = statusDisabled
				up.BalanceStatus = "unavailable"
				up.RiskStatus = "unavailable"
				up.UpdatedAt = time.Now().UTC()
			}
		}
		if err := a.store.save(); err != nil {
			a.store.mu.Unlock()
			writeErr(w, http.StatusInternalServerError, "store_save_failed")
			return
		}
		a.store.mu.Unlock()
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": session.Status, "state": req.State})
}

func (a *App) createCodexOAuthSession(adminID, upstreamID string, login *codexDeviceCodeLogin) codexOAuthSession {
	now := time.Now().UTC()
	session := codexOAuthSession{
		State:      randomToken(24),
		AdminID:    adminID,
		Status:     "pending",
		AuthMethod: "app_server_device_code",
		UpstreamID: upstreamID,
		CreatedAt:  now,
		ExpiresAt:  now.Add(codexOAuthSessionTTL),
	}
	if login != nil {
		if login.AuthMethod != "" {
			session.AuthMethod = login.AuthMethod
		}
		session.VerificationURL = login.VerificationURL
		session.UserCode = login.UserCode
		session.LoginID = login.LoginID
	}
	a.oauthMu.Lock()
	a.ensureOAuthSessionsLocked()
	a.pruneCodexOAuthSessionsLocked(now)
	a.oauthSessions[session.State] = session
	a.oauthLogins[session.State] = login
	a.oauthMu.Unlock()
	return session
}

func (a *App) completeCodexDeviceCodeLogin(state string, login *codexDeviceCodeLogin) {
	if login == nil {
		return
	}
	if login.Cleanup != nil {
		defer login.Cleanup()
	}
	if login.Wait == nil {
		return
	}
	authJSONRaw, err := login.Wait(context.Background())
	if err != nil {
		a.failCodexOAuthSession(state, err.Error())
		return
	}
	req, err := codexAuthRequestFromAuthJSONRaw(authJSONRaw)
	if err != nil {
		a.failCodexOAuthSession(state, err.Error())
		return
	}
	now := time.Now().UTC()
	session, ok := a.codexOAuthSessionInternalSnapshot(state)
	if !ok || session.Status == "cancelled" {
		return
	}
	if session.AuthMethod == "app_server_browser" {
		req.SourceType = "browser_oauth"
	} else {
		req.SourceType = "device_code"
	}
	a.store.mu.Lock()
	admin := a.adminByID(session.AdminID)
	if admin.ID == "" {
		a.store.mu.Unlock()
		a.failCodexOAuthSession(state, "admin_not_found")
		return
	}
	var result upstreamImportResult
	if session.UpstreamID != "" {
		result, err = a.applyAuthorizedUpstreamRequestLocked(admin, session.UpstreamID, req, now)
	} else {
		result, err = a.importUpstreamRequestsLocked(admin, []adminUpstreamRequest{req}, now)
	}
	if err == nil {
		err = a.store.save()
	}
	a.store.mu.Unlock()
	if err != nil {
		a.failCodexOAuthSession(state, err.Error())
		return
	}

	keyPreview, accountName := codexOAuthResultSummary(result)
	a.updateCodexOAuthSession(state, func(s *codexOAuthSession) {
		s.Status = "imported"
		s.Imported = result.Imported
		s.Updated = result.Updated
		s.KeyPreview = keyPreview
		s.AccountName = accountName
	})
	a.oauthMu.Lock()
	delete(a.oauthLogins, state)
	a.oauthMu.Unlock()
}

func (a *App) applyAuthorizedUpstreamRequestLocked(admin Admin, upstreamID string, req adminUpstreamRequest, now time.Time) (upstreamImportResult, error) {
	idx := a.upstreamIndex(upstreamID)
	if idx < 0 {
		return upstreamImportResult{}, errors.New("upstream_not_found")
	}
	up := &a.store.state.UpstreamAccounts[idx]
	if looksLikeEmail(up.Email) && looksLikeEmail(req.Email) && !strings.EqualFold(strings.TrimSpace(up.Email), strings.TrimSpace(req.Email)) {
		return upstreamImportResult{}, errors.New("codex_oauth_email_mismatch")
	}
	req.Name = firstNonEmptyText(up.Name, req.Name, req.Email)
	req.Group = firstNonEmptyText(up.Group, req.Group)
	req.Email = firstNonEmptyText(req.Email, up.Email)
	if err := a.applyUpstreamRequestLocked(up, req, now); err != nil {
		return upstreamImportResult{}, errors.New("secret_encrypt_failed")
	}
	key, created, err := a.ensureAPIKeyForUpstreamLocked(up.ID, now)
	if err != nil {
		return upstreamImportResult{}, errors.New("key_generate_failed")
	}
	a.auditLocked(admin.ID, "admin", "upstream.authorize", up.ID, up.Name)
	if created {
		a.auditLocked(admin.ID, "admin", "api_key.create", key.ID, up.ID)
	}
	return upstreamImportResult{
		Updated: 1,
		Items:   []map[string]any{publicUpstream(*up)},
		APIKeys: []map[string]any{a.publicAPIKeyLocked(key)},
	}, nil
}

func (a *App) failCodexOAuthSession(state, code string) {
	code = sanitizeCodexOAuthError(code)
	a.oauthMu.Lock()
	a.ensureOAuthSessionsLocked()
	session, exists := a.oauthSessions[state]
	if exists && session.Status == "cancelled" {
		delete(a.oauthLogins, state)
		a.oauthMu.Unlock()
		return
	}
	if exists {
		session.Status = "failed"
		session.Error = code
		a.oauthSessions[state] = session
		delete(a.oauthLogins, state)
	}
	a.oauthMu.Unlock()
	if !exists || session.UpstreamID == "" {
		return
	}
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	if idx := a.upstreamIndex(session.UpstreamID); idx >= 0 {
		up := &a.store.state.UpstreamAccounts[idx]
		up.LastAuthorizationError = code
		if up.AuthorizationStatus != upstreamAuthAuthorized || strings.TrimSpace(up.AccessTokenCipher) == "" {
			up.AuthorizationStatus = upstreamAuthFailed
			up.Status = statusDisabled
			up.BalanceStatus = "unavailable"
			up.RiskStatus = "unavailable"
		}
		up.UpdatedAt = time.Now().UTC()
		if err := a.store.save(); err != nil {
			log.Printf("save upstream authorization failure failed: %v", err)
		}
	}
}

func sanitizeCodexOAuthError(code string) string {
	code = strings.TrimSpace(code)
	if code == "" || len(code) > 96 {
		return "codex_oauth_login_failed"
	}
	for _, r := range code {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' {
			return "codex_oauth_login_failed"
		}
	}
	for _, prefix := range []string{"codex_", "upstream_", "secret_", "key_", "store_", "admin_"} {
		if strings.HasPrefix(code, prefix) {
			return code
		}
	}
	return "codex_oauth_login_failed"
}

func codexAuthRequestFromAuthJSONRaw(authJSONRaw string) (adminUpstreamRequest, error) {
	var authJSON map[string]any
	dec := json.NewDecoder(strings.NewReader(authJSONRaw))
	dec.UseNumber()
	if err := dec.Decode(&authJSON); err != nil {
		return adminUpstreamRequest{}, errors.New("codex_device_code_auth_json_invalid")
	}
	req, handled, err := parseCodexAuthJSONImport(authJSON, 1, "")
	if err != nil {
		return adminUpstreamRequest{}, err
	}
	if !handled {
		return adminUpstreamRequest{}, errors.New("codex_device_code_auth_json_invalid")
	}
	if req.Name == "" {
		req.Name = "Codex device-code account"
	}
	if strings.TrimSpace(req.RefreshToken) == "" {
		return adminUpstreamRequest{}, errors.New("codex_device_code_refresh_token_missing")
	}
	return normalizeAdminUpstreamRequest(req)
}

func startCodexDeviceCodeLogin(ctx context.Context, ttl time.Duration) (*codexDeviceCodeLogin, error) {
	return startCodexManagedLogin(ctx, ttl, "device_code")
}

func startCodexBrowserLogin(ctx context.Context, ttl time.Duration) (*codexDeviceCodeLogin, error) {
	return startCodexManagedLogin(ctx, ttl, "browser")
}

func startCodexManagedLogin(_ context.Context, ttl time.Duration, flow string) (*codexDeviceCodeLogin, error) {
	if ttl <= 0 {
		ttl = codexOAuthSessionTTL
	}
	authDir, err := os.MkdirTemp("", "codexppp-managed-auth-*")
	if err != nil {
		return nil, errors.New("codex_device_code_auth_home_failed")
	}
	cleanupAuthDir := true
	cleanupDir := func() {
		if cleanupAuthDir {
			_ = os.RemoveAll(authDir)
		}
	}
	if err := writeCodexDeviceAuthConfig(authDir); err != nil {
		cleanupDir()
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), ttl)
	cmdName := valueOr(os.Getenv("CODEXPPP_CODEX_COMMAND"), "codex")
	cmd := exec.CommandContext(ctx, cmdName, "app-server", "--listen", "stdio://")
	cmd.Env = append(os.Environ(), "CODEX_HOME="+authDir)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		cleanupDir()
		return nil, errors.New("codex_app_server_start_failed")
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		cleanupDir()
		return nil, errors.New("codex_app_server_start_failed")
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		cancel()
		cleanupDir()
		return nil, errors.New("codex_app_server_start_failed")
	}

	var cleanupOnce sync.Once
	cleanup := func() {
		cleanupOnce.Do(func() {
			cancel()
			_ = stdin.Close()
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			_ = cmd.Wait()
			cleanupDir()
		})
	}
	dec := json.NewDecoder(stdout)
	enc := json.NewEncoder(stdin)
	send := func(message map[string]any) error {
		if err := enc.Encode(message); err != nil {
			return errors.New("codex_app_server_write_failed")
		}
		return nil
	}
	readResult := func(id int) (map[string]any, error) {
		for {
			var msg map[string]any
			if err := dec.Decode(&msg); err != nil {
				if ctx.Err() != nil {
					return nil, errors.New("codex_device_code_timeout")
				}
				return nil, errors.New("codex_app_server_read_failed")
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
			respondToCodexDeviceServerRequest(enc, msg)
		}
	}

	if err := send(map[string]any{"id": 1, "method": "initialize", "params": map[string]any{"clientInfo": map[string]any{"name": "Codex+++", "version": "0.1.0"}, "capabilities": map[string]any{}}}); err != nil {
		cleanup()
		return nil, err
	}
	if _, err := readResult(1); err != nil {
		cleanup()
		return nil, err
	}
	if err := send(map[string]any{"method": "initialized"}); err != nil {
		cleanup()
		return nil, err
	}
	loginParams := map[string]any{"type": "chatgptDeviceCode"}
	if flow == "browser" {
		loginParams = map[string]any{"type": "chatgpt", "useHostedLoginSuccessPage": true, "appBrand": "chatgpt"}
	}
	if err := send(map[string]any{"id": 2, "method": "account/login/start", "params": loginParams}); err != nil {
		cleanup()
		return nil, err
	}
	result, err := readResult(2)
	if err != nil {
		cleanup()
		return nil, err
	}
	verificationURL := firstNonEmptyText(importStringField(result, "verificationUrl", "verification_url"), importStringField(result, "verificationUri", "verification_uri"))
	if flow == "browser" {
		verificationURL = importStringField(result, "authUrl", "auth_url")
	}
	userCode := importStringField(result, "userCode", "user_code")
	loginID := importStringField(result, "loginId", "login_id")
	if verificationURL == "" || (flow != "browser" && userCode == "") {
		cleanup()
		return nil, errors.New("codex_device_code_start_response_invalid")
	}
	login := &codexDeviceCodeLogin{
		AuthMethod:      "app_server_device_code",
		VerificationURL: verificationURL,
		UserCode:        userCode,
		LoginID:         loginID,
		Cleanup:         cleanup,
	}
	if flow == "browser" {
		login.AuthMethod = "app_server_browser"
		authURL, parseErr := url.Parse(verificationURL)
		if parseErr != nil {
			cleanup()
			return nil, errors.New("codex_browser_start_response_invalid")
		}
		login.ExpectedState = authURL.Query().Get("state")
		if strings.TrimSpace(login.ExpectedState) == "" {
			cleanup()
			return nil, errors.New("codex_browser_start_response_invalid")
		}
		login.Callback = func(callbackCtx context.Context, callbackURL string) error {
			validated, err := validateCodexBrowserCallbackURL(callbackURL, login.ExpectedState)
			if err != nil {
				return err
			}
			request, err := http.NewRequestWithContext(callbackCtx, http.MethodGet, validated, nil)
			if err != nil {
				return errors.New("codex_oauth_callback_invalid")
			}
			client := &http.Client{
				Timeout:   10 * time.Second,
				Transport: &http.Transport{Proxy: nil},
				CheckRedirect: func(*http.Request, []*http.Request) error {
					return http.ErrUseLastResponse
				},
			}
			response, err := client.Do(request)
			if err != nil {
				return errors.New("codex_oauth_callback_forward_failed")
			}
			defer response.Body.Close()
			_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
			if response.StatusCode >= 400 {
				return errors.New("codex_oauth_callback_rejected")
			}
			return nil
		}
	}
	login.Wait = func(waitCtx context.Context) (string, error) {
		return waitCodexDeviceCodeAuthJSON(ctx, waitCtx, dec, enc, loginID, authDir)
	}
	return login, nil
}

func validateCodexBrowserCallbackURL(raw, expectedState string) (string, error) {
	if strings.TrimSpace(expectedState) == "" {
		return "", errors.New("codex_oauth_callback_state_mismatch")
	}
	callbackURL, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || callbackURL.Scheme != "http" || callbackURL.User != nil {
		return "", errors.New("codex_oauth_callback_invalid")
	}
	host := strings.ToLower(strings.TrimSpace(callbackURL.Hostname()))
	if host != "localhost" && host != "127.0.0.1" && host != "::1" {
		return "", errors.New("codex_oauth_callback_host_invalid")
	}
	port, err := strconv.Atoi(callbackURL.Port())
	if err != nil || port < 1 || port > 65535 || callbackURL.Path != "/auth/callback" {
		return "", errors.New("codex_oauth_callback_invalid")
	}
	state := strings.TrimSpace(callbackURL.Query().Get("state"))
	if state == "" || !hmac.Equal([]byte(state), []byte(expectedState)) {
		return "", errors.New("codex_oauth_callback_state_mismatch")
	}
	query := callbackURL.Query()
	if strings.TrimSpace(query.Get("code")) == "" && strings.TrimSpace(query.Get("error")) == "" {
		return "", errors.New("codex_oauth_callback_invalid")
	}
	callbackURL.Fragment = ""
	return callbackURL.String(), nil
}

func writeCodexDeviceAuthConfig(authDir string) error {
	if err := os.MkdirAll(authDir, 0700); err != nil {
		return errors.New("codex_device_code_auth_home_failed")
	}
	const config = "cli_auth_credentials_store = \"file\"\n"
	if err := os.WriteFile(filepath.Join(authDir, "config.toml"), []byte(config), 0600); err != nil {
		return errors.New("codex_device_code_auth_home_failed")
	}
	return nil
}

func waitCodexDeviceCodeAuthJSON(cmdCtx, waitCtx context.Context, dec *json.Decoder, enc *json.Encoder, loginID, authDir string) (string, error) {
	completed := false
	updated := false
	for {
		select {
		case <-waitCtx.Done():
			return "", errors.New("codex_device_code_timeout")
		default:
		}
		var msg map[string]any
		if err := dec.Decode(&msg); err != nil {
			if cmdCtx.Err() != nil || waitCtx.Err() != nil {
				return "", errors.New("codex_device_code_timeout")
			}
			return "", errors.New("codex_app_server_read_failed")
		}
		method := stringField(msg, "method")
		switch method {
		case "account/login/completed":
			params, _ := msg["params"].(map[string]any)
			msgLoginID := importStringField(params, "loginId", "login_id")
			if loginID != "" && msgLoginID != "" && msgLoginID != loginID {
				continue
			}
			if !boolFromAny(params["success"]) {
				errText := firstNonEmptyText(importStringField(params, "error"), "codex_device_code_login_failed")
				return "", errors.New(errText)
			}
			completed = true
		case "account/updated":
			params, _ := msg["params"].(map[string]any)
			authMode := importStringField(params, "authMode", "auth_mode")
			if authMode == "chatgpt" {
				updated = true
			}
		default:
			respondToCodexDeviceServerRequest(enc, msg)
		}
		if completed && updated {
			return readCodexDeviceAuthJSON(waitCtx, authDir)
		}
	}
}

func respondToCodexDeviceServerRequest(enc *json.Encoder, msg map[string]any) {
	if enc == nil || stringField(msg, "method") == "" {
		return
	}
	requestID, ok := msg["id"]
	if !ok {
		return
	}
	_ = enc.Encode(map[string]any{"id": requestID, "result": map[string]any{"decision": "deny", "reason": "interactive approval is unavailable during Codex+++ account-pool authorization"}})
}

func boolFromAny(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "true")
	default:
		return false
	}
}

func readCodexDeviceAuthJSON(ctx context.Context, authDir string) (string, error) {
	authPath := filepath.Join(authDir, "auth.json")
	deadline := time.Now().Add(5 * time.Second)
	for {
		raw, err := os.ReadFile(authPath)
		if err == nil && len(bytes.TrimSpace(raw)) > 0 {
			return string(raw), nil
		}
		if time.Now().After(deadline) {
			return "", errors.New("codex_device_code_auth_json_missing")
		}
		select {
		case <-ctx.Done():
			return "", errors.New("codex_device_code_timeout")
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func codexOAuthResultSummary(result upstreamImportResult) (string, string) {
	if len(result.APIKeys) == 0 {
		return "", ""
	}
	keyPreview, _ := result.APIKeys[0]["keyPreview"].(string)
	accountName := ""
	if upstream, ok := result.APIKeys[0]["upstream"].(map[string]any); ok {
		accountName, _ = upstream["name"].(string)
		if accountName == "" {
			accountName, _ = upstream["email"].(string)
		}
	}
	return keyPreview, accountName
}

func (a *App) codexOAuthSessionSnapshot(state string) (codexOAuthSession, bool) {
	now := time.Now().UTC()
	a.oauthMu.Lock()
	defer a.oauthMu.Unlock()
	a.ensureOAuthSessionsLocked()
	a.pruneCodexOAuthSessionsLocked(now)
	session, ok := a.oauthSessions[state]
	if !ok {
		return codexOAuthSession{}, false
	}
	return session, true
}

func (a *App) codexOAuthSessionInternalSnapshot(state string) (codexOAuthSession, bool) {
	a.oauthMu.Lock()
	defer a.oauthMu.Unlock()
	a.ensureOAuthSessionsLocked()
	session, ok := a.oauthSessions[state]
	return session, ok
}

func (a *App) updateCodexOAuthSession(state string, update func(*codexOAuthSession)) {
	a.oauthMu.Lock()
	defer a.oauthMu.Unlock()
	a.ensureOAuthSessionsLocked()
	session, ok := a.oauthSessions[state]
	if !ok {
		return
	}
	update(&session)
	a.oauthSessions[state] = session
}

func (a *App) ensureOAuthSessionsLocked() {
	if a.oauthSessions == nil {
		a.oauthSessions = map[string]codexOAuthSession{}
	}
	if a.oauthLogins == nil {
		a.oauthLogins = map[string]*codexDeviceCodeLogin{}
	}
}

func (a *App) pruneCodexOAuthSessionsLocked(now time.Time) {
	for state, session := range a.oauthSessions {
		if session.ExpiresAt.Add(5 * time.Minute).Before(now) {
			delete(a.oauthSessions, state)
			delete(a.oauthLogins, state)
		}
	}
}

func publicCodexOAuthSession(session codexOAuthSession) map[string]any {
	return map[string]any{
		"state":           session.State,
		"status":          session.Status,
		"error":           session.Error,
		"imported":        session.Imported,
		"updated":         session.Updated,
		"keyPreview":      session.KeyPreview,
		"accountName":     session.AccountName,
		"authMethod":      session.AuthMethod,
		"verificationUrl": session.VerificationURL,
		"authorizeUrl":    session.VerificationURL,
		"userCode":        session.UserCode,
		"loginId":         session.LoginID,
		"upstreamId":      session.UpstreamID,
		"expiresAt":       session.ExpiresAt,
	}
}

func (a *App) newUpstreamFromRequestLocked(req adminUpstreamRequest, now time.Time) (UpstreamAccount, error) {
	up := UpstreamAccount{ID: a.store.nextID("up"), CreatedAt: now}
	if err := a.applyUpstreamRequestLocked(&up, req, now); err != nil {
		return UpstreamAccount{}, err
	}
	return up, nil
}

func (a *App) applyUpstreamRequestLocked(up *UpstreamAccount, req adminUpstreamRequest, now time.Time) error {
	if req.Remark != "" {
		up.Remark = req.Remark
	}
	if strings.TrimSpace(req.AccessToken) == "" {
		if up.AuthorizationStatus == upstreamAuthAuthorized && strings.TrimSpace(up.AccessTokenCipher) != "" {
			return nil
		}
		passwordCipher, err := a.encrypt(req.Password)
		if err != nil {
			return err
		}
		up.Name = req.Name
		up.Group = req.Group
		up.CredentialType = "email_password"
		up.SourceType = valueOr(req.SourceType, "email_password")
		up.AuthorizationStatus = upstreamAuthPending
		up.PasswordCipher = passwordCipher
		up.LastAuthorizationError = ""
		up.Email = strings.ToLower(strings.TrimSpace(req.Email))
		up.EntitlementStatus = upstreamAuthPending
		up.Status = statusDisabled
		up.BalanceStatus = "unavailable"
		up.RiskStatus = "unavailable"
		up.CredentialFingerprint = a.fingerprint("email_password:" + up.Email)
		up.UpdatedAt = now
		return nil
	}
	wasAuthorized := up.AuthorizationStatus == upstreamAuthAuthorized && strings.TrimSpace(up.AccessTokenCipher) != ""
	previousSourceType := strings.TrimSpace(up.SourceType)
	accessCipher, err := a.encrypt(req.AccessToken)
	if err != nil {
		return err
	}
	refreshCipher, err := a.encryptOptional(req.RefreshToken)
	if err != nil {
		return err
	}
	authJSONCipher, err := a.encryptOptional(req.AuthJSONRaw)
	if err != nil {
		return err
	}
	entitlementStatus := req.EntitlementStatus
	balanceStatus := "available"
	if req.ExpiresAt != nil && !req.ExpiresAt.After(now) {
		balanceStatus = "unavailable"
		if entitlementStatus == "" || entitlementStatus == "short_lived_auth" {
			entitlementStatus = "token_expired"
		}
	}
	up.Name = req.Name
	up.Group = req.Group
	up.CredentialType = valueOr(req.CredentialType, "oauth")
	up.SourceType = valueOr(req.SourceType, "token_json")
	if !wasAuthorized && previousSourceType != "" {
		up.SourceType = previousSourceType
	}
	up.AuthorizationStatus = upstreamAuthAuthorized
	up.AccessTokenCipher = accessCipher
	up.RefreshTokenCipher = refreshCipher
	up.AuthJSONCipher = authJSONCipher
	up.PasswordCipher = ""
	up.LastAuthorizationError = ""
	up.TokenType = req.TokenType
	up.ChatGPTAccountID = req.ChatGPTAccountID
	up.ExpiresAt = req.ExpiresAt
	up.Email = req.Email
	up.SubscriptionTier = req.SubscriptionTier
	up.EntitlementStatus = entitlementStatus
	if !wasAuthorized || up.Status == "" {
		up.Status = statusActive
	}
	up.BalanceStatus = balanceStatus
	up.RiskStatus = "available"
	up.UsageTokens = 0
	up.RateLimitUsedPercent = nil
	up.RateLimitResetsAt = nil
	up.CreditBalance = nil
	up.CreditBalanceLabel = ""
	up.LastCheckedAt = nil
	up.CredentialFingerprint = a.fingerprint(req.AccessToken + req.RefreshToken + req.AuthJSONRaw)
	up.UpdatedAt = now
	return nil
}

func (a *App) upstreamDuplicateIndexLocked(req adminUpstreamRequest) int {
	if strings.TrimSpace(req.AccessToken) == "" && looksLikeEmail(req.Email) {
		for i, up := range a.store.state.UpstreamAccounts {
			if strings.EqualFold(strings.TrimSpace(up.Email), strings.TrimSpace(req.Email)) {
				return i
			}
		}
		return -1
	}
	accountID := strings.TrimSpace(req.ChatGPTAccountID)
	if accountID != "" {
		for i, up := range a.store.state.UpstreamAccounts {
			if strings.EqualFold(strings.TrimSpace(up.ChatGPTAccountID), accountID) {
				return i
			}
		}
	}
	fingerprint := a.fingerprint(req.AccessToken + req.RefreshToken + req.AuthJSONRaw)
	if fingerprint == "" {
		return -1
	}
	for i, up := range a.store.state.UpstreamAccounts {
		if up.CredentialFingerprint == fingerprint {
			return i
		}
	}
	if looksLikeEmail(req.Email) {
		for i, up := range a.store.state.UpstreamAccounts {
			if strings.EqualFold(strings.TrimSpace(up.Email), strings.TrimSpace(req.Email)) && (up.AuthorizationStatus != upstreamAuthAuthorized || strings.TrimSpace(up.AccessTokenCipher) == "") {
				return i
			}
		}
	}
	return -1
}

func (a *App) ensureAPIKeyForUpstreamLocked(upstreamID string, now time.Time) (APIKey, bool, error) {
	for i := range a.store.state.APIKeys {
		if a.store.state.APIKeys[i].UpstreamAccountID == upstreamID {
			if strings.TrimSpace(a.store.state.APIKeys[i].KeyCipher) == "" {
				raw, err := generateSub2APIKey()
				if err != nil {
					return APIKey{}, false, err
				}
				cipherText, err := a.encrypt(raw)
				if err != nil {
					return APIKey{}, false, err
				}
				a.store.state.APIKeys[i].KeyCipher = cipherText
				a.store.state.APIKeys[i].KeyHash = hashString(raw)
				a.store.state.APIKeys[i].PublicPrefix = raw[:10]
				a.store.state.APIKeys[i].UpdatedAt = now
			}
			return a.store.state.APIKeys[i], false, nil
		}
	}
	key, err := a.newAPIKeyForUpstreamLocked(upstreamID, now)
	if err != nil {
		return APIKey{}, false, err
	}
	a.store.state.APIKeys = append(a.store.state.APIKeys, key)
	return key, true, nil
}

func (a *App) adminUpstreamAction(w http.ResponseWriter, r *http.Request, rest string) {
	admin, ok := a.requireAdmin(w, r, false)
	if !ok {
		return
	}
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) == 1 && parts[0] != "" && r.Method == http.MethodDelete {
		if !readEmptyJSON(w, r, "invalid_upstream_delete_request") {
			return
		}
		a.adminDeleteUpstream(w, admin, parts[0])
		return
	}
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
	if len(parts) == 2 && parts[1] == "remark" && r.Method == http.MethodPost {
		a.adminUpdateUpstreamRemark(w, r, admin, parts[0])
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

func (a *App) adminDeleteUpstream(w http.ResponseWriter, admin Admin, id string) {
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	idx := a.upstreamIndex(id)
	if idx < 0 {
		writeErr(w, http.StatusNotFound, "upstream_not_found")
		return
	}
	name := a.store.state.UpstreamAccounts[idx].Name
	a.store.state.UpstreamAccounts = append(a.store.state.UpstreamAccounts[:idx], a.store.state.UpstreamAccounts[idx+1:]...)
	keys := a.store.state.APIKeys[:0]
	for _, key := range a.store.state.APIKeys {
		if key.UpstreamAccountID != id {
			keys = append(keys, key)
		}
	}
	a.store.state.APIKeys = keys
	a.auditLocked(admin.ID, "admin", "upstream.delete", id, name)
	if !a.saveOrErrorLocked(w) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) adminUpdateUpstreamRemark(w http.ResponseWriter, r *http.Request, admin Admin, id string) {
	var req struct {
		Remark *string `json:"remark"`
	}
	if !readStrictJSON(w, r, &req, "invalid_upstream_remark_request", "remark") {
		return
	}
	if req.Remark == nil {
		writeErr(w, http.StatusBadRequest, "invalid_upstream_remark_request")
		return
	}
	remark := strings.TrimSpace(*req.Remark)
	if len([]rune(remark)) > maxUpstreamRemarkRunes {
		writeErr(w, http.StatusBadRequest, "invalid_upstream_remark_request")
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
	up.Remark = remark
	up.UpdatedAt = time.Now().UTC()
	a.auditLocked(admin.ID, "admin", "upstream.remark", up.ID, fmt.Sprintf("remark_length=%d", len([]rune(remark))))
	if !a.saveOrErrorLocked(w) {
		return
	}
	writeJSON(w, http.StatusOK, a.publicAdminUpstreamLocked(*up))
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
	refreshCipher, err := a.encryptOptional(req.RefreshToken)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "secret_encrypt_failed")
		return
	}
	authJSONCipher, err := a.encryptOptional(req.AuthJSONRaw)
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
	up.AuthJSONCipher = authJSONCipher
	up.TokenType = req.TokenType
	up.ChatGPTAccountID = req.ChatGPTAccountID
	up.ExpiresAt = req.ExpiresAt
	up.Email = req.Email
	up.SubscriptionTier = req.SubscriptionTier
	up.EntitlementStatus = req.EntitlementStatus
	up.BalanceStatus = "available"
	if req.ExpiresAt != nil && !req.ExpiresAt.After(time.Now().UTC()) {
		up.BalanceStatus = "unavailable"
		up.EntitlementStatus = "token_expired"
	}
	up.RiskStatus = "available"
	up.UsageTokens = 0
	up.RateLimitUsedPercent = nil
	up.RateLimitResetsAt = nil
	up.CreditBalance = nil
	up.CreditBalanceLabel = ""
	up.LastCheckedAt = nil
	up.CredentialFingerprint = a.fingerprint(req.AccessToken + req.RefreshToken + req.AuthJSONRaw)
	up.UpdatedAt = time.Now().UTC()
	a.auditLocked(admin.ID, "admin", "upstream.credentials.replace", up.ID, "credentials_replaced")
	if !a.saveOrErrorLocked(w) {
		return
	}
	writeJSON(w, http.StatusOK, publicUpstream(*up))
}

func (a *App) credentialsForUpstream(ctx context.Context, up UpstreamAccount) (codexProbeCredentials, *adminUpstreamRequest, error) {
	accessToken, err := a.decrypt(up.AccessTokenCipher)
	if err != nil {
		return codexProbeCredentials{}, nil, err
	}
	credentials := codexProbeCredentials{
		AccessToken:      accessToken,
		ChatGPTAccountID: strings.TrimSpace(up.ChatGPTAccountID),
		ChatGPTPlanType:  up.SubscriptionTier,
	}
	if credentials.ChatGPTAccountID == "" {
		credentials.ChatGPTAccountID = chatGPTAccountIDFromAccessToken(accessToken)
	}
	refreshToken, err := a.decrypt(up.RefreshTokenCipher)
	if err != nil {
		return credentials, nil, err
	}
	if !accessTokenRefreshDue(accessToken, up.ExpiresAt) || strings.TrimSpace(refreshToken) == "" || strings.TrimSpace(up.AuthJSONCipher) == "" {
		return credentials, nil, nil
	}
	authJSONRaw, err := a.decrypt(up.AuthJSONCipher)
	if err != nil {
		return credentials, nil, err
	}
	refreshed, err := refreshCodexAuthJSON(ctx, authJSONRaw)
	if err != nil {
		return credentials, nil, nil
	}
	if refreshed.Group == "" {
		refreshed.Group = up.Group
	}
	if refreshed.Name == "" {
		refreshed.Name = up.Name
	}
	if refreshed.Name == "" {
		refreshed.Name = firstNonEmptyText(up.Email, up.ChatGPTAccountID, up.ID)
	}
	if refreshed.SubscriptionTier == "" {
		refreshed.SubscriptionTier = up.SubscriptionTier
	}
	credentials.AccessToken = refreshed.AccessToken
	credentials.ChatGPTAccountID = firstNonEmptyText(refreshed.ChatGPTAccountID, credentials.ChatGPTAccountID)
	credentials.ChatGPTPlanType = firstNonEmptyText(refreshed.SubscriptionTier, credentials.ChatGPTPlanType)
	return credentials, &refreshed, nil
}

func accessTokenRefreshDue(accessToken string, expiresAt *time.Time) bool {
	deadline := expiresAt
	if deadline == nil {
		deadline = jwtTimeClaim(jwtClaimsFromToken(accessToken), "exp")
	}
	if deadline == nil {
		return false
	}
	return time.Until(*deadline) <= upstreamAccessTokenRefreshWindow
}

func (a *App) applyRefreshedUpstreamLocked(up *UpstreamAccount, req adminUpstreamRequest, now time.Time) error {
	accessCipher, err := a.encrypt(req.AccessToken)
	if err != nil {
		return err
	}
	refreshCipher, err := a.encryptOptional(req.RefreshToken)
	if err != nil {
		return err
	}
	authJSONCipher, err := a.encryptOptional(req.AuthJSONRaw)
	if err != nil {
		return err
	}
	up.AccessTokenCipher = accessCipher
	up.RefreshTokenCipher = refreshCipher
	up.AuthJSONCipher = authJSONCipher
	up.TokenType = req.TokenType
	up.ChatGPTAccountID = req.ChatGPTAccountID
	up.ExpiresAt = req.ExpiresAt
	if req.Email != "" {
		up.Email = req.Email
	}
	if req.SubscriptionTier != "" {
		up.SubscriptionTier = req.SubscriptionTier
	}
	up.EntitlementStatus = req.EntitlementStatus
	if up.EntitlementStatus == "" && req.RefreshToken == "" {
		up.EntitlementStatus = "short_lived_auth"
	}
	up.BalanceStatus = "available"
	if req.ExpiresAt != nil && !req.ExpiresAt.After(now) {
		up.BalanceStatus = "unavailable"
		up.EntitlementStatus = "token_expired"
	}
	up.RiskStatus = "available"
	up.CredentialFingerprint = a.fingerprint(req.AccessToken + req.RefreshToken + req.AuthJSONRaw)
	up.UpdatedAt = now
	return nil
}

type upstreamCheckError struct {
	Status int
	Code   string
}

func (e *upstreamCheckError) Error() string { return e.Code }

func (a *App) setUpstreamCheckInProgress(id string, checking bool) {
	a.upstreamCheckStateMu.Lock()
	defer a.upstreamCheckStateMu.Unlock()
	if checking {
		if a.upstreamChecking == nil {
			a.upstreamChecking = map[string]struct{}{}
		}
		a.upstreamChecking[id] = struct{}{}
		return
	}
	delete(a.upstreamChecking, id)
}

func (a *App) upstreamCheckInProgress(id string) bool {
	a.upstreamCheckStateMu.Lock()
	defer a.upstreamCheckStateMu.Unlock()
	_, ok := a.upstreamChecking[id]
	return ok
}

func upstreamStoredCheckState(up UpstreamAccount) (string, string) {
	if up.LastCheckedAt == nil {
		return "not_checked", ""
	}
	switch up.EntitlementStatus {
	case "check_failed", "missing_access_token", "missing_chatgpt_account_id", "auth_failed", "usage_limit", "rate_limited", "service_overloaded", "check_service_start_failed", "check_service_unavailable", "check_timed_out", "secret_decrypt_failed", "token_expired":
		return "failed", up.EntitlementStatus
	default:
		return "completed", ""
	}
}

func (a *App) adminCheckUpstream(w http.ResponseWriter, r *http.Request, admin Admin, id string) {
	result, err := a.checkUpstream(r.Context(), id, admin.ID, "admin", "upstream.check")
	if err != nil {
		writeErr(w, err.Status, err.Code)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *App) checkUpstream(parent context.Context, id, actorID, actorRole, action string) (map[string]any, *upstreamCheckError) {
	a.upstreamCheckMu.Lock()
	defer a.upstreamCheckMu.Unlock()
	a.setUpstreamCheckInProgress(id, true)
	defer a.setUpstreamCheckInProgress(id, false)

	ctx, cancel := context.WithTimeout(parent, 45*time.Second)
	defer cancel()
	a.store.mu.Lock()
	up := a.upstreamByID(id)
	a.store.mu.Unlock()
	if up.ID == "" {
		return nil, &upstreamCheckError{Status: http.StatusNotFound, Code: "upstream_not_found"}
	}
	credentials, refreshedReq, err := a.credentialsForUpstream(ctx, up)
	if err != nil {
		return a.finishUpstreamCheckWithoutProbe(id, actorID, actorRole, action, "secret_decrypt_failed")
	}
	if strings.TrimSpace(credentials.AccessToken) == "" {
		return a.finishUpstreamCheckWithoutProbe(id, actorID, actorRole, action, "missing_access_token")
	}
	if credentials.ChatGPTAccountID == "" {
		return a.finishUpstreamCheckWithoutProbe(id, actorID, actorRole, action, "missing_chatgpt_account_id")
	}
	result, probeErr := codexAppServerProbe(ctx, credentials)

	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	idx := a.upstreamIndex(id)
	if idx < 0 {
		return nil, &upstreamCheckError{Status: http.StatusNotFound, Code: "upstream_not_found"}
	}
	now := time.Now().UTC()
	target := &a.store.state.UpstreamAccounts[idx]
	if refreshedReq != nil {
		if err := a.applyRefreshedUpstreamLocked(target, *refreshedReq, now); err != nil {
			return nil, &upstreamCheckError{Status: http.StatusInternalServerError, Code: "secret_encrypt_failed"}
		}
	}
	target.ChatGPTAccountID = credentials.ChatGPTAccountID
	target.LastCheckedAt = &now
	target.UpdatedAt = now
	if probeErr != nil {
		target.BalanceStatus = "unavailable"
		target.RiskStatus = "unavailable"
		target.EntitlementStatus = upstreamProbeFailureStatus(probeErr)
		clearUpstreamCheckMetrics(target)
		a.auditLocked(actorID, actorRole, action, id, target.EntitlementStatus)
		if err := a.store.save(); err != nil {
			return nil, &upstreamCheckError{Status: http.StatusInternalServerError, Code: "state_save_failed"}
		}
		return publicUpstream(*target), nil
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
	a.auditLocked(actorID, actorRole, action, id, detail)
	if err := a.store.save(); err != nil {
		return nil, &upstreamCheckError{Status: http.StatusInternalServerError, Code: "state_save_failed"}
	}
	resp := publicUpstream(*target)
	resp["usageTokens"] = result.UsageTokens
	resp["accountType"] = result.AccountType
	return resp, nil
}

func (a *App) finishUpstreamCheckWithoutProbe(id, actorID, actorRole, action, reason string) (map[string]any, *upstreamCheckError) {
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	idx := a.upstreamIndex(id)
	if idx < 0 {
		return nil, &upstreamCheckError{Status: http.StatusNotFound, Code: "upstream_not_found"}
	}
	now := time.Now().UTC()
	target := &a.store.state.UpstreamAccounts[idx]
	target.BalanceStatus = "unavailable"
	target.RiskStatus = "unavailable"
	target.EntitlementStatus = reason
	clearUpstreamCheckMetrics(target)
	target.LastCheckedAt = &now
	target.UpdatedAt = now
	a.auditLocked(actorID, actorRole, action, id, reason)
	if err := a.store.save(); err != nil {
		return nil, &upstreamCheckError{Status: http.StatusInternalServerError, Code: "state_save_failed"}
	}
	return publicUpstream(*target), nil
}

func clearUpstreamCheckMetrics(upstream *UpstreamAccount) {
	upstream.UsageTokens = 0
	upstream.RateLimitUsedPercent = nil
	upstream.RateLimitResetsAt = nil
	upstream.CreditBalance = nil
	upstream.CreditBalanceLabel = ""
}

var upstreamCheckLocation = time.FixedZone("Asia/Shanghai", 8*60*60)

func upstreamDailyCheckBoundary(now time.Time) time.Time {
	local := now.In(upstreamCheckLocation)
	return time.Date(local.Year(), local.Month(), local.Day(), 9, 0, 0, 0, upstreamCheckLocation)
}

func nextUpstreamDailyCheck(now time.Time) time.Time {
	next := upstreamDailyCheckBoundary(now)
	if !now.Before(next) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}

func (a *App) startDailyUpstreamCheckScheduler(ctx context.Context) {
	go func() {
		now := time.Now()
		due := upstreamDailyCheckBoundary(now)
		if !now.Before(due) {
			a.runScheduledUpstreamChecks(ctx, due, true)
		}
		for {
			next := nextUpstreamDailyCheck(time.Now())
			timer := time.NewTimer(time.Until(next))
			select {
			case <-ctx.Done():
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				return
			case <-timer.C:
				a.runScheduledUpstreamChecks(ctx, next, false)
			}
		}
	}()
}

func (a *App) scheduledUpstreamCheckIDs(due time.Time, onlyUncheckedSinceDue bool) []string {
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	ids := make([]string, 0, len(a.store.state.UpstreamAccounts))
	for _, upstream := range a.store.state.UpstreamAccounts {
		if upstream.AuthorizationStatus != upstreamAuthAuthorized {
			continue
		}
		if onlyUncheckedSinceDue && upstream.LastCheckedAt != nil && !upstream.LastCheckedAt.Before(due) {
			continue
		}
		ids = append(ids, upstream.ID)
	}
	return ids
}

func (a *App) runScheduledUpstreamChecks(ctx context.Context, due time.Time, onlyUncheckedSinceDue bool) {
	lockKey := "upstream-check:" + due.In(upstreamCheckLocation).Format("2006-01-02")
	lockOwner := a.gatewayInstanceID
	if lockOwner == "" {
		lockOwner = randomToken(12)
	}
	if a.gatewayRuntime != nil {
		acquired, err := a.gatewayRuntime.AcquireLock(ctx, lockKey, lockOwner, 2*time.Hour)
		if err != nil {
			log.Printf("scheduled upstream authorization checks skipped: runtime lock unavailable")
			return
		}
		if !acquired {
			return
		}
		defer func() {
			releaseCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_ = a.gatewayRuntime.ReleaseLock(releaseCtx, lockKey, lockOwner)
		}()
	}
	defer a.runScheduledOperationalCleanup()
	ids := a.scheduledUpstreamCheckIDs(due, onlyUncheckedSinceDue)
	if len(ids) == 0 {
		return
	}
	succeeded := 0
	for _, id := range ids {
		if ctx.Err() != nil {
			break
		}
		if _, err := a.checkUpstream(ctx, id, "system", "system", "upstream.check.scheduled"); err != nil {
			log.Printf("scheduled upstream authorization check failed: upstream=%s error=%s", id, err.Code)
			continue
		}
		succeeded++
	}
	log.Printf("scheduled upstream authorization checks completed: due=%s attempted=%d succeeded=%d", due.Format(time.RFC3339), len(ids), succeeded)
}

func (a *App) startOperationalCleanupScheduler(ctx context.Context) {
	go func() {
		a.runScheduledOperationalCleanup()
		ticker := time.NewTicker(operationalCleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				a.runScheduledOperationalCleanup()
			}
		}
	}()
}

func (a *App) runScheduledOperationalCleanup() {
	now := time.Now().UTC()
	a.store.mu.Lock()
	if a.pruneOperationalHistoryLocked(now) {
		if err := a.store.save(); err != nil {
			log.Printf("scheduled operational history cleanup failed: %v", err)
		}
	}
	a.store.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	clearedBodies, deletedRecords, err := a.store.pruneGatewayPersistence(ctx, now)
	if err != nil {
		log.Printf("scheduled gateway persistence cleanup failed: %v", err)
		return
	}
	if clearedBodies > 0 || deletedRecords > 0 {
		log.Printf("scheduled gateway persistence cleanup completed: cleared_replay_bodies=%d deleted_idempotency_records=%d", clearedBodies, deletedRecords)
	}
}

func (a *App) adminAPIKeys(w http.ResponseWriter, r *http.Request) {
	_, ok := a.requireAdmin(w, r, false)
	if !ok {
		return
	}
	available := r.URL.Query().Get("available")
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	items := make([]map[string]any, 0, len(a.store.state.APIKeys))
	for _, key := range a.store.state.APIKeys {
		item := a.publicAPIKeyLocked(key)
		if available == "true" && item["routeAvailable"] != true {
			continue
		}
		items = append(items, item)
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
	upstreamIndex := a.upstreamIndex(req.UpstreamAccountID)
	if upstreamIndex < 0 {
		writeErr(w, http.StatusNotFound, "upstream_not_found")
		return
	}
	if up := a.store.state.UpstreamAccounts[upstreamIndex]; up.AuthorizationStatus != upstreamAuthAuthorized || strings.TrimSpace(up.AccessTokenCipher) == "" {
		writeErr(w, http.StatusConflict, "upstream_not_authorized")
		return
	}
	now := time.Now().UTC()
	key, created, err := a.ensureAPIKeyForUpstreamLocked(req.UpstreamAccountID, now)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "key_generate_failed")
		return
	}
	if created {
		a.auditLocked(admin.ID, "admin", "api_key.create", key.ID, req.UpstreamAccountID)
	}
	if !a.saveOrErrorLocked(w) {
		return
	}
	writeJSON(w, http.StatusCreated, a.publicAPIKeyLocked(key))
}

func (a *App) newAPIKeyForUpstreamLocked(upstreamAccountID string, now time.Time) (APIKey, error) {
	raw, err := generateSub2APIKey()
	if err != nil {
		return APIKey{}, err
	}
	cipherText, err := a.encrypt(raw)
	if err != nil {
		return APIKey{}, err
	}
	return APIKey{ID: a.store.nextID("key"), KeyCipher: cipherText, KeyHash: hashString(raw), PublicPrefix: raw[:10], UpstreamAccountID: upstreamAccountID, Status: statusActive, CreatedAt: now, UpdatedAt: now}, nil
}

func (a *App) adminAPIKeyAction(w http.ResponseWriter, r *http.Request, rest string) {
	admin, ok := a.requireAdmin(w, r, false)
	if !ok {
		return
	}
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) == 1 && parts[0] != "" && r.Method == http.MethodDelete {
		a.adminDeleteAPIKey(w, r, admin, parts[0])
		return
	}
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

func (a *App) adminDeleteAPIKey(w http.ResponseWriter, r *http.Request, admin Admin, id string) {
	if !readEmptyJSON(w, r, "invalid_api_key_delete_request") {
		return
	}
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	idx := a.apiKeyIndex(id)
	if idx < 0 {
		writeErr(w, http.StatusNotFound, "api_key_not_found")
		return
	}
	upstreamID := a.store.state.APIKeys[idx].UpstreamAccountID
	a.store.state.APIKeys = append(a.store.state.APIKeys[:idx], a.store.state.APIKeys[idx+1:]...)
	a.auditLocked(admin.ID, "admin", "api_key.delete", id, upstreamID)
	if upstreamID != "" && !a.apiKeyReferencesUpstreamLocked(upstreamID) {
		if upIdx := a.upstreamIndex(upstreamID); upIdx >= 0 {
			name := a.store.state.UpstreamAccounts[upIdx].Name
			a.store.state.UpstreamAccounts = append(a.store.state.UpstreamAccounts[:upIdx], a.store.state.UpstreamAccounts[upIdx+1:]...)
			a.auditLocked(admin.ID, "admin", "upstream.delete", upstreamID, name)
		}
	}
	if !a.saveOrErrorLocked(w) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
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

func (a *App) adminUsageAnalytics(w http.ResponseWriter, r *http.Request) {
	_, ok := a.requireAdmin(w, r, false)
	if !ok {
		return
	}
	filter, err := parseUsageAnalyticsFilter(r, time.Now().UTC())
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	payload, err := a.usageAnalyticsLocked(filter)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

func parseUsageAnalyticsFilter(r *http.Request, now time.Time) (usageAnalyticsFilter, error) {
	query := r.URL.Query()
	timezone := strings.TrimSpace(query.Get("timezone"))
	if timezone == "" {
		timezone = "Asia/Shanghai"
	}
	location, ok := usageAnalyticsLocation(timezone)
	if !ok {
		return usageAnalyticsFilter{}, errors.New("invalid_usage_timezone")
	}
	groupBy := strings.TrimSpace(query.Get("groupBy"))
	if groupBy == "" {
		groupBy = "day"
	}
	if groupBy != "day" && groupBy != "week" && groupBy != "month" {
		return usageAnalyticsFilter{}, errors.New("invalid_usage_group")
	}
	metric := strings.TrimSpace(query.Get("metric"))
	if metric == "" {
		metric = "averageTokensPerTask"
	}
	if metric != "averageTokensPerTask" && metric != "taskCount" && metric != "totalTokens" {
		return usageAnalyticsFilter{}, errors.New("invalid_usage_metric")
	}
	today := now.In(location)
	defaultTo := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, location)
	defaultFrom := defaultTo.AddDate(0, 0, -29)
	fromLocal, fromDate, err := parseUsageAnalyticsDate(query.Get("from"), defaultFrom, location)
	if err != nil {
		return usageAnalyticsFilter{}, errors.New("invalid_usage_date")
	}
	toLocal, toDate, err := parseUsageAnalyticsDate(query.Get("to"), defaultTo, location)
	if err != nil {
		return usageAnalyticsFilter{}, errors.New("invalid_usage_date")
	}
	if toLocal.Before(fromLocal) || toLocal.Sub(fromLocal) > 365*24*time.Hour {
		return usageAnalyticsFilter{}, errors.New("invalid_usage_date_range")
	}
	accountIDs := splitUsageAnalyticsAccountIDs(query.Get("accountIds"))
	if len(accountIDs) > 20 {
		return usageAnalyticsFilter{}, errors.New("too_many_usage_accounts")
	}
	minTasks, err := optionalNonNegativeQueryInt64(query.Get("minTasks"))
	if err != nil {
		return usageAnalyticsFilter{}, errors.New("invalid_usage_task_range")
	}
	maxTasks, err := optionalNonNegativeQueryInt64(query.Get("maxTasks"))
	if err != nil || minTasks != nil && maxTasks != nil && *minTasks > *maxTasks {
		return usageAnalyticsFilter{}, errors.New("invalid_usage_task_range")
	}
	minTokens, err := optionalNonNegativeQueryInt64(query.Get("minTokens"))
	if err != nil {
		return usageAnalyticsFilter{}, errors.New("invalid_usage_token_range")
	}
	maxTokens, err := optionalNonNegativeQueryInt64(query.Get("maxTokens"))
	if err != nil || minTokens != nil && maxTokens != nil && *minTokens > *maxTokens {
		return usageAnalyticsFilter{}, errors.New("invalid_usage_token_range")
	}
	return usageAnalyticsFilter{
		AccountIDs: accountIDs,
		From:       fromLocal.UTC(),
		To:         toLocal.AddDate(0, 0, 1).UTC(),
		FromDate:   fromDate,
		ToDate:     toDate,
		GroupBy:    groupBy,
		Metric:     metric,
		Timezone:   timezone,
		MinTasks:   minTasks,
		MaxTasks:   maxTasks,
		MinTokens:  minTokens,
		MaxTokens:  maxTokens,
	}, nil
}

func usageAnalyticsLocation(name string) (*time.Location, bool) {
	switch name {
	case "Asia/Shanghai":
		return time.FixedZone("Asia/Shanghai", 8*60*60), true
	case "UTC":
		return time.UTC, true
	default:
		return nil, false
	}
}

func parseUsageAnalyticsDate(raw string, fallback time.Time, location *time.Location) (time.Time, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, fallback.Format("2006-01-02"), nil
	}
	parsed, err := time.ParseInLocation("2006-01-02", raw, location)
	if err != nil {
		return time.Time{}, "", err
	}
	return parsed, parsed.Format("2006-01-02"), nil
}

func optionalNonNegativeQueryInt64(raw string) (*int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 0 {
		return nil, errors.New("invalid_non_negative_integer")
	}
	return &value, nil
}

func splitUsageAnalyticsAccountIDs(raw string) []string {
	seen := map[string]struct{}{}
	items := make([]string, 0)
	for _, value := range strings.Split(raw, ",") {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		items = append(items, value)
	}
	return items
}

func (a *App) usageAnalyticsLocked(filter usageAnalyticsFilter) (map[string]any, error) {
	location, ok := usageAnalyticsLocation(filter.Timezone)
	if !ok {
		return nil, errors.New("invalid_usage_timezone")
	}
	accountLabels := map[string]string{}
	accountRows := make([]map[string]any, 0, len(a.store.state.UpstreamAccounts))
	for _, upstream := range a.store.state.UpstreamAccounts {
		label := usageAnalyticsAccountLabel(upstream)
		accountLabels[upstream.ID] = label
		accountRows = append(accountRows, map[string]any{
			"id":    upstream.ID,
			"label": label,
			"email": upstream.Email,
			"name":  upstream.Name,
		})
	}
	for _, record := range a.store.state.UsageRecords {
		if record.UpstreamAccountID != "" {
			if _, exists := accountLabels[record.UpstreamAccountID]; !exists {
				accountLabels[record.UpstreamAccountID] = record.UpstreamAccountID
				accountRows = append(accountRows, map[string]any{"id": record.UpstreamAccountID, "label": record.UpstreamAccountID})
			}
		}
	}
	sort.Slice(accountRows, func(i, j int) bool {
		return stringField(accountRows[i], "label") < stringField(accountRows[j], "label")
	})
	selectedIDs := append([]string(nil), filter.AccountIDs...)
	if len(selectedIDs) == 0 {
		latest := UsageRecord{}
		for _, record := range a.store.state.UsageRecords {
			if record.UpstreamAccountID != "" && (latest.ID == "" || record.CreatedAt.After(latest.CreatedAt)) {
				latest = record
			}
		}
		if latest.UpstreamAccountID != "" {
			selectedIDs = []string{latest.UpstreamAccountID}
		} else if len(accountRows) > 0 {
			selectedIDs = []string{stringField(accountRows[0], "id")}
		}
	}
	selected := map[string]struct{}{}
	for _, accountID := range selectedIDs {
		if _, exists := accountLabels[accountID]; !exists {
			return nil, errors.New("invalid_usage_account")
		}
		selected[accountID] = struct{}{}
	}
	accumulators := map[string]*usageAnalyticsAccumulator{}
	bucketStarts := make([]time.Time, 0)
	firstBucket := usageAnalyticsBucketStart(filter.From, filter.GroupBy, location)
	for cursor := firstBucket; cursor.Before(filter.To.In(location)); cursor = usageAnalyticsNextBucket(cursor, filter.GroupBy) {
		bucketStarts = append(bucketStarts, cursor)
	}
	if len(bucketStarts)*len(selectedIDs) > 5000 {
		return nil, errors.New("usage_analytics_result_too_large")
	}
	for _, bucketStart := range bucketStarts {
		for _, accountID := range selectedIDs {
			key := usageAnalyticsAccumulatorKey(accountID, bucketStart)
			accumulators[key] = &usageAnalyticsAccumulator{AccountID: accountID, BucketStart: bucketStart, Tasks: map[string]struct{}{}}
		}
	}
	for _, record := range a.store.state.UsageRecords {
		if record.CreatedAt.Before(filter.From) || !record.CreatedAt.Before(filter.To) {
			continue
		}
		if _, exists := selected[record.UpstreamAccountID]; !exists {
			continue
		}
		bucketStart := usageAnalyticsBucketStart(record.CreatedAt, filter.GroupBy, location)
		key := usageAnalyticsAccumulatorKey(record.UpstreamAccountID, bucketStart)
		accumulator := accumulators[key]
		if accumulator == nil {
			continue
		}
		accumulator.InputTokens += record.InputTokens
		accumulator.CachedInputTokens += record.CachedInputTokens
		accumulator.OutputTokens += record.OutputTokens
		accumulator.TotalTokens += record.TotalTokens
		accumulator.RecordCount++
		taskKey := "session:" + record.UserID + ":" + strings.TrimSpace(record.SessionID)
		if strings.TrimSpace(record.SessionID) == "" {
			taskKey = "legacy:" + record.ID
			accumulator.FallbackRecords++
		}
		accumulator.Tasks[taskKey] = struct{}{}
	}
	points := make([]map[string]any, 0, len(accumulators))
	var summaryTasks, summaryInput, summaryCached, summaryOutput, summaryTokens int64
	var summaryRecords, summaryFallback int64
	for _, bucketStart := range bucketStarts {
		for _, accountID := range selectedIDs {
			accumulator := accumulators[usageAnalyticsAccumulatorKey(accountID, bucketStart)]
			if accumulator == nil {
				continue
			}
			taskCount := int64(len(accumulator.Tasks))
			if !usageAnalyticsRangeMatches(taskCount, filter.MinTasks, filter.MaxTasks) || !usageAnalyticsRangeMatches(accumulator.TotalTokens, filter.MinTokens, filter.MaxTokens) {
				continue
			}
			average := float64(0)
			if taskCount > 0 {
				average = float64(accumulator.TotalTokens) / float64(taskCount)
			}
			points = append(points, map[string]any{
				"bucket":               usageAnalyticsBucketLabel(bucketStart, filter.GroupBy),
				"bucketStart":          bucketStart.UTC(),
				"accountId":            accountID,
				"accountLabel":         accountLabels[accountID],
				"taskCount":            taskCount,
				"recordCount":          accumulator.RecordCount,
				"inputTokens":          accumulator.InputTokens,
				"cachedInputTokens":    accumulator.CachedInputTokens,
				"outputTokens":         accumulator.OutputTokens,
				"totalTokens":          accumulator.TotalTokens,
				"averageTokensPerTask": average,
				"historicalFallback":   accumulator.FallbackRecords,
			})
			summaryTasks += taskCount
			summaryInput += accumulator.InputTokens
			summaryCached += accumulator.CachedInputTokens
			summaryOutput += accumulator.OutputTokens
			summaryTokens += accumulator.TotalTokens
			summaryRecords += accumulator.RecordCount
			summaryFallback += accumulator.FallbackRecords
		}
	}
	summaryAverage := float64(0)
	if summaryTasks > 0 {
		summaryAverage = float64(summaryTokens) / float64(summaryTasks)
	}
	return map[string]any{
		"accounts":           accountRows,
		"selectedAccountIds": selectedIDs,
		"query": map[string]any{
			"from":      filter.FromDate,
			"to":        filter.ToDate,
			"groupBy":   filter.GroupBy,
			"metric":    filter.Metric,
			"timezone":  filter.Timezone,
			"minTasks":  filter.MinTasks,
			"maxTasks":  filter.MaxTasks,
			"minTokens": filter.MinTokens,
			"maxTokens": filter.MaxTokens,
		},
		"summary": map[string]any{
			"taskCount":            summaryTasks,
			"recordCount":          summaryRecords,
			"inputTokens":          summaryInput,
			"cachedInputTokens":    summaryCached,
			"outputTokens":         summaryOutput,
			"totalTokens":          summaryTokens,
			"averageTokensPerTask": summaryAverage,
			"matchedBuckets":       len(points),
		},
		"dataQuality": map[string]any{
			"sessionTrackedRecords": summaryRecords - summaryFallback,
			"historicalFallback":    summaryFallback,
			"taskDefinition":        "同一账号、普通用户和 Codex 会话在同一时间桶内计为一个任务；缺少会话标识的历史记录每条计为一个任务",
		},
		"points":      points,
		"generatedAt": time.Now().UTC(),
		"source":      "usage_records",
	}, nil
}

func usageAnalyticsAccountLabel(upstream UpstreamAccount) string {
	return firstNonEmptyText(strings.TrimSpace(upstream.Email), strings.TrimSpace(upstream.Name), strings.TrimSpace(upstream.ChatGPTAccountID), upstream.ID)
}

func usageAnalyticsBucketStart(value time.Time, groupBy string, location *time.Location) time.Time {
	local := value.In(location)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
	switch groupBy {
	case "week":
		daysSinceMonday := (int(start.Weekday()) + 6) % 7
		return start.AddDate(0, 0, -daysSinceMonday)
	case "month":
		return time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, location)
	default:
		return start
	}
}

func usageAnalyticsNextBucket(value time.Time, groupBy string) time.Time {
	switch groupBy {
	case "week":
		return value.AddDate(0, 0, 7)
	case "month":
		return value.AddDate(0, 1, 0)
	default:
		return value.AddDate(0, 0, 1)
	}
}

func usageAnalyticsBucketLabel(value time.Time, groupBy string) string {
	if groupBy == "month" {
		return value.Format("2006-01")
	}
	return value.Format("2006-01-02")
}

func usageAnalyticsAccumulatorKey(accountID string, bucketStart time.Time) string {
	return accountID + "\x00" + bucketStart.Format(time.RFC3339)
}

func usageAnalyticsRangeMatches(value int64, minimum, maximum *int64) bool {
	if minimum != nil && value < *minimum {
		return false
	}
	if maximum != nil && value > *maximum {
		return false
	}
	return true
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
	a.markClientPresenceLocked(userID, devID, now)
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
	platform := desktopPlatform(r.URL.Query().Get("platform"))
	writeJSON(w, http.StatusOK, desktopUpdatePayloadForPlatform(current, platform, time.Now().UTC()))
}

func desktopUpdatePayload(current string, checkedAt time.Time) map[string]any {
	return desktopUpdatePayloadForPlatform(current, "windows", checkedAt)
}

func desktopUpdatePayloadForPlatform(current, platform string, checkedAt time.Time) map[string]any {
	current = strings.TrimSpace(current)
	if current == "" {
		current = "unknown"
	}
	platform = desktopPlatform(platform)
	prefix := "CODEXPPP_DESKTOP_"
	if platform == "macos" {
		prefix = "CODEXPPP_MACOS_DESKTOP_"
	}
	latest := strings.TrimSpace(os.Getenv(prefix + "LATEST_VERSION"))
	if latest == "" {
		latest = current
	}
	available := versionGreater(latest, current)
	downloadURL := ""
	if available {
		downloadURL = safeHTTPURL(os.Getenv(prefix + "DOWNLOAD_URL"))
	}
	sha256Value := ""
	if available {
		sha256Value = strings.TrimSpace(os.Getenv(prefix + "DOWNLOAD_SHA256"))
	}
	notes := ""
	if available {
		notes = strings.TrimSpace(os.Getenv(prefix + "RELEASE_NOTES"))
	}
	return map[string]any{
		"platform":       platform,
		"currentVersion": current,
		"latestVersion":  latest,
		"available":      available,
		"downloadUrl":    downloadURL,
		"sha256":         sha256Value,
		"releaseNotes":   notes,
		"checkedAt":      checkedAt,
	}
}

func desktopPlatform(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), "macos") {
		return "macos"
	}
	return "windows"
}

func (a *App) clientLaunchPrepare(w http.ResponseWriter, r *http.Request) {
	user, device, ok := a.requireClient(w, r)
	if !ok {
		return
	}
	var req struct {
		LocalAccount   string `json:"localAccount"`
		LocalAuthMode  string `json:"localAuthMode"`
		ManagedRuntime bool   `json:"managedRuntime"`
	}
	if !readStrictJSON(w, r, &req, "invalid_launch_prepare_request", "localAccount", "localAuthMode", "managedRuntime") {
		return
	}
	a.store.mu.Lock()
	localAuthMode := strings.ToLower(strings.TrimSpace(req.LocalAuthMode))
	if localAuthMode != "" && (localAuthMode != "chatgpt" || !a.localCodexAccountBelongsToPoolLocked(req.LocalAccount)) {
		a.store.mu.Unlock()
		writeErr(w, http.StatusConflict, "personal_codex_login_detected")
		return
	}
	if a.availableTokenBalanceLocked(user.ID) <= 0 {
		a.store.mu.Unlock()
		writeErr(w, http.StatusPaymentRequired, "token_not_available")
		return
	}
	now := time.Now().UTC()
	providerToken, err := a.clientLaunchAccessKeyLocked(user.ID, device.ID, req.ManagedRuntime, now)
	if errors.Is(err, errRouteUnavailable) {
		a.store.mu.Unlock()
		writeErr(w, http.StatusServiceUnavailable, "route_unavailable")
		return
	}
	if err != nil {
		a.store.mu.Unlock()
		if err.Error() == "client_update_required" {
			writeErr(w, http.StatusUpgradeRequired, "client_update_required")
		} else {
			writeErr(w, http.StatusInternalServerError, "secret_decrypt_failed")
		}
		return
	}
	if !a.saveOrErrorLocked(w) {
		a.store.mu.Unlock()
		return
	}
	if req.ManagedRuntime {
		current := a.clientRuntimes[user.ID][device.ID]
		current.ExpiresAt = now.Add(gatewayClientLeaseTTL)
		a.setClientRuntimeLocked(user.ID, device.ID, current)
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

func (a *App) localCodexAccountBelongsToPoolLocked(account string) bool {
	wanted := strings.ToLower(strings.TrimSpace(account))
	if wanted == "" {
		return false
	}
	for _, upstream := range a.store.state.UpstreamAccounts {
		for _, candidate := range []string{upstream.ChatGPTAccountID, upstream.Email} {
			if strings.ToLower(strings.TrimSpace(candidate)) == wanted {
				return true
			}
		}
	}
	return false
}

func (a *App) clientLaunchHeartbeat(w http.ResponseWriter, r *http.Request) {
	user, device, ok := a.requireClient(w, r)
	if !ok {
		return
	}
	var req struct {
		DesktopVersion string `json:"desktopVersion"`
		CodexVersion   string `json:"codexVersion"`
	}
	if !readStrictJSON(w, r, &req, "invalid_launch_heartbeat_request", "desktopVersion", "codexVersion") {
		return
	}
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	now := time.Now().UTC()
	if a.gatewayLeases == nil {
		a.gatewayLeases = map[string]map[string]time.Time{}
	}
	a.setClientRuntimeLocked(user.ID, device.ID, clientRuntimeLease{
		ExpiresAt:      now.Add(gatewayClientLeaseTTL),
		DesktopVersion: sanitizeRuntimeVersion(req.DesktopVersion, 32),
		CodexVersion:   sanitizeRuntimeVersion(req.CodexVersion, 96),
	})
	if assigned := a.gatewayUserAssignedUpstreamLocked(user.ID, now); assigned != "" {
		if a.gatewayUpstreamRoutableLocked(assigned) {
			a.gatewayLeases[assigned][user.ID] = now.Add(gatewayClientLeaseTTL)
			writeJSON(w, http.StatusOK, map[string]any{"active": true, "assigned": true})
			return
		}
		a.clearGatewayLeaseForUserLocked(user.ID)
	}
	latest := UsageRecord{}
	for _, record := range a.store.state.UsageRecords {
		if record.UserID == user.ID && record.UpstreamAccountID != "" && (latest.ID == "" || record.CreatedAt.After(latest.CreatedAt)) {
			latest = record
		}
	}
	if latest.ID == "" {
		writeJSON(w, http.StatusOK, map[string]any{"active": true, "assigned": false})
		return
	}
	if !a.gatewayUpstreamRoutableLocked(latest.UpstreamAccountID) {
		writeJSON(w, http.StatusOK, map[string]any{"active": true, "assigned": false})
		return
	}
	activeUsers := a.gatewayUpstreamActiveUsersLocked(latest.UpstreamAccountID, now)
	limit := a.upstreamUserLimit
	if limit <= 0 {
		limit = defaultGatewayUpstreamUserLimit
	}
	if _, exists := activeUsers[user.ID]; !exists && int64(len(activeUsers)) >= limit {
		writeJSON(w, http.StatusOK, map[string]any{"active": true, "assigned": false})
		return
	}
	users := a.gatewayLeases[latest.UpstreamAccountID]
	if users == nil {
		users = map[string]time.Time{}
		a.gatewayLeases[latest.UpstreamAccountID] = users
	}
	users[user.ID] = now.Add(gatewayClientLeaseTTL)
	writeJSON(w, http.StatusOK, map[string]any{"active": true, "assigned": true})
}

func (a *App) clientLaunchStop(w http.ResponseWriter, r *http.Request) {
	user, device, ok := a.requireClient(w, r)
	if !ok {
		return
	}
	if !readEmptyJSON(w, r, "invalid_launch_stop_request") {
		return
	}
	a.store.mu.Lock()
	a.clearClientRuntimeLocked(user.ID, device.ID)
	_, runtimeActive := a.clientRuntimeSnapshotLocked(user.ID, time.Now().UTC())
	if !runtimeActive {
		a.clearGatewayLeaseForUserLocked(user.ID)
	}
	a.store.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"active": false})
}

func (a *App) clientPresenceHeartbeat(w http.ResponseWriter, r *http.Request) {
	_, _, ok := a.requireClient(w, r)
	if !ok {
		return
	}
	if !readEmptyJSON(w, r, "invalid_client_presence_request") {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"online": true})
}

func (a *App) clientPresenceStop(w http.ResponseWriter, r *http.Request) {
	user, device, ok := a.requireClient(w, r)
	if !ok {
		return
	}
	if !readEmptyJSON(w, r, "invalid_client_presence_request") {
		return
	}
	a.store.mu.Lock()
	a.clearClientPresenceLocked(user.ID, device.ID)
	a.store.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"online": false})
}

func sanitizeRuntimeVersion(value string, limit int) string {
	value = strings.TrimSpace(value)
	clean := make([]rune, 0, len(value))
	for _, char := range value {
		if unicode.IsControl(char) {
			continue
		}
		clean = append(clean, char)
		if len(clean) == limit {
			break
		}
	}
	return string(clean)
}

func (a *App) setClientRuntimeLocked(userID, deviceID string, lease clientRuntimeLease) {
	if a.clientRuntimes == nil {
		a.clientRuntimes = map[string]map[string]clientRuntimeLease{}
	}
	devices := a.clientRuntimes[userID]
	if devices == nil {
		devices = map[string]clientRuntimeLease{}
		a.clientRuntimes[userID] = devices
	}
	devices[deviceID] = lease
}

func (a *App) clearClientRuntimeLocked(userID, deviceID string) {
	devices := a.clientRuntimes[userID]
	delete(devices, deviceID)
	if len(devices) == 0 {
		delete(a.clientRuntimes, userID)
	}
}

func (a *App) markClientPresenceLocked(userID, deviceID string, now time.Time) {
	userID = strings.TrimSpace(userID)
	deviceID = strings.TrimSpace(deviceID)
	if userID == "" {
		return
	}
	if deviceID == "" {
		deviceID = legacyClientPresenceDeviceID
	}
	if a.clientPresence == nil {
		a.clientPresence = map[string]map[string]time.Time{}
	}
	devices := a.clientPresence[userID]
	if devices == nil {
		devices = map[string]time.Time{}
		a.clientPresence[userID] = devices
	}
	devices[deviceID] = now.Add(clientPresenceTTL)
}

func (a *App) clearClientPresenceLocked(userID, deviceID string) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		deviceID = legacyClientPresenceDeviceID
	}
	devices := a.clientPresence[userID]
	delete(devices, deviceID)
	if len(devices) == 0 {
		delete(a.clientPresence, userID)
	}
}

func (a *App) clearGatewayLeaseForUserLocked(userID string) {
	for upstreamID, users := range a.gatewayLeases {
		delete(users, userID)
		if len(users) == 0 {
			delete(a.gatewayLeases, upstreamID)
		}
	}
}

func (a *App) gatewayUserAssignedUpstreamLocked(userID string, now time.Time) string {
	assigned := ""
	var latestExpiry time.Time
	for upstreamID, users := range a.gatewayLeases {
		for candidate, expiresAt := range users {
			if !expiresAt.After(now) {
				delete(users, candidate)
				continue
			}
			if candidate == userID && (assigned == "" || expiresAt.After(latestExpiry)) {
				assigned = upstreamID
				latestExpiry = expiresAt
			}
		}
		if len(users) == 0 {
			delete(a.gatewayLeases, upstreamID)
		}
	}
	return assigned
}

func (a *App) gatewayUpstreamActiveUsersLocked(upstreamID string, now time.Time) map[string]struct{} {
	active := make(map[string]struct{})
	for userID, count := range a.gatewayActive[upstreamID] {
		if count > 0 {
			active[userID] = struct{}{}
		}
	}
	if users := a.gatewayLeases[upstreamID]; users != nil {
		for userID, expiresAt := range users {
			if expiresAt.After(now) {
				active[userID] = struct{}{}
				continue
			}
			delete(users, userID)
		}
		if len(users) == 0 {
			delete(a.gatewayLeases, upstreamID)
		}
	}
	return active
}

func (a *App) gatewayUpstreamRoutableLocked(upstreamID string) bool {
	upstream := a.upstreamByID(upstreamID)
	if upstream.ID == "" || !upstreamIsAvailable(upstream) {
		return false
	}
	for _, key := range a.store.state.APIKeys {
		if key.UpstreamAccountID == upstreamID && key.Status == statusActive && strings.TrimSpace(key.KeyCipher) != "" {
			return true
		}
	}
	return false
}

func (a *App) acquireGatewayUserSlot(upstreamID, userID string, allowSwitch bool) bool {
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	if a.gatewayLeases == nil {
		a.gatewayLeases = map[string]map[string]time.Time{}
	}
	if a.gatewayActive == nil {
		a.gatewayActive = map[string]map[string]int{}
	}
	now := time.Now().UTC()
	current := a.gatewayUserAssignedUpstreamLocked(userID, now)
	if current == upstreamID {
		a.gatewayLeases[upstreamID][userID] = now.Add(gatewayClientLeaseTTL)
		return true
	}
	if current != "" && !allowSwitch {
		return false
	}
	activeUsers := a.gatewayUpstreamActiveUsersLocked(upstreamID, now)
	limit := a.upstreamUserLimit
	if limit <= 0 {
		limit = defaultGatewayUpstreamUserLimit
	}
	_, alreadyActive := activeUsers[userID]
	if (!alreadyActive && int64(len(activeUsers)) >= limit) || (alreadyActive && int64(len(activeUsers)) > limit) {
		return false
	}
	a.clearGatewayLeaseForUserLocked(userID)
	users := a.gatewayLeases[upstreamID]
	if users == nil {
		users = map[string]time.Time{}
		a.gatewayLeases[upstreamID] = users
	}
	users[userID] = now.Add(gatewayClientLeaseTTL)
	return true
}

func (a *App) clientLaunchAccessKeyLocked(userID, deviceID string, managedRuntime bool, now time.Time) (string, error) {
	if len(a.routeCandidatesLocked()) == 0 {
		return "", errRouteUnavailable
	}
	if managedRuntime && strings.TrimSpace(deviceID) == "" {
		return "", errors.New("client_update_required")
	}
	if !managedRuntime {
		for _, key := range a.store.state.ClientAccessKeys {
			if key.UserID == userID && key.DeviceID != "" && key.Status == statusActive {
				return "", errors.New("client_update_required")
			}
		}
	}
	for i := range a.store.state.ClientAccessKeys {
		key := &a.store.state.ClientAccessKeys[i]
		if key.UserID != userID || key.Status != statusActive || strings.TrimSpace(key.KeyCipher) == "" || (managedRuntime && key.DeviceID != deviceID) || (!managedRuntime && key.DeviceID != "") {
			continue
		}
		raw, err := a.decrypt(key.KeyCipher)
		if err != nil {
			continue
		}
		raw = strings.TrimSpace(raw)
		if raw == "" || hashString(raw) != key.KeyHash {
			continue
		}
		if managedRuntime {
			a.disableLegacyClientAccessKeysLocked(userID, now)
		}
		key.UpdatedAt = now
		return raw, nil
	}
	raw, err := generateSub2APIKey()
	if err != nil {
		return "", err
	}
	cipherText, err := a.encrypt(raw)
	if err != nil {
		return "", err
	}
	keyDeviceID := ""
	if managedRuntime {
		keyDeviceID = deviceID
	}
	a.store.state.ClientAccessKeys = append(a.store.state.ClientAccessKeys, ClientAccessKey{
		ID:           a.store.nextID("cak"),
		KeyCipher:    cipherText,
		KeyHash:      hashString(raw),
		PublicPrefix: raw[:10],
		UserID:       userID,
		DeviceID:     keyDeviceID,
		Status:       statusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if managedRuntime {
		a.disableLegacyClientAccessKeysLocked(userID, now)
	}
	return raw, nil
}

func (a *App) disableLegacyClientAccessKeysLocked(userID string, now time.Time) {
	for i := range a.store.state.ClientAccessKeys {
		key := &a.store.state.ClientAccessKeys[i]
		if key.UserID == userID && key.DeviceID == "" && key.Status == statusActive {
			key.Status = statusDisabled
			key.UpdatedAt = now
		}
	}
}

func (a *App) gatewayRun(w http.ResponseWriter, r *http.Request) {
	user, _, ok := a.requireClient(w, r)
	if !ok {
		return
	}
	if !a.acquireGatewayUploadSlot(user.ID) {
		w.Header().Set("Retry-After", "2")
		writeErr(w, http.StatusTooManyRequests, "upload_capacity_exhausted")
		return
	}
	defer r.Body.Close()
	body, bodyErr := prepareGatewayRequest(r)
	a.releaseGatewayUploadSlot(user.ID)
	if bodyErr != nil {
		logGatewayBodyError(r, bodyErr)
		writeErr(w, bodyErr.Status, bodyErr.Code)
		return
	}
	defer body.Close()
	a.gatewayRunForUser(w, r, user, "", body)
}

func (a *App) gatewayRunForUser(w http.ResponseWriter, r *http.Request, user User, clientAccessKeyID string, requestBody *gatewayRequestSource) {
	requestModel := requestBody.metadata.Model
	requestID := gatewayRequestID(r, requestBody.metadata.RequestID)
	sessionID := gatewaySessionIDForMetadata(r, requestBody.metadata)
	a.store.mu.Lock()
	idx := a.userIndex(user.ID)
	if idx < 0 || a.store.state.Users[idx].Status != statusActive {
		a.store.mu.Unlock()
		writeErr(w, http.StatusUnauthorized, "login_failed")
		return
	}
	recoveryAuditStart := len(a.store.state.AuditLogs)
	if a.releaseOrphanGatewayRequestLocked(user.ID, requestID, time.Now().UTC()) {
		recovered := a.store.state.GatewayRequests[a.gatewayRequestIndexLocked(user.ID, requestID)]
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := a.store.saveGatewayFailure(ctx, recovered, append([]AuditLog(nil), a.store.state.AuditLogs[recoveryAuditStart:]...))
		cancel()
		if err != nil {
			a.reloadPostgresAfterGatewayFailureLocked()
			a.store.mu.Unlock()
			writeErr(w, http.StatusInternalServerError, "state_save_failed")
			return
		}
	}
	if status, payload, handled := a.existingGatewayResponseLocked(user.ID, requestID); handled {
		record := a.store.state.GatewayRequests[a.gatewayRequestIndexLocked(user.ID, requestID)]
		a.store.mu.Unlock()
		if err := a.captureStoredCodexResponse(r, record); err != nil {
			writeErr(w, http.StatusServiceUnavailable, "state_read_failed")
			return
		}
		writeJSON(w, status, payload)
		return
	}
	a.store.mu.Unlock()
	if status, code := a.gatewayRequestAdmission(r.Context(), user.ID); code != "" {
		writeErr(w, status, code)
		return
	}
	preferredUpstreamID := ""
	var err error
	if a.gatewayRuntime != nil && sessionID != "" {
		preferredUpstreamID, err = a.gatewayRuntime.SessionRoute(r.Context(), gatewayRuntimeSessionKey(user.ID, sessionID))
		if err != nil {
			writeErr(w, http.StatusServiceUnavailable, "gateway_runtime_unavailable")
			return
		}
	}
	a.store.mu.Lock()
	idx = a.userIndex(user.ID)
	if idx < 0 || a.store.state.Users[idx].Status != statusActive {
		a.store.mu.Unlock()
		writeErr(w, http.StatusUnauthorized, "login_failed")
		return
	}
	if status, payload, handled := a.existingGatewayResponseLocked(user.ID, requestID); handled {
		record := a.store.state.GatewayRequests[a.gatewayRequestIndexLocked(user.ID, requestID)]
		a.store.mu.Unlock()
		if err := a.captureStoredCodexResponse(r, record); err != nil {
			writeErr(w, http.StatusServiceUnavailable, "state_read_failed")
			return
		}
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
	assignedUpstreamID := a.gatewayUserAssignedUpstreamLocked(user.ID, time.Now().UTC())
	assignedRouteAvailable := false
	for _, route := range routes {
		if route.Upstream.ID == assignedUpstreamID {
			assignedRouteAvailable = true
			break
		}
	}
	routes = a.prioritizeGatewaySessionRouteLocked(user.ID, sessionID, preferredUpstreamID, routes)
	routes = a.prioritizeGatewayUserRouteLocked(user.ID, routes, time.Now().UTC())
	reservedTokens := gatewayRequestReservationTokensForSource(requestBody)
	reservedTokens = gatewayReservationForBalance(reservedTokens, availableBalance)
	now := time.Now().UTC()
	previousNextID := a.store.state.NextID
	existingIdx := a.gatewayRequestIndexLocked(user.ID, requestID)
	var previousRequest GatewayRequest
	if existingIdx >= 0 {
		previousRequest = a.store.state.GatewayRequests[existingIdx]
		req := &a.store.state.GatewayRequests[existingIdx]
		req.Status = gatewayReserved
		req.ReservedTokens = reservedTokens
		req.ChargedTokens = 0
		req.UsageRecordID = ""
		req.UpstreamStatus = 0
		req.Error = ""
		req.ResultText = ""
		req.ResultBody = ""
		req.ResultType = ""
		req.ResultHeaders = ""
		req.UpdatedAt = now
	} else {
		a.store.state.GatewayRequests = append(a.store.state.GatewayRequests, GatewayRequest{ID: a.store.nextID("gw"), UserID: user.ID, RequestID: requestID, Status: gatewayReserved, ReservedTokens: reservedTokens, CreatedAt: now, UpdatedAt: now})
		existingIdx = len(a.store.state.GatewayRequests) - 1
	}
	a.setGatewayRequestInFlightLocked(user.ID, requestID, true)
	reservation := a.store.state.GatewayRequests[existingIdx]
	if err := a.store.saveGatewayReservation(r.Context(), reservation); err != nil {
		a.setGatewayRequestInFlightLocked(user.ID, requestID, false)
		if previousRequest.ID == "" {
			a.store.state.GatewayRequests = a.store.state.GatewayRequests[:existingIdx]
			a.store.state.NextID = previousNextID
		} else {
			a.store.state.GatewayRequests[existingIdx] = previousRequest
		}
		a.store.mu.Unlock()
		writeGatewayReservationError(w, err)
		return
	}
	a.store.mu.Unlock()
	defer a.setGatewayRequestInFlight(user.ID, requestID, false)

	var upstreamResult codexResponsesResult
	var selected gatewayRoute
	var lastFailure gatewayRouteFailure
	var switchFrom gatewayRoute
	var switchReason string
	allowUserSwitch := assignedUpstreamID != "" && !assignedRouteAvailable
	for _, route := range routes {
		leaseID, acquired, leaseErr := a.acquireGatewayUpstreamLease(r.Context(), route.Upstream.ID, user.ID, requestID)
		if leaseErr != nil {
			lastFailure = gatewayRouteFailure{Code: "gateway_runtime_unavailable", HTTPStatus: http.StatusServiceUnavailable, TryNext: false}
			break
		}
		if !acquired {
			lastFailure = gatewayRouteFailure{Code: "upstream_capacity_exhausted", HTTPStatus: http.StatusServiceUnavailable, TryNext: false}
			continue
		}
		stopLease := a.maintainGatewayUpstreamLease(route.Upstream.ID, user.ID, leaseID)
		if !a.acquireGatewayUserSlot(route.Upstream.ID, user.ID, allowUserSwitch) {
			stopLease()
			lastFailure = gatewayRouteFailure{Code: "upstream_user_limit_reached", HTTPStatus: http.StatusServiceUnavailable, TryNext: false}
			continue
		}
		result, failure, ok := a.tryGatewayRoute(r.Context(), route, requestBody, r.Header)
		stopLease()
		if ok {
			upstreamResult = result
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
		allowUserSwitch = true
		switchFrom = route
		switchReason = failure.Code
	}
	if selected.Upstream.ID == "" {
		status := lastFailure.UpstreamStatus
		code := valueOr(lastFailure.Code, "route_unavailable")
		if err := a.failGatewayRequest(user.ID, requestID, status, code); err != nil {
			if !lastFailure.ResponseStarted {
				writeErr(w, http.StatusInternalServerError, "state_save_failed")
			}
			return
		}
		if lastFailure.ResponseStarted {
			return
		}
		writeGatewayFailure(w, lastFailure)
		return
	}
	if sessionID != "" {
		a.rememberGatewaySessionRoute(r.Context(), user.ID, sessionID, selected.Upstream.ID)
	}
	model := valueOr(stringField(upstreamResult.Payload, "model"), valueOr(requestModel, "codex"))
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	idx = a.userIndex(user.ID)
	if idx < 0 || a.store.state.Users[idx].Status != statusActive {
		a.markGatewayRequestFailedLocked(user.ID, requestID, upstreamResult.Status, "login_failed")
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
	usage := upstreamResult.Usage
	chargeTokens := usage.TotalTokens
	if balance := a.store.state.Users[idx].TokenBalance; balance < chargeTokens {
		chargeTokens = balance
	}
	oldBalance := a.store.state.Users[idx].TokenBalance
	ledgerStart := len(a.store.state.TokenLedgers)
	auditStart := len(a.store.state.AuditLogs)
	finishedAt := time.Now().UTC()
	rec := UsageRecord{ID: a.store.nextID("use"), UserID: user.ID, UpstreamAccountID: selected.Upstream.ID, APIKeyID: selected.Key.ID, ClientAccessKeyID: clientAccessKeyID, SessionID: sessionID, Model: model, InputTokens: usage.InputTokens, CachedInputTokens: usage.CachedInputTokens, OutputTokens: usage.OutputTokens, TotalTokens: usage.TotalTokens, CreatedAt: finishedAt}
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
	result := resultField(upstreamResult.Payload)
	req := &a.store.state.GatewayRequests[reqIdx]
	req.Status = gatewayCompleted
	req.ReservedTokens = 0
	req.ChargedTokens = chargeTokens
	req.UsageRecordID = rec.ID
	req.UpstreamStatus = upstreamResult.Status
	req.Error = ""
	req.ResultText = textFromAny(result)
	req.ResultBody = ""
	if int64(len(upstreamResult.Body)) <= maxStoredCodexResponseBodyBytes {
		req.ResultBody = string(upstreamResult.Body)
	}
	req.ResultType = upstreamResult.ContentType
	req.ResultHeaders = encodeCodexResponseHeaders(upstreamResult.Header)
	req.UpdatedAt = finishedAt
	if keyIdx := a.apiKeyIndex(selected.Key.ID); keyIdx >= 0 {
		a.store.state.APIKeys[keyIdx].LastUsedAt = &finishedAt
		a.store.state.APIKeys[keyIdx].UpdatedAt = finishedAt
	}
	if clientKeyIdx := a.clientAccessKeyIndex(clientAccessKeyID); clientKeyIdx >= 0 {
		a.store.state.ClientAccessKeys[clientKeyIdx].LastUsedAt = &finishedAt
		a.store.state.ClientAccessKeys[clientKeyIdx].UpdatedAt = finishedAt
	}
	var persistedAPIKey *APIKey
	if keyIdx := a.apiKeyIndex(selected.Key.ID); keyIdx >= 0 {
		copy := a.store.state.APIKeys[keyIdx]
		persistedAPIKey = &copy
	}
	var persistedClientKey *ClientAccessKey
	if clientKeyIdx := a.clientAccessKeyIndex(clientAccessKeyID); clientKeyIdx >= 0 {
		copy := a.store.state.ClientAccessKeys[clientKeyIdx]
		persistedClientKey = &copy
	}
	var persistedSession *GatewaySession
	if sessionID != "" {
		sessionKey := gatewaySessionKey(sessionID)
		for i := range a.store.state.GatewaySessions {
			if a.store.state.GatewaySessions[i].UserID == user.ID && a.store.state.GatewaySessions[i].SessionKey == sessionKey {
				copy := a.store.state.GatewaySessions[i]
				persistedSession = &copy
				break
			}
		}
	}
	settlement := gatewaySettlement{
		Request:    *req,
		Usage:      rec,
		OldBalance: oldBalance,
		NewBalance: a.store.state.Users[idx].TokenBalance,
		Ledgers:    append([]TokenLedger(nil), a.store.state.TokenLedgers[ledgerStart:]...),
		APIKey:     persistedAPIKey,
		ClientKey:  persistedClientKey,
		Session:    persistedSession,
		Audits:     append([]AuditLog(nil), a.store.state.AuditLogs[auditStart:]...),
		NextID:     a.store.state.NextID,
	}
	persistCtx, persistCancel := context.WithTimeout(context.Background(), 10*time.Second)
	err = a.store.saveGatewaySettlement(persistCtx, settlement)
	persistCancel()
	if err != nil {
		a.reloadPostgresAfterGatewayFailureLocked()
		writeErr(w, http.StatusInternalServerError, "state_save_failed")
		return
	}
	if a.store.db != nil {
		// The settlement copy above owns the durable replay payload. Keep only
		// metadata in the resident state so completed tasks cannot accumulate in
		// the backend heap.
		req.ResultBody = ""
	}
	setCodexResponseCapture(r, upstreamResult)
	writeJSON(w, http.StatusOK, gatewayRunResponse(requestID, rec, upstreamResult.Status, chargeTokens, false, result))
}

func (a *App) codexResponses(w http.ResponseWriter, r *http.Request) {
	user, _, accessKey, ok := a.requireCodexClient(w, r)
	if !ok {
		return
	}
	if !a.acquireGatewayUploadSlot(user.ID) {
		w.Header().Set("Retry-After", "2")
		writeCodexProviderError(w, http.StatusTooManyRequests, "upload_capacity_exhausted")
		return
	}
	defer r.Body.Close()
	body, bodyErr := prepareGatewayRequest(r)
	a.releaseGatewayUploadSlot(user.ID)
	if bodyErr != nil {
		logGatewayBodyError(r, bodyErr)
		writeCodexProviderError(w, bodyErr.Status, bodyErr.Code)
		return
	}
	defer body.Close()
	stream := body.metadata.Stream
	providerCapture := &codexResponseCapture{}
	r = r.WithContext(context.WithValue(r.Context(), codexResponseCaptureContextKey{}, providerCapture))
	streamTarget := &codexStreamTarget{}
	if stream {
		streamTarget.Writer = w
		r = r.WithContext(context.WithValue(r.Context(), codexStreamTargetContextKey{}, streamTarget))
	}
	gatewayCapture := newCaptureResponse()
	a.gatewayRunForUser(gatewayCapture, r, user, accessKey.ID, body)
	if streamTarget.Started {
		return
	}
	status := gatewayCapture.status
	if status == 0 {
		status = http.StatusOK
	}
	var gatewayPayload map[string]any
	dec := json.NewDecoder(bytes.NewReader(gatewayCapture.body.Bytes()))
	dec.UseNumber()
	if err := dec.Decode(&gatewayPayload); err != nil {
		writeErr(w, http.StatusBadGateway, "gateway_invalid_json")
		return
	}
	if status < 200 || status >= 300 {
		writeJSON(w, status, codexErrorPayloadFromGateway(gatewayPayload))
		return
	}
	if len(providerCapture.Result.Body) > 0 {
		writeCodexResponsesResult(w, providerCapture.Result)
		return
	}
	// Compatibility fallback for idempotency records written before raw
	// Responses payloads were persisted.
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
	ResponseStarted bool
}

func (a *App) tryGatewayRoute(ctx context.Context, route gatewayRoute, requestBody *gatewayRequestSource, requestHeaders http.Header) (codexResponsesResult, gatewayRouteFailure, bool) {
	credentials, refreshedReq, err := a.credentialsForUpstream(ctx, route.Upstream)
	if err != nil {
		return codexResponsesResult{}, gatewayRouteFailure{Code: "secret_decrypt_failed", HTTPStatus: http.StatusServiceUnavailable, TryNext: true, MarkUnavailable: true}, false
	}
	if refreshedReq != nil {
		a.store.mu.Lock()
		idx := a.upstreamIndex(route.Upstream.ID)
		if idx >= 0 {
			if err := a.applyRefreshedUpstreamLocked(&a.store.state.UpstreamAccounts[idx], *refreshedReq, time.Now().UTC()); err == nil {
				if saveErr := a.store.save(); saveErr != nil {
					log.Printf("save refreshed upstream credentials failed: %v", saveErr)
				}
			} else {
				log.Printf("apply refreshed upstream credentials failed: %v", err)
			}
		}
		a.store.mu.Unlock()
	}
	var result codexResponsesResult
	streamTarget, _ := ctx.Value(codexStreamTargetContextKey{}).(*codexStreamTarget)
	if streamTarget != nil {
		result, err = codexResponsesStreamRun(ctx, credentials, requestBody, requestHeaders, streamTarget)
	} else {
		result, err = codexResponsesRun(ctx, credentials, requestBody, requestHeaders)
	}
	if err != nil {
		failure := classifyCodexResponsesError(err)
		if streamTarget != nil && streamTarget.Started {
			failure.TryNext = false
			failure.ResponseStarted = true
		}
		return codexResponsesResult{}, failure, false
	}
	if result.Status < 200 || result.Status >= 300 {
		return codexResponsesResult{}, classifyUpstreamStatus(result.Status), false
	}
	if result.Usage.TotalTokens <= 0 {
		return codexResponsesResult{}, classifyCodexResponsesError(errors.New("codex_responses_usage_missing")), false
	}
	if result.Status == 0 {
		result.Status = http.StatusOK
	}
	if strings.TrimSpace(result.ContentType) == "" {
		result.ContentType = result.Header.Get("Content-Type")
	}
	return result, gatewayRouteFailure{}, true
}

func classifyCodexResponsesError(err error) gatewayRouteFailure {
	code := err.Error()
	failure := gatewayRouteFailure{Code: code, HTTPStatus: http.StatusBadGateway, TryNext: true, MarkUnavailable: false}
	switch {
	case code == "codex_responses_request_invalid":
		failure.HTTPStatus = http.StatusBadRequest
		failure.TryNext = false
	case code == "codex_responses_missing_chatgpt_account_id":
		failure.HTTPStatus = http.StatusServiceUnavailable
		failure.MarkUnavailable = true
	case code == "codex_responses_usage_missing":
		failure.HTTPStatus = http.StatusBadGateway
		failure.TryNext = false
	case code == "codex_responses_invalid_body" || code == "codex_responses_too_large":
		failure.HTTPStatus = http.StatusBadGateway
		failure.TryNext = false
	case code == "codex_responses_unavailable" || code == "codex_responses_read_failed":
		failure.HTTPStatus = http.StatusServiceUnavailable
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

func runCodexResponsesRequest(ctx context.Context, credentials codexProbeCredentials, requestBody []byte, requestHeaders http.Header) (codexResponsesResult, error) {
	return runCodexResponsesRequestSource(ctx, credentials, newMemoryGatewayRequestSource(requestBody), requestHeaders)
}

func runCodexResponsesRequestSource(ctx context.Context, credentials codexProbeCredentials, requestBody *gatewayRequestSource, requestHeaders http.Header) (codexResponsesResult, error) {
	if strings.TrimSpace(credentials.AccessToken) == "" {
		return codexResponsesResult{}, errors.New("codex_responses_unavailable")
	}
	if strings.TrimSpace(credentials.ChatGPTAccountID) == "" {
		return codexResponsesResult{}, errors.New("codex_responses_missing_chatgpt_account_id")
	}
	endpoint := strings.TrimSpace(os.Getenv("CODEXPPP_CODEX_RESPONSES_URL"))
	if endpoint == "" {
		endpoint = defaultCodexResponsesURL
	}
	bodyReader, err := requestBody.Open()
	if err != nil {
		return codexResponsesResult{}, errors.New("codex_responses_request_invalid")
	}
	defer bodyReader.Close()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bodyReader)
	if err != nil {
		return codexResponsesResult{}, errors.New("codex_responses_request_invalid")
	}
	req.ContentLength = requestBody.size
	copyCodexRequestHeaders(req.Header, requestHeaders)
	req.Header.Set("Authorization", "Bearer "+credentials.AccessToken)
	req.Header.Set("ChatGPT-Account-ID", credentials.ChatGPTAccountID)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return codexResponsesResult{}, errors.New("codex_responses_unavailable")
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, maxCodexResponseBodyBytes+1))
	if err != nil {
		return codexResponsesResult{}, errors.New("codex_responses_read_failed")
	}
	if int64(len(raw)) > maxCodexResponseBodyBytes {
		return codexResponsesResult{}, errors.New("codex_responses_too_large")
	}
	result := codexResponsesResult{
		Status:      res.StatusCode,
		Header:      filterCodexResponseHeaders(res.Header),
		Body:        raw,
		ContentType: res.Header.Get("Content-Type"),
	}
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

func copyCodexRequestHeaders(dst, src http.Header) {
	for name, values := range src {
		if !codexRequestHeaderAllowed(name) {
			continue
		}
		for _, value := range values {
			dst.Add(name, value)
		}
	}
}

func codexRequestHeaderAllowed(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if strings.HasPrefix(name, "x-codex-") {
		return true
	}
	switch name {
	case "openai-beta", "originator", "session-id", "thread-id", "x-client-request-id",
		"x-openai-subagent", "x-openai-memgen-request", "x-openai-internal-codex-responses-lite",
		"x-responsesapi-include-timing-metrics", "x-oai-attestation", "user-agent":
		return true
	default:
		return false
	}
}

func filterCodexResponseHeaders(src http.Header) http.Header {
	dst := http.Header{}
	for name, values := range src {
		if !codexResponseHeaderAllowed(name) {
			continue
		}
		for _, value := range values {
			dst.Add(name, value)
		}
	}
	return dst
}

func codexResponseHeaderAllowed(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if strings.HasPrefix(name, "x-codex-") || strings.HasPrefix(name, "x-ratelimit-") {
		return true
	}
	switch name {
	case "content-type", "openai-model", "openai-processing-ms", "retry-after", "x-reasoning-included", "x-request-id":
		return true
	default:
		return false
	}
}

func parseCodexResponsesPayload(raw []byte, contentType string) (any, gatewayUsage, error) {
	if !strings.Contains(strings.ToLower(contentType), "text/event-stream") && !bytes.HasPrefix(bytes.TrimSpace(raw), []byte("event:")) && !bytes.HasPrefix(bytes.TrimSpace(raw), []byte("data:")) {
		payload, err := decodeJSONValue(raw)
		if err != nil {
			return nil, gatewayUsage{}, errors.New("codex_responses_invalid_body")
		}
		usage, ok := extractGatewayUsage(payload)
		if !ok {
			return nil, gatewayUsage{}, errors.New("codex_responses_usage_missing")
		}
		return payload, usage, nil
	}
	var completed any
	var lastPayload any
	var foundUsage gatewayUsage
	var hasUsage bool
	for _, line := range bytes.Split(raw, []byte{'\n'}) {
		line = bytes.TrimSpace(line)
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		data := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
		if len(data) == 0 || bytes.Equal(data, []byte("[DONE]")) {
			continue
		}
		payload, err := decodeJSONValue(data)
		if err != nil {
			return nil, gatewayUsage{}, errors.New("codex_responses_invalid_body")
		}
		lastPayload = payload
		if usage, ok := extractGatewayUsage(payload); ok {
			foundUsage = usage
			hasUsage = true
		}
		if event, ok := payload.(map[string]any); ok {
			switch stringField(event, "type") {
			case "response.completed":
				if response, exists := event["response"]; exists {
					completed = response
				}
			case "response.failed":
				return nil, gatewayUsage{}, errors.New("codex_responses_upstream_failed")
			}
		}
	}
	if completed == nil {
		completed = lastPayload
	}
	if completed == nil {
		return nil, gatewayUsage{}, errors.New("codex_responses_invalid_body")
	}
	if usage, ok := extractGatewayUsage(completed); ok {
		foundUsage = usage
		hasUsage = true
	}
	if !hasUsage {
		return nil, gatewayUsage{}, errors.New("codex_responses_usage_missing")
	}
	return completed, foundUsage, nil
}

func decodeJSONValue(raw []byte) (any, error) {
	var payload any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&payload); err != nil {
		return nil, err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, errors.New("invalid_json")
	}
	return payload, nil
}

func setCodexResponseCapture(r *http.Request, result codexResponsesResult) {
	if r == nil {
		return
	}
	capture, _ := r.Context().Value(codexResponseCaptureContextKey{}).(*codexResponseCapture)
	if capture == nil {
		return
	}
	result.Header = filterCodexResponseHeaders(result.Header)
	result.Body = append([]byte(nil), result.Body...)
	capture.Result = result
}

func (a *App) captureStoredCodexResponse(r *http.Request, record GatewayRequest) error {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	result, found, err := a.store.loadGatewayReplay(ctx, record)
	if err != nil {
		return err
	}
	if found {
		setCodexResponseCapture(r, result)
	}
	return nil
}

func encodeCodexResponseHeaders(headers http.Header) string {
	raw, err := json.Marshal(filterCodexResponseHeaders(headers))
	if err != nil {
		return ""
	}
	return string(raw)
}

func decodeCodexResponseHeaders(raw string) http.Header {
	headers := http.Header{}
	if strings.TrimSpace(raw) == "" {
		return headers
	}
	if err := json.Unmarshal([]byte(raw), &headers); err != nil {
		return http.Header{}
	}
	return filterCodexResponseHeaders(headers)
}

func writeCodexResponsesResult(w http.ResponseWriter, result codexResponsesResult) {
	for name, values := range filterCodexResponseHeaders(result.Header) {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}
	if strings.TrimSpace(w.Header().Get("Content-Type")) == "" {
		contentType := strings.TrimSpace(result.ContentType)
		if contentType == "" {
			contentType = "application/json; charset=utf-8"
		}
		w.Header().Set("Content-Type", contentType)
	}
	status := result.Status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	_, _ = w.Write(result.Body)
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

func gatewaySessionID(r *http.Request, body []byte) string {
	for _, key := range []string{"Session-ID", "Thread-ID", "X-Codex-Session-ID", "X-Codex-Thread-ID", "X-Codex-Window-ID", "X-Subrouter-Session", "X-Session-ID"} {
		if value := gatewaySessionIDFromValue(r.Header.Get(key)); value != "" {
			return value
		}
	}
	var payload map[string]any
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&payload); err != nil {
		return ""
	}
	for _, key := range []string{"session_id", "sessionId", "thread_id", "threadId", "conversation_id", "conversationId", "conversation", "codex_session_id", "codexSessionId"} {
		if value := gatewaySessionIDFromValue(stringField(payload, key)); value != "" {
			return value
		}
	}
	if conversation, ok := payload["conversation"].(map[string]any); ok {
		if value := gatewaySessionIDFromValue(stringField(conversation, "id")); value != "" {
			return value
		}
	}
	if metadata, ok := payload["metadata"].(map[string]any); ok {
		for _, key := range []string{"session_id", "sessionId", "thread_id", "threadId", "conversation_id", "conversationId"} {
			if value := gatewaySessionIDFromValue(stringField(metadata, key)); value != "" {
				return value
			}
		}
	}
	return ""
}

func gatewaySessionIDForMetadata(r *http.Request, metadata gatewayRequestMetadata) string {
	for _, key := range []string{"Session-ID", "Thread-ID", "X-Codex-Session-ID", "X-Codex-Thread-ID", "X-Codex-Window-ID", "X-Subrouter-Session", "X-Session-ID"} {
		if value := gatewaySessionIDFromValue(r.Header.Get(key)); value != "" {
			return value
		}
	}
	return metadata.SessionID
}

func gatewaySessionIDFromValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) <= 128 && isSafeGatewayRequestID(value) {
		return value
	}
	sum := sha256.Sum256([]byte(value))
	encoded := hex.EncodeToString(sum[:])
	return "sess_" + encoded[:24]
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
	case "request_too_large":
		return "当前任务上下文过大，请新建任务后重试"
	case "unsupported_content_encoding":
		return "当前客户端请求格式暂不受支持，请更新客户端后重试"
	case "upload_capacity_exhausted":
		return "当前正在上传其他任务，请稍后重试"
	case "request_storage_unavailable":
		return "服务器暂时无法接收任务，请稍后重试"
	case "invalid_json", "invalid_gateway_request", "codex_responses_request_invalid", "upstream_rejected_request":
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

func gatewayRequestReservationTokens(body []byte) int64 {
	inputReservation := int64((len(body) + 2) / 3)
	outputReservation := gatewayDefaultOutputReservation
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err == nil {
		for _, key := range []string{"max_output_tokens", "max_completion_tokens"} {
			if value := intField(payload, key); value > 0 {
				outputReservation = value
				break
			}
		}
	}
	if outputReservation > gatewayMaximumOutputReservation {
		outputReservation = gatewayMaximumOutputReservation
	}
	reservation := inputReservation + outputReservation
	if reservation < gatewayDefaultOutputReservation {
		reservation = gatewayDefaultOutputReservation
	}
	if reservation > gatewayMaximumRequestReservation {
		reservation = gatewayMaximumRequestReservation
	}
	return reservation
}

func gatewayRequestReservationTokensForSource(body *gatewayRequestSource) int64 {
	if body == nil {
		return gatewayDefaultOutputReservation
	}
	inputReservation := (body.size + 2) / 3
	outputReservation := body.metadata.MaxOutputTokens
	if outputReservation <= 0 {
		outputReservation = gatewayDefaultOutputReservation
	}
	if outputReservation > gatewayMaximumOutputReservation {
		outputReservation = gatewayMaximumOutputReservation
	}
	reservation := inputReservation + outputReservation
	if reservation < gatewayDefaultOutputReservation {
		reservation = gatewayDefaultOutputReservation
	}
	if reservation > gatewayMaximumRequestReservation {
		reservation = gatewayMaximumRequestReservation
	}
	return reservation
}

func gatewayReservationForBalance(estimated, available int64) int64 {
	if available <= 0 {
		return 0
	}
	limit := available
	if available >= gatewayDefaultOutputReservation*2 {
		limit = available / 2
	}
	if estimated < gatewayDefaultOutputReservation {
		estimated = gatewayDefaultOutputReservation
	}
	if estimated > limit {
		return limit
	}
	return estimated
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
	if errors.Is(err, context.DeadlineExceeded) {
		return "check_timed_out"
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
	a.store.mu.Lock()
	a.markClientPresenceLocked(user.ID, device.ID, time.Now().UTC())
	a.store.mu.Unlock()
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

func (a *App) codexClientFromRequest(r *http.Request) (User, Device, ClientAccessKey, bool) {
	token := bearerToken(r)
	if strings.TrimSpace(token) == "" {
		return User{}, Device{}, ClientAccessKey{}, false
	}
	keyHash := hashString(token)
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	for _, key := range a.store.state.ClientAccessKeys {
		if key.KeyHash != keyHash || key.Status != statusActive || strings.TrimSpace(key.UserID) == "" {
			continue
		}
		user := a.userByID(key.UserID)
		if user.ID == "" || user.Status != statusActive {
			return User{}, Device{}, ClientAccessKey{}, false
		}
		if key.DeviceID == "" {
			a.markClientPresenceLocked(user.ID, "", time.Now().UTC())
			return user, Device{}, key, true
		}
		device := a.deviceByID(key.DeviceID)
		now := time.Now().UTC()
		if device.ID == "" || device.UserID != user.ID || device.Status != statusActive || !a.clientRuntimeDeviceActiveLocked(user.ID, device.ID, now) {
			return User{}, Device{}, ClientAccessKey{}, false
		}
		a.markClientPresenceLocked(user.ID, device.ID, now)
		return user, device, key, true
	}
	return User{}, Device{}, ClientAccessKey{}, false
}

func (a *App) clientRuntimeDeviceActiveLocked(userID, deviceID string, now time.Time) bool {
	devices := a.clientRuntimes[userID]
	runtime, ok := devices[deviceID]
	if !ok {
		return false
	}
	if runtime.ExpiresAt.After(now) {
		return true
	}
	delete(devices, deviceID)
	if len(devices) == 0 {
		delete(a.clientRuntimes, userID)
	}
	return false
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
	if strings.TrimSpace(token) == "" {
		return Session{}, false
	}
	now := time.Now().UTC()
	digest := sessionTokenDigest(token)
	for _, session := range a.store.state.Sessions {
		matches := session.Token == digest || (!strings.HasPrefix(session.Token, sessionTokenHashPrefix) && session.Token == token)
		if matches && session.Role == role && session.ExpiresAt.After(now) {
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
	a.store.state.Sessions = append(a.store.state.Sessions, Session{Token: sessionTokenDigest(token), Role: role, SubjectID: subjectID, DeviceID: deviceID, CreatedAt: now, ExpiresAt: now.Add(30 * 24 * time.Hour)})
}

func sessionTokenDigest(token string) string {
	return sessionTokenHashPrefix + hashString(strings.TrimSpace(token))
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

func (a *App) releaseOrphanGatewayRequestLocked(userID, requestID string, now time.Time) bool {
	idx := a.gatewayRequestIndexLocked(userID, requestID)
	if idx < 0 || a.store.state.GatewayRequests[idx].Status != gatewayReserved || a.gatewayRequestInFlightLocked(userID, requestID) {
		return false
	}
	req := &a.store.state.GatewayRequests[idx]
	released := req.ReservedTokens
	req.Status = gatewayFailed
	req.ReservedTokens = 0
	req.Error = "request_interrupted"
	req.UpdatedAt = now
	a.auditLocked("system", "system", "gateway.request.recovered", requestID, fmt.Sprintf("user_id=%s released_tokens=%d reason=request_interrupted", userID, released))
	return true
}

func (a *App) gatewayRequestInFlightLocked(userID, requestID string) bool {
	if a.gatewayInFlight == nil {
		return false
	}
	_, ok := a.gatewayInFlight[gatewayRequestProcessKey(userID, requestID)]
	return ok
}

func (a *App) setGatewayRequestInFlightLocked(userID, requestID string, active bool) {
	if a.gatewayInFlight == nil {
		a.gatewayInFlight = map[string]struct{}{}
	}
	key := gatewayRequestProcessKey(userID, requestID)
	if active {
		a.gatewayInFlight[key] = struct{}{}
		return
	}
	delete(a.gatewayInFlight, key)
}

func (a *App) setGatewayRequestInFlight(userID, requestID string, active bool) {
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	a.setGatewayRequestInFlightLocked(userID, requestID, active)
}

func gatewayRequestProcessKey(userID, requestID string) string {
	return userID + "\x00" + requestID
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

func (a *App) gatewayRequestAdmission(ctx context.Context, userID string) (int, string) {
	if a.gatewayLimiter == nil || a.gatewayRateLimit <= 0 {
		return 0, ""
	}
	window := a.gatewayRateWindow
	if window <= 0 {
		window = time.Minute
	}
	allowed, err := a.gatewayLimiter.Allow(ctx, "codexppp:gateway:user:"+userID, a.gatewayRateLimit, window)
	if err != nil {
		return http.StatusServiceUnavailable, "rate_limiter_unavailable"
	}
	if !allowed {
		return http.StatusTooManyRequests, "rate_limited"
	}
	return 0, ""
}

func (a *App) acquireGatewayUploadSlot(userID string) bool {
	a.gatewayUploadMu.Lock()
	defer a.gatewayUploadMu.Unlock()
	globalLimit := a.gatewayUploadLimit
	if globalLimit <= 0 {
		globalLimit = defaultGatewayUploadConcurrency
	}
	userLimit := a.gatewayUploadUserLimit
	if userLimit <= 0 {
		userLimit = defaultGatewayUploadUserLimit
	}
	if a.gatewayUploadTotal >= globalLimit || a.gatewayUploads[userID] >= userLimit {
		return false
	}
	if a.gatewayUploads == nil {
		a.gatewayUploads = map[string]int{}
	}
	a.gatewayUploadTotal++
	a.gatewayUploads[userID]++
	return true
}

func (a *App) releaseGatewayUploadSlot(userID string) {
	a.gatewayUploadMu.Lock()
	defer a.gatewayUploadMu.Unlock()
	if a.gatewayUploadTotal > 0 {
		a.gatewayUploadTotal--
	}
	if count := a.gatewayUploads[userID]; count > 1 {
		a.gatewayUploads[userID] = count - 1
	} else {
		delete(a.gatewayUploads, userID)
	}
}

func (a *App) failGatewayRequest(userID, requestID string, upstreamStatus int, errCode string) error {
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	auditStart := len(a.store.state.AuditLogs)
	a.markGatewayRequestFailedLocked(userID, requestID, upstreamStatus, errCode)
	idx := a.gatewayRequestIndexLocked(userID, requestID)
	if idx < 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := a.store.saveGatewayFailure(ctx, a.store.state.GatewayRequests[idx], append([]AuditLog(nil), a.store.state.AuditLogs[auditStart:]...))
	if err != nil {
		a.reloadPostgresAfterGatewayFailureLocked()
	}
	return err
}

func (a *App) reloadPostgresAfterGatewayFailureLocked() {
	if a.store.db == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := a.store.loadPostgres(ctx); err != nil {
		log.Printf("reload PostgreSQL state after gateway persistence failure: %v", err)
	}
}

func writeGatewayReservationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errGatewayBalanceConflict):
		writeErr(w, http.StatusPaymentRequired, "token_not_available")
	case errors.Is(err, errGatewayReservationConflict):
		writeErr(w, http.StatusConflict, "request_in_progress")
	case errors.Is(err, errGatewayUserUnavailable):
		writeErr(w, http.StatusUnauthorized, "login_failed")
	default:
		writeErr(w, http.StatusInternalServerError, "state_save_failed")
	}
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
	for _, key := range a.store.state.APIKeys {
		if key.Status != statusActive {
			continue
		}
		if strings.TrimSpace(key.KeyCipher) == "" {
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

func (a *App) prioritizeGatewaySessionRouteLocked(userID, sessionID, preferredUpstreamID string, routes []gatewayRoute) []gatewayRoute {
	if sessionID == "" || len(routes) < 2 {
		return routes
	}
	upstreamID := preferredUpstreamID
	if upstreamID == "" {
		upstreamID = a.gatewaySessionUpstreamLocked(userID, sessionID, time.Now().UTC())
	}
	if upstreamID == "" {
		return routes
	}
	return prioritizeGatewayRoute(routes, upstreamID)
}

func (a *App) prioritizeGatewayUserRouteLocked(userID string, routes []gatewayRoute, now time.Time) []gatewayRoute {
	if len(routes) < 2 {
		return routes
	}
	return prioritizeGatewayRoute(routes, a.gatewayUserAssignedUpstreamLocked(userID, now))
}

func prioritizeGatewayRoute(routes []gatewayRoute, upstreamID string) []gatewayRoute {
	if upstreamID == "" || len(routes) < 2 {
		return routes
	}
	idx := -1
	for i, route := range routes {
		if route.Upstream.ID == upstreamID {
			idx = i
			break
		}
	}
	if idx <= 0 {
		return routes
	}
	prioritized := make([]gatewayRoute, 0, len(routes))
	prioritized = append(prioritized, routes[idx])
	prioritized = append(prioritized, routes[:idx]...)
	prioritized = append(prioritized, routes[idx+1:]...)
	return prioritized
}

func (a *App) rememberGatewaySessionRoute(ctx context.Context, userID, sessionID, upstreamID string) {
	if sessionID == "" || upstreamID == "" {
		return
	}
	a.store.mu.Lock()
	now := time.Now().UTC()
	sessionKey := gatewaySessionKey(sessionID)
	for i := range a.store.state.GatewaySessions {
		session := &a.store.state.GatewaySessions[i]
		if session.UserID == userID && session.SessionKey == sessionKey {
			session.UpstreamAccountID = upstreamID
			session.ExpiresAt = now.Add(gatewaySessionRouteTTL)
			session.UpdatedAt = now
			a.store.mu.Unlock()
			a.rememberGatewayRuntimeSessionRoute(ctx, userID, sessionID, upstreamID)
			return
		}
	}
	a.store.state.GatewaySessions = append(a.store.state.GatewaySessions, GatewaySession{
		UserID:            userID,
		SessionKey:        sessionKey,
		UpstreamAccountID: upstreamID,
		ExpiresAt:         now.Add(gatewaySessionRouteTTL),
		UpdatedAt:         now,
	})
	a.store.mu.Unlock()
	a.rememberGatewayRuntimeSessionRoute(ctx, userID, sessionID, upstreamID)
}

func (a *App) setGatewayRouteActive(upstreamID, userID string, active bool) {
	if upstreamID == "" || userID == "" {
		return
	}
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	if a.gatewayActive == nil {
		a.gatewayActive = map[string]map[string]int{}
	}
	users := a.gatewayActive[upstreamID]
	if active {
		if users == nil {
			users = map[string]int{}
			a.gatewayActive[upstreamID] = users
		}
		users[userID]++
		return
	}
	if users == nil {
		return
	}
	if users[userID] <= 1 {
		delete(users, userID)
	} else {
		users[userID]--
	}
	if len(users) == 0 {
		delete(a.gatewayActive, upstreamID)
	}
}

func (a *App) gatewaySessionUpstreamLocked(userID, sessionID string, now time.Time) string {
	sessionKey := gatewaySessionKey(sessionID)
	for i := 0; i < len(a.store.state.GatewaySessions); {
		session := a.store.state.GatewaySessions[i]
		if !session.ExpiresAt.After(now) {
			a.store.state.GatewaySessions = append(a.store.state.GatewaySessions[:i], a.store.state.GatewaySessions[i+1:]...)
			continue
		}
		if session.UserID == userID && session.SessionKey == sessionKey {
			return session.UpstreamAccountID
		}
		i++
	}
	return ""
}

func gatewaySessionKey(sessionID string) string {
	return hashString(strings.TrimSpace(sessionID))
}

func (a *App) publicUserLocked(user User) map[string]any {
	now := time.Now().UTC()
	out := map[string]any{
		"id": user.ID, "account": user.Account, "status": user.Status, "tokenBalance": user.TokenBalance,
		"recentRechargeStatus": a.recentRechargeStatusLocked(user.ID), "lastLoginAt": user.LastLoginAt, "createdAt": user.CreatedAt,
	}
	if lastSeenAt, online := a.clientPresenceSnapshotLocked(user.ID, now); online {
		out["clientOnline"] = true
		out["clientOnlineLastSeenAt"] = lastSeenAt
	} else {
		out["clientOnline"] = false
	}
	if runtime, active := a.clientRuntimeSnapshotLocked(user.ID, now); active {
		out["clientActive"] = true
		out["clientLastSeenAt"] = runtime.ExpiresAt.Add(-gatewayClientLeaseTTL)
		out["desktopVersion"] = runtime.DesktopVersion
		out["codexVersion"] = runtime.CodexVersion
	} else {
		out["clientActive"] = false
	}
	assignedUpstreamID := a.gatewayUserActiveUpstreamLocked(user.ID, now)
	out["gatewayActive"] = assignedUpstreamID != ""
	if assignedUpstreamID != "" {
		upstream := a.upstreamByID(assignedUpstreamID)
		out["assignedUpstreamAccount"] = map[string]any{
			"id": upstream.ID, "name": upstream.Name, "email": upstream.Email,
		}
	}
	return out
}

func (a *App) clientPresenceSnapshotLocked(userID string, now time.Time) (time.Time, bool) {
	devices := a.clientPresence[userID]
	var latestExpiry time.Time
	for deviceID, expiresAt := range devices {
		if !expiresAt.After(now) {
			delete(devices, deviceID)
			continue
		}
		if latestExpiry.IsZero() || expiresAt.After(latestExpiry) {
			latestExpiry = expiresAt
		}
	}
	if len(devices) == 0 {
		delete(a.clientPresence, userID)
	}
	if latestExpiry.IsZero() {
		return time.Time{}, false
	}
	return latestExpiry.Add(-clientPresenceTTL), true
}

func (a *App) clientRuntimeSnapshotLocked(userID string, now time.Time) (clientRuntimeLease, bool) {
	devices := a.clientRuntimes[userID]
	var latest clientRuntimeLease
	for deviceID, runtime := range devices {
		if !runtime.ExpiresAt.After(now) {
			delete(devices, deviceID)
			continue
		}
		if latest.ExpiresAt.IsZero() || runtime.ExpiresAt.After(latest.ExpiresAt) {
			latest = runtime
		}
	}
	if len(devices) == 0 {
		delete(a.clientRuntimes, userID)
	}
	return latest, !latest.ExpiresAt.IsZero()
}

func (a *App) activeClientAccountsLocked(now time.Time) (active, unassigned []string) {
	for _, user := range a.store.state.Users {
		if _, ok := a.clientRuntimeSnapshotLocked(user.ID, now); !ok {
			continue
		}
		active = append(active, user.Account)
		if !a.gatewayUserAssignedLocked(user.ID, now) {
			unassigned = append(unassigned, user.Account)
		}
	}
	sort.Strings(active)
	sort.Strings(unassigned)
	return active, unassigned
}

func (a *App) onlineClientAccountsLocked(now time.Time) []string {
	online := make([]string, 0)
	for _, user := range a.store.state.Users {
		if _, ok := a.clientPresenceSnapshotLocked(user.ID, now); ok {
			online = append(online, user.Account)
		}
	}
	sort.Strings(online)
	return online
}

func (a *App) gatewayUserActiveUpstreamLocked(userID string, now time.Time) string {
	if assigned := a.gatewayUserAssignedUpstreamLocked(userID, now); assigned != "" {
		return assigned
	}
	for _, upstream := range a.store.state.UpstreamAccounts {
		if a.gatewayActive[upstream.ID][userID] > 0 {
			return upstream.ID
		}
	}
	return ""
}

func (a *App) gatewayUserAssignedLocked(userID string, now time.Time) bool {
	for _, users := range a.gatewayActive {
		if users[userID] > 0 {
			return true
		}
	}
	for upstreamID, users := range a.gatewayLeases {
		for candidate, expiresAt := range users {
			if !expiresAt.After(now) {
				delete(users, candidate)
				continue
			}
			if candidate == userID {
				return true
			}
		}
		if len(users) == 0 {
			delete(a.gatewayLeases, upstreamID)
		}
	}
	return false
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
	upstream := a.upstreamByID(rec.UpstreamAccountID)
	userAccount := user.Account
	if userAccount == "" {
		userAccount = rec.UserID
	}
	upstreamAccount := rec.UpstreamAccountID
	if upstream.ID != "" {
		upstreamAccount = usageAnalyticsAccountLabel(upstream)
	}
	return map[string]any{
		"id":                rec.ID,
		"userId":            rec.UserID,
		"userAccount":       userAccount,
		"upstreamAccountId": rec.UpstreamAccountID,
		"upstreamAccount":   upstreamAccount,
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

func (a *App) clientAccessKeyIndex(id string) int {
	if strings.TrimSpace(id) == "" {
		return -1
	}
	for i := range a.store.state.ClientAccessKeys {
		if a.store.state.ClientAccessKeys[i].ID == id {
			return i
		}
	}
	return -1
}

func (a *App) apiKeyReferencesUpstreamLocked(upstreamID string) bool {
	for _, key := range a.store.state.APIKeys {
		if key.UpstreamAccountID == upstreamID {
			return true
		}
	}
	return false
}

func publicAdmin(admin Admin) map[string]any {
	return map[string]any{"id": admin.ID, "account": admin.Account, "mustChangePassword": admin.MustChangePassword, "createdAt": admin.CreatedAt}
}

func upstreamIsAvailable(up UpstreamAccount) bool {
	return up.Status == statusActive && up.AuthorizationStatus == upstreamAuthAuthorized && strings.TrimSpace(up.AccessTokenCipher) != "" && upstreamAccountIsAvailable(up)
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

func upstreamCredentialStatus(up UpstreamAccount) string {
	if up.AuthorizationStatus != "" && up.AuthorizationStatus != upstreamAuthAuthorized {
		return up.AuthorizationStatus
	}
	if up.ExpiresAt != nil && !up.ExpiresAt.After(time.Now().UTC()) {
		return "token_expired"
	}
	if strings.TrimSpace(up.RefreshTokenCipher) == "" {
		return "short_lived_auth"
	}
	return "refreshable"
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
	checkStatus, checkFailureReason := upstreamStoredCheckState(up)
	out := map[string]any{
		"id":                        up.ID,
		"name":                      up.Name,
		"group":                     up.Group,
		"credentialType":            up.CredentialType,
		"sourceType":                up.SourceType,
		"authorizationStatus":       up.AuthorizationStatus,
		"hasPassword":               strings.TrimSpace(up.PasswordCipher) != "",
		"lastAuthorizationError":    up.LastAuthorizationError,
		"tokenType":                 up.TokenType,
		"chatgptAccountId":          up.ChatGPTAccountID,
		"expiresAt":                 up.ExpiresAt,
		"email":                     up.Email,
		"subscriptionTier":          up.SubscriptionTier,
		"entitlementStatus":         up.EntitlementStatus,
		"credentialStatus":          upstreamCredentialStatus(up),
		"enabled":                   up.Status == statusActive,
		"availabilityStatus":        upstreamAvailabilityStatus(up),
		"usageTokens":               up.UsageTokens,
		"rateLimitUsedPercent":      up.RateLimitUsedPercent,
		"rateLimitRemainingPercent": upstreamRateLimitRemainingPercent(up),
		"rateLimitResetsAt":         up.RateLimitResetsAt,
		"creditBalance":             up.CreditBalance,
		"creditBalanceLabel":        up.CreditBalanceLabel,
		"lastCheckedAt":             up.LastCheckedAt,
		"checkStatus":               checkStatus,
		"createdAt":                 up.CreatedAt,
	}
	if checkFailureReason != "" {
		out["checkFailureReason"] = checkFailureReason
	}
	return out
}

func (a *App) publicAdminUpstreamLocked(up UpstreamAccount) map[string]any {
	out := publicUpstream(up)
	if a.upstreamCheckInProgress(up.ID) {
		out["checkStatus"] = "checking"
		delete(out, "checkFailureReason")
	}
	out["remark"] = up.Remark
	activeUserIDs := make(map[string]struct{})
	for userID, count := range a.gatewayActive[up.ID] {
		if count <= 0 {
			continue
		}
		activeUserIDs[userID] = struct{}{}
	}
	now := time.Now().UTC()
	if leases := a.gatewayLeases[up.ID]; leases != nil {
		for userID, expiresAt := range leases {
			if expiresAt.After(now) {
				activeUserIDs[userID] = struct{}{}
				continue
			}
			delete(leases, userID)
		}
		if len(leases) == 0 {
			delete(a.gatewayLeases, up.ID)
		}
	}

	start, end := utcDayBounds(time.Now())
	var routedUsageTokens int64
	var latest UsageRecord
	for _, rec := range a.store.state.UsageRecords {
		if rec.UpstreamAccountID != up.ID {
			continue
		}
		if !rec.CreatedAt.Before(start) && rec.CreatedAt.Before(end) {
			routedUsageTokens += rec.TotalTokens
		}
		if latest.ID == "" || rec.CreatedAt.After(latest.CreatedAt) {
			latest = rec
		}
	}

	activeAccounts := make([]string, 0, len(activeUserIDs))
	for userID := range activeUserIDs {
		account := a.userByID(userID).Account
		if account == "" {
			account = userID
		}
		activeAccounts = append(activeAccounts, account)
	}
	sort.Strings(activeAccounts)
	out["activeUserAccounts"] = activeAccounts
	out["activeUserCount"] = len(activeAccounts)
	limit := a.upstreamUserLimit
	if limit <= 0 {
		limit = defaultGatewayUpstreamUserLimit
	}
	out["activeUserLimit"] = limit
	out["routedUsageTokens"] = routedUsageTokens
	if latest.ID != "" {
		lastUsedFrom := latest.CreatedAt
		if latest.SessionID != "" {
			for _, rec := range a.store.state.UsageRecords {
				if rec.UpstreamAccountID == latest.UpstreamAccountID && rec.UserID == latest.UserID && rec.SessionID == latest.SessionID && rec.CreatedAt.Before(lastUsedFrom) {
					lastUsedFrom = rec.CreatedAt
				}
			}
		}
		account := a.userByID(latest.UserID).Account
		if account == "" {
			account = latest.UserID
		}
		out["lastUserAccount"] = account
		out["lastUsedFrom"] = lastUsedFrom
		out["lastUsedAt"] = latest.CreatedAt
	}
	return out
}

func (a *App) publicAPIKeyLocked(key APIKey) map[string]any {
	up := a.upstreamByID(key.UpstreamAccountID)
	routeAvailable := false
	if up.ID != "" {
		routeAvailable = key.Status == statusActive && upstreamIsAvailable(up)
	}
	out := map[string]any{
		"id":                  key.ID,
		"keyPreview":          apiKeyPreview(key),
		"upstreamAccountId":   key.UpstreamAccountID,
		"upstreamAccountName": up.Name,
		"status":              key.Status,
		"routeAvailable":      routeAvailable,
		"lastUsedAt":          key.LastUsedAt,
		"createdAt":           key.CreatedAt,
	}
	if up.ID != "" {
		out["upstream"] = publicUpstream(up)
	}
	return out
}

func apiKeyPreview(key APIKey) string {
	if strings.TrimSpace(key.PublicPrefix) == "" {
		return key.ID
	}
	return key.PublicPrefix + "..."
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

func (a *App) encryptOptional(plain string) (string, error) {
	if strings.TrimSpace(plain) == "" {
		return "", nil
	}
	return a.encrypt(plain)
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

func gatewayPositiveIntFromEnv(raw string, fallback int, name string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return value, nil
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

var errGatewayBodyTooLarge = errors.New("gateway_request_body_too_large")

type gatewayBodyReadError struct {
	Status int
	Code   string
	Cause  error
}

func readGatewayRequestBody(r *http.Request) ([]byte, *gatewayBodyReadError) {
	return readGatewayRequestBodyWithinLimits(r, maxGatewayEncodedBodyBytes, maxGatewayDecodedBodyBytes)
}

func readGatewayRequestBodyWithinLimits(r *http.Request, encodedLimit, decodedLimit int64) ([]byte, *gatewayBodyReadError) {
	prepared, bodyErr := prepareGatewayRequestWithinLimits(r, encodedLimit, decodedLimit)
	if bodyErr != nil {
		return nil, bodyErr
	}
	defer prepared.Close()
	body, err := prepared.Bytes()
	if err != nil {
		return nil, gatewayStorageError(err)
	}
	return body, nil
}

func readBodyWithinLimit(r io.Reader, limit int64) ([]byte, error) {
	limited := &io.LimitedReader{R: r, N: limit + 1}
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, errGatewayBodyTooLarge
	}
	return body, nil
}

func classifyGatewayBodyReadError(err error) *gatewayBodyReadError {
	if errors.Is(err, errGatewayBodyTooLarge) {
		return &gatewayBodyReadError{Status: http.StatusRequestEntityTooLarge, Code: "request_too_large", Cause: err}
	}
	return &gatewayBodyReadError{Status: http.StatusBadRequest, Code: "invalid_json", Cause: err}
}

func logGatewayBodyError(r *http.Request, bodyErr *gatewayBodyReadError) {
	if r == nil || bodyErr == nil {
		return
	}
	log.Printf(
		"gateway request body rejected: path=%q content_encoding=%q content_length=%d status=%d code=%s cause=%v",
		r.URL.Path,
		strings.TrimSpace(r.Header.Get("Content-Encoding")),
		r.ContentLength,
		bodyErr.Status,
		bodyErr.Code,
		bodyErr.Cause,
	)
}

func readLimitedBody(w http.ResponseWriter, r io.Reader, limit int64) ([]byte, bool) {
	body, err := readBodyWithinLimit(r, limit)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_json")
		return nil, false
	}
	return body, true
}

type adminUpstreamRequest struct {
	Name              string     `json:"name"`
	Group             string     `json:"group"`
	Remark            string     `json:"remark"`
	CredentialType    string     `json:"credentialType"`
	SourceType        string     `json:"sourceType"`
	AccessToken       string     `json:"accessToken"`
	RefreshToken      string     `json:"refreshToken"`
	Password          string     `json:"password"`
	AuthJSONRaw       string     `json:"-"`
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
		"remark":            {},
		"credentialType":    {},
		"sourceType":        {},
		"accessToken":       {},
		"refreshToken":      {},
		"password":          {},
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
	if len([]rune(strings.TrimSpace(req.Remark))) > maxUpstreamRemarkRunes {
		writeErr(w, http.StatusBadRequest, "invalid_upstream_request")
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
	if len(bytes.TrimSpace(body)) == 0 {
		writeErr(w, http.StatusBadRequest, "invalid_upstream_import_request")
		return nil, false
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var payload any
	if err := dec.Decode(&payload); err != nil {
		payload = string(body)
	} else {
		var extra any
		if err := dec.Decode(&extra); err != io.EOF {
			writeErr(w, http.StatusBadRequest, "invalid_json")
			return nil, false
		}
	}
	reqs, err := parseAdminUpstreamImportPayload(payload)
	if err != nil || len(reqs) == 0 {
		writeErr(w, http.StatusBadRequest, "invalid_upstream_import_request")
		return nil, false
	}
	return reqs, true
}

func parseAdminUpstreamImportPayload(payload any) ([]adminUpstreamRequest, error) {
	return parseAdminUpstreamImportPayloadWithDefault(payload, "")
}

func parseAdminUpstreamImportPayloadWithDefault(payload any, inheritedGroup string) ([]adminUpstreamRequest, error) {
	if accounts, ok := payload.([]any); ok {
		return parseAdminUpstreamImportAccounts(accounts, inheritedGroup)
	}
	if text, ok := payload.(string); ok {
		return parseAdminUpstreamImportContent(text, inheritedGroup)
	}
	obj, ok := payload.(map[string]any)
	if !ok {
		return nil, errors.New("upstream_import_must_be_object")
	}
	defaultGroup := importStringField(obj, "group", "account_group", "accountGroup")
	if defaultGroup == "" {
		defaultGroup = inheritedGroup
	}
	if content := importStringField(obj, "content", "text", "raw"); content != "" {
		return parseAdminUpstreamImportContent(content, defaultGroup)
	}
	if rawContents, ok := importValueField(obj, "contents", "files"); ok {
		contents, ok := rawContents.([]any)
		if !ok {
			return nil, errors.New("upstream_import_contents_must_be_array")
		}
		requests := make([]adminUpstreamRequest, 0, len(contents))
		for _, content := range contents {
			parsed, err := parseAdminUpstreamImportPayloadWithDefault(content, defaultGroup)
			if err != nil {
				return nil, err
			}
			requests = append(requests, parsed...)
		}
		if len(requests) == 0 {
			return nil, errors.New("upstream_import_contents_empty")
		}
		return requests, nil
	}
	if rawAccounts, ok := importValueField(obj, "accounts", "items", "sessions"); ok {
		accounts, ok := rawAccounts.([]any)
		if !ok {
			return nil, errors.New("upstream_import_accounts_must_be_array")
		}
		return parseAdminUpstreamImportAccounts(accounts, defaultGroup)
	}
	if rawData, ok := importValueField(obj, "data"); ok {
		if accounts, ok := rawData.([]any); ok {
			return parseAdminUpstreamImportAccounts(accounts, defaultGroup)
		}
		return parseAdminUpstreamImportPayloadWithDefault(rawData, defaultGroup)
	}
	req, err := parseAdminUpstreamImportAccount(obj, 1, defaultGroup)
	if err != nil {
		return nil, err
	}
	return []adminUpstreamRequest{req}, nil
}

func parseAdminUpstreamImportContent(content, defaultGroup string) ([]adminUpstreamRequest, error) {
	text := strings.TrimSpace(content)
	if text == "" {
		return nil, errors.New("upstream_import_content_empty")
	}
	var payload any
	dec := json.NewDecoder(strings.NewReader(text))
	dec.UseNumber()
	if err := dec.Decode(&payload); err == nil {
		var extra any
		if err := dec.Decode(&extra); err == io.EOF {
			return parseAdminUpstreamImportPayloadWithDefault(payload, defaultGroup)
		}
	}
	if items, ok := parseEmailPasswordCSV(text); ok {
		return parseAdminUpstreamImportAccounts(items, defaultGroup)
	}
	lines := strings.Split(text, "\n")
	items := make([]any, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if isEmailPasswordHeader(line) {
			continue
		}
		var item any
		dec := json.NewDecoder(strings.NewReader(line))
		dec.UseNumber()
		if err := dec.Decode(&item); err == nil {
			var extra any
			if err := dec.Decode(&extra); err == io.EOF {
				if nested, ok := item.([]any); ok {
					items = append(items, nested...)
				} else {
					items = append(items, item)
				}
				continue
			}
		}
		items = append(items, line)
	}
	return parseAdminUpstreamImportAccounts(items, defaultGroup)
}

func parseEmailPasswordCSV(text string) ([]any, bool) {
	reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(text, "\ufeff")))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true
	records, err := reader.ReadAll()
	if err != nil || len(records) == 0 {
		return nil, false
	}
	start := 0
	if len(records[0]) >= 2 && isEmailPasswordHeader(strings.Join(records[0][:2], ",")) {
		start = 1
	}
	items := make([]any, 0, len(records)-start)
	for _, record := range records[start:] {
		if len(record) < 2 {
			return nil, false
		}
		email := strings.ToLower(strings.TrimSpace(record[0]))
		password := record[1]
		if len(record) > 2 {
			password = strings.Join(record[1:], ",")
		}
		if !looksLikeEmail(email) || password == "" {
			return nil, false
		}
		items = append(items, map[string]any{"email": email, "password": password, "sourceType": "email_password"})
	}
	return items, len(items) > 0
}

func parseAdminUpstreamImportAccounts(accounts []any, defaultGroup string) ([]adminUpstreamRequest, error) {
	if len(accounts) == 0 {
		return nil, errors.New("upstream_import_accounts_empty")
	}
	reqs := make([]adminUpstreamRequest, 0, len(accounts))
	for i, item := range accounts {
		if !supportedCodexImportAccount(item) {
			continue
		}
		req, err := parseAdminUpstreamImportAccount(item, i+1, defaultGroup)
		if err != nil {
			return nil, err
		}
		reqs = append(reqs, req)
	}
	if len(reqs) == 0 {
		return nil, errors.New("upstream_import_no_supported_codex_accounts")
	}
	return reqs, nil
}

func supportedCodexImportAccount(item any) bool {
	obj, ok := item.(map[string]any)
	if !ok {
		return true
	}
	platform := strings.ToLower(strings.TrimSpace(importStringField(obj, "platform", "provider")))
	if platform == "" {
		return true
	}
	return strings.Contains(platform, "openai") || strings.Contains(platform, "codex") || strings.Contains(platform, "chatgpt")
}

func parseAdminUpstreamImportAccount(item any, index int, defaultGroup string) (adminUpstreamRequest, error) {
	if token, ok := item.(string); ok {
		if email, password, ok := parseEmailPasswordLine(token); ok {
			item = map[string]any{"email": email, "password": password, "sourceType": "email_password"}
		} else {
			item = map[string]any{"accessToken": token, "sourceType": "access_token"}
		}
	}
	obj, ok := item.(map[string]any)
	if !ok {
		return adminUpstreamRequest{}, errors.New("upstream_import_account_must_be_object")
	}
	if req, handled, err := parseChatGPTSessionImport(obj, index, defaultGroup); handled || err != nil {
		if err != nil {
			return adminUpstreamRequest{}, err
		}
		return normalizeAdminUpstreamRequest(req)
	}
	if req, handled, err := parseCodexAuthJSONImport(obj, index, defaultGroup); handled || err != nil {
		if err != nil {
			return adminUpstreamRequest{}, err
		}
		return normalizeAdminUpstreamRequest(req)
	}
	credentials, hasCredentials := importFirstMapField(obj, "credentials", "auth", "tokens")
	if !hasCredentials {
		credentials = obj
	}
	user, hasUser := importFirstMapField(obj, "user", "profile")
	account, hasAccount := importFirstMapField(obj, "account", "chatgptAccount", "chatgpt_account")
	name := importStringField(obj, "name", "accountName", "account_name")
	if name == "" {
		name = importStringField(credentials, "name", "email")
	}
	if name == "" && hasUser {
		name = importStringField(user, "email", "name", "id")
	}
	if name == "" && hasAccount {
		name = importStringField(account, "email", "name", "id")
	}
	if name == "" {
		name = fmt.Sprintf("Codex account %d", index)
	}
	group := importStringField(obj, "group", "account_group", "accountGroup")
	if group == "" {
		group = defaultGroup
	}
	credentialType := importNestedStringField(credentials, obj, hasCredentials, "credentialType", "credential_type")
	password := importStringFromMaps([]map[string]any{credentials, obj}, "password", "pass", "passwd", "pwd")
	if credentialType == "" && password == "" {
		credentialType = "oauth"
	}
	sourceType := importStringField(obj, "sourceType", "source_type")
	if sourceType == "" && (importStringField(obj, "platform") != "" || importStringField(obj, "type") != "") {
		sourceType = "sub2api"
	}
	if sourceType == "" && password != "" {
		sourceType = "email_password"
	}
	if sourceType == "" {
		sourceType = "token_json"
	}
	expiresAt, err := importNestedExpiresAt(credentials, obj, hasCredentials)
	if err != nil {
		return adminUpstreamRequest{}, err
	}
	req := adminUpstreamRequest{
		Name:              name,
		Group:             group,
		Remark:            importStringField(obj, "remark", "note", "notes", "comment"),
		CredentialType:    credentialType,
		SourceType:        sourceType,
		AccessToken:       importStringFromMaps([]map[string]any{credentials, obj}, "access_token", "accessToken", "OPENAI_API_KEY", "openai_api_key", "apiKey", "api_key", "token"),
		RefreshToken:      importNestedStringField(credentials, obj, hasCredentials, "refresh_token", "refreshToken"),
		Password:          password,
		TokenType:         importNestedStringField(credentials, obj, hasCredentials, "token_type", "tokenType"),
		ChatGPTAccountID:  firstNonEmptyText(importStringFromMaps([]map[string]any{account, credentials, obj}, "chatgpt_account_id", "chatgptAccountId", "account_id", "accountId", "id", "organization_id", "organizationId"), importStringFromMaps([]map[string]any{user, credentials, obj}, "chatgpt_user_id", "chatgptUserId", "user_id", "userId", "id")),
		ExpiresAt:         expiresAt,
		Email:             importStringFromMaps([]map[string]any{user, account, credentials, obj}, "email", "mail", "username", "login", "account"),
		SubscriptionTier:  importStringFromMaps([]map[string]any{account, credentials, obj}, "plan_type", "planType", "subscription_tier", "subscriptionTier"),
		EntitlementStatus: importNestedStringField(credentials, obj, hasCredentials, "entitlement_status", "entitlementStatus"),
	}
	if req.TokenType == "" {
		req.TokenType = "Bearer"
	}
	return normalizeAdminUpstreamRequest(req)
}

func parseChatGPTSessionImport(obj map[string]any, index int, defaultGroup string) (adminUpstreamRequest, bool, error) {
	accessToken := strings.TrimSpace(importStringField(obj, "accessToken", "access_token"))
	user, hasUser := importFirstMapField(obj, "user", "profile")
	account, hasAccount := importFirstMapField(obj, "account", "chatgptAccount", "chatgpt_account")
	rawExpiry, hasExpiry := importValueField(obj, "expires", "expiresAt", "expires_at")
	if accessToken == "" || !hasExpiry || (!hasUser && !hasAccount) {
		return adminUpstreamRequest{}, false, nil
	}

	sessionExpiry, err := parseImportExpiresAt(rawExpiry)
	if err != nil {
		return adminUpstreamRequest{}, true, errors.New("upstream_chatgpt_session_expires_invalid")
	}
	accessClaims := jwtClaimsFromToken(accessToken)
	accountID := firstNonEmptyText(
		importStringField(account, "id", "account_id", "accountId", "chatgpt_account_id", "chatgptAccountId"),
		importStringField(obj, "account_id", "accountId", "chatgpt_account_id", "chatgptAccountId"),
		claimNestedString(accessClaims, "https://api.openai.com/auth", "chatgpt_account_id", "chatgptAccountId", "chatgpt_account_user_id", "chatgpt_user_id", "user_id"),
		firstStringField(accessClaims, "chatgpt_account_id", "chatgptAccountId", "user_id", "userId"),
		importStringField(user, "id", "user_id", "userId"),
	)
	if accountID == "" {
		return adminUpstreamRequest{}, true, errors.New("upstream_chatgpt_session_account_id_missing")
	}
	email := strings.ToLower(firstNonEmptyText(
		importStringField(user, "email", "mail"),
		importStringField(obj, "email", "mail"),
		claimNestedString(accessClaims, "https://api.openai.com/profile", "email"),
		firstStringField(accessClaims, "email"),
	))
	planType := firstNonEmptyText(
		importStringField(account, "planType", "plan_type", "subscriptionTier", "subscription_tier"),
		importStringField(obj, "planType", "plan_type", "subscriptionTier", "subscription_tier"),
		claimNestedString(accessClaims, "https://api.openai.com/auth", "chatgpt_plan_type", "plan_type"),
	)
	expiresAt := jwtTimeClaim(accessClaims, "exp")
	if sessionExpiry != nil && (expiresAt == nil || sessionExpiry.Before(*expiresAt)) {
		expiresAt = sessionExpiry
	}

	minimalAuthJSON, err := json.Marshal(map[string]any{
		"auth_mode": "chatgpt",
		"tokens": map[string]any{
			"access_token":  accessToken,
			"refresh_token": "",
			"account_id":    accountID,
		},
	})
	if err != nil {
		return adminUpstreamRequest{}, true, errors.New("upstream_chatgpt_session_invalid")
	}
	entitlementStatus := "short_lived_auth"
	if expiresAt != nil && !expiresAt.After(time.Now().UTC()) {
		entitlementStatus = "token_expired"
	}
	name := firstNonEmptyText(email, importStringField(user, "name"), accountID)
	if name == "" {
		name = fmt.Sprintf("ChatGPT session %d", index)
	}

	return adminUpstreamRequest{
		Name:              name,
		Group:             defaultGroup,
		Remark:            importStringField(obj, "remark", "note", "notes", "comment"),
		CredentialType:    "oauth",
		SourceType:        "chatgpt_session",
		AccessToken:       accessToken,
		AuthJSONRaw:       string(minimalAuthJSON),
		TokenType:         "Bearer",
		ChatGPTAccountID:  accountID,
		ExpiresAt:         expiresAt,
		Email:             email,
		SubscriptionTier:  planType,
		EntitlementStatus: entitlementStatus,
	}, true, nil
}

func parseCodexAuthJSONImport(obj map[string]any, index int, defaultGroup string) (adminUpstreamRequest, bool, error) {
	authMode := strings.TrimSpace(importStringField(obj, "auth_mode", "authMode"))
	tokens, hasTokens := importFirstMapField(obj, "tokens")
	accessToken := importStringField(tokens, "access_token", "accessToken")
	idToken := importStringField(tokens, "id_token", "idToken")
	refreshToken := importStringField(tokens, "refresh_token", "refreshToken")
	looksLikeCodexAuth := hasTokens && (accessToken != "" || idToken != "" || authMode != "")
	if !looksLikeCodexAuth {
		if strings.EqualFold(authMode, "apikey") || importStringField(obj, "OPENAI_API_KEY", "openai_api_key") != "" {
			return adminUpstreamRequest{}, true, errors.New("upstream_codex_api_key_auth_unsupported")
		}
		return adminUpstreamRequest{}, false, nil
	}
	if authMode != "" && !strings.EqualFold(authMode, "chatgpt") {
		return adminUpstreamRequest{}, true, errors.New("upstream_codex_auth_mode_unsupported")
	}

	accessClaims := jwtClaimsFromToken(accessToken)
	idClaims := jwtClaimsFromToken(idToken)
	accountID := firstNonEmptyText(
		importStringField(tokens, "account_id", "accountId", "chatgpt_account_id", "chatgptAccountId"),
		claimNestedString(accessClaims, "https://api.openai.com/auth", "chatgpt_account_id", "chatgptAccountId", "chatgpt_account_user_id", "chatgpt_user_id", "user_id"),
		claimNestedString(idClaims, "https://api.openai.com/auth", "chatgpt_account_id", "chatgptAccountId", "chatgpt_account_user_id", "chatgpt_user_id", "user_id"),
		firstStringField(accessClaims, "chatgpt_account_id", "chatgptAccountId", "user_id", "userId"),
	)
	email := firstNonEmptyText(
		importStringField(obj, "email"),
		claimNestedString(accessClaims, "https://api.openai.com/profile", "email"),
		firstStringField(idClaims, "email"),
		claimNestedString(idClaims, "https://api.openai.com/profile", "email"),
	)
	planType := firstNonEmptyText(
		importStringField(obj, "plan_type", "planType", "subscription_tier", "subscriptionTier"),
		claimNestedString(accessClaims, "https://api.openai.com/auth", "chatgpt_plan_type", "plan_type"),
		claimNestedString(idClaims, "https://api.openai.com/auth", "chatgpt_plan_type", "plan_type"),
	)
	name := firstNonEmptyText(importStringField(obj, "name", "accountName", "account_name"), email)
	if name == "" {
		name = fmt.Sprintf("Codex account %d", index)
	}
	group := importStringField(obj, "group", "account_group", "accountGroup")
	if group == "" {
		group = defaultGroup
	}
	authJSON, err := json.Marshal(obj)
	if err != nil {
		return adminUpstreamRequest{}, true, errors.New("upstream_codex_auth_json_invalid")
	}
	entitlementStatus := importStringField(obj, "entitlement_status", "entitlementStatus")
	if strings.TrimSpace(refreshToken) == "" && entitlementStatus == "" {
		entitlementStatus = "short_lived_auth"
	}
	expiresAt := jwtTimeClaim(accessClaims, "exp")
	if expiresAt == nil {
		if value, ok := importValueField(obj, "expires_at", "expiresAt", "expires"); ok {
			expiresAt, err = parseImportExpiresAt(value)
			if err != nil {
				return adminUpstreamRequest{}, true, err
			}
		}
	}
	if expiresAt != nil && !expiresAt.After(time.Now().UTC()) {
		entitlementStatus = "token_expired"
	}
	return adminUpstreamRequest{
		Name:              name,
		Group:             group,
		Remark:            importStringField(obj, "remark", "note", "notes", "comment"),
		CredentialType:    "oauth",
		SourceType:        valueOr(importStringField(obj, "sourceType", "source_type"), "codex_auth_json"),
		AccessToken:       accessToken,
		RefreshToken:      refreshToken,
		AuthJSONRaw:       string(authJSON),
		TokenType:         "Bearer",
		ChatGPTAccountID:  accountID,
		ExpiresAt:         expiresAt,
		Email:             email,
		SubscriptionTier:  planType,
		EntitlementStatus: entitlementStatus,
	}, true, nil
}

func refreshCodexAuthJSON(ctx context.Context, authJSONRaw string) (adminUpstreamRequest, error) {
	var obj map[string]any
	dec := json.NewDecoder(strings.NewReader(authJSONRaw))
	dec.UseNumber()
	if err := dec.Decode(&obj); err != nil {
		return adminUpstreamRequest{}, errors.New("upstream_codex_auth_json_invalid")
	}
	tokens, ok := importFirstMapField(obj, "tokens")
	if !ok {
		return adminUpstreamRequest{}, errors.New("upstream_codex_auth_json_missing_tokens")
	}
	refreshToken := strings.TrimSpace(importStringField(tokens, "refresh_token", "refreshToken"))
	if refreshToken == "" {
		return adminUpstreamRequest{}, errors.New("upstream_refresh_token_missing")
	}
	authDir, err := os.MkdirTemp("", "codexppp-refresh-auth-*")
	if err != nil {
		return adminUpstreamRequest{}, errors.New("upstream_refresh_auth_home_failed")
	}
	defer os.RemoveAll(authDir)
	if err := writeCodexDeviceAuthConfig(authDir); err != nil {
		return adminUpstreamRequest{}, errors.New("upstream_refresh_auth_home_failed")
	}
	if err := os.WriteFile(filepath.Join(authDir, "auth.json"), []byte(authJSONRaw), 0600); err != nil {
		return adminUpstreamRequest{}, errors.New("upstream_refresh_auth_home_failed")
	}
	cmdName := valueOr(os.Getenv("CODEXPPP_CODEX_COMMAND"), "codex")
	cmd := exec.CommandContext(ctx, cmdName, "app-server", "--listen", "stdio://")
	cmd.Env = append(os.Environ(), "CODEX_HOME="+authDir)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return adminUpstreamRequest{}, errors.New("codex_app_server_start_failed")
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return adminUpstreamRequest{}, errors.New("codex_app_server_start_failed")
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return adminUpstreamRequest{}, errors.New("codex_app_server_start_failed")
	}
	defer func() {
		_ = stdin.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}()

	dec = json.NewDecoder(stdout)
	enc := json.NewEncoder(stdin)
	send := func(message map[string]any) error {
		if err := enc.Encode(message); err != nil {
			return errors.New("codex_app_server_write_failed")
		}
		return nil
	}
	readResult := func(id int) (map[string]any, error) {
		for {
			var msg map[string]any
			if err := dec.Decode(&msg); err != nil {
				if ctx.Err() != nil {
					return nil, errors.New("upstream_refresh_failed")
				}
				return nil, errors.New("codex_app_server_read_failed")
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
			respondToCodexDeviceServerRequest(enc, msg)
		}
	}
	if err := send(map[string]any{"id": 1, "method": "initialize", "params": map[string]any{"clientInfo": map[string]any{"name": "Codex+++", "version": "0.1.0"}, "capabilities": map[string]any{}}}); err != nil {
		return adminUpstreamRequest{}, err
	}
	if _, err := readResult(1); err != nil {
		return adminUpstreamRequest{}, err
	}
	if err := send(map[string]any{"method": "initialized"}); err != nil {
		return adminUpstreamRequest{}, err
	}
	if err := send(map[string]any{"id": 2, "method": "account/read", "params": map[string]any{"refreshToken": true}}); err != nil {
		return adminUpstreamRequest{}, err
	}
	if _, err := readResult(2); err != nil {
		return adminUpstreamRequest{}, err
	}
	refreshedRaw, err := readCodexDeviceAuthJSON(ctx, authDir)
	if err != nil {
		return adminUpstreamRequest{}, errors.New("upstream_refresh_response_invalid")
	}
	dec = json.NewDecoder(strings.NewReader(refreshedRaw))
	dec.UseNumber()
	if err := dec.Decode(&obj); err != nil {
		return adminUpstreamRequest{}, errors.New("upstream_refresh_response_invalid")
	}
	tokens, ok = importFirstMapField(obj, "tokens")
	if !ok || importStringField(tokens, "access_token", "accessToken") == "" {
		return adminUpstreamRequest{}, errors.New("upstream_refresh_access_token_missing")
	}
	obj["last_refresh"] = time.Now().UTC().Format(time.RFC3339)
	parsed, handled, err := parseCodexAuthJSONImport(obj, 1, "")
	if err != nil {
		return adminUpstreamRequest{}, err
	}
	if !handled {
		return adminUpstreamRequest{}, errors.New("upstream_codex_auth_json_invalid")
	}
	return parsed, nil
}

func normalizeAdminUpstreamRequest(req adminUpstreamRequest) (adminUpstreamRequest, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Group = strings.TrimSpace(req.Group)
	req.Remark = strings.TrimSpace(req.Remark)
	if len([]rune(req.Remark)) > maxUpstreamRemarkRunes {
		return adminUpstreamRequest{}, errors.New("upstream_remark_too_long")
	}
	req.AccessToken = strings.TrimSpace(req.AccessToken)
	req.RefreshToken = strings.TrimSpace(req.RefreshToken)
	req.AuthJSONRaw = strings.TrimSpace(req.AuthJSONRaw)
	req.CredentialType = strings.TrimSpace(req.CredentialType)
	req.SourceType = strings.TrimSpace(req.SourceType)
	req.TokenType = strings.TrimSpace(req.TokenType)
	req.ChatGPTAccountID = strings.TrimSpace(req.ChatGPTAccountID)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.SubscriptionTier = strings.TrimSpace(req.SubscriptionTier)
	req.EntitlementStatus = strings.TrimSpace(req.EntitlementStatus)
	if req.ChatGPTAccountID == "" {
		req.ChatGPTAccountID = chatGPTAccountIDFromAccessToken(req.AccessToken)
	}
	if req.AccessToken == "" {
		if !looksLikeEmail(req.Email) || req.Password == "" {
			return adminUpstreamRequest{}, errors.New("upstream_authorization_material_required")
		}
		if req.Name == "" {
			req.Name = req.Email
		}
		if req.CredentialType != "" && req.CredentialType != "email_password" {
			return adminUpstreamRequest{}, errors.New("upstream_credential_type_unsupported")
		}
		req.CredentialType = "email_password"
		req.SourceType = valueOr(req.SourceType, "email_password")
		req.EntitlementStatus = upstreamAuthPending
		return req, nil
	}
	if req.CredentialType != "" && req.CredentialType != "oauth" {
		return adminUpstreamRequest{}, errors.New("upstream_credential_type_unsupported")
	}
	req.CredentialType = "oauth"
	req.SourceType = valueOr(req.SourceType, "token_json")
	return req, nil
}

func looksLikeEmail(value string) bool {
	value = strings.TrimSpace(value)
	if strings.Count(value, "@") != 1 || strings.ContainsAny(value, " \t\r\n,|:") {
		return false
	}
	at := strings.LastIndexByte(value, '@')
	return at > 0 && at < len(value)-1
}

func parseEmailPasswordLine(line string) (string, string, bool) {
	line = strings.TrimSpace(strings.TrimPrefix(line, "\ufeff"))
	for _, separator := range []string{"----", "\t", "|", ",", ":"} {
		left, right, ok := strings.Cut(line, separator)
		if !ok {
			continue
		}
		email := strings.ToLower(strings.TrimSpace(left))
		password := strings.TrimSpace(right)
		if looksLikeEmail(email) && password != "" {
			return email, password, true
		}
	}
	return "", "", false
}

func isEmailPasswordHeader(line string) bool {
	normalized := strings.ToLower(strings.NewReplacer(" ", "", "\t", ",", "----", ",", "|", ",", ":", ",").Replace(strings.TrimSpace(strings.TrimPrefix(line, "\ufeff"))))
	return normalized == "email,password" || normalized == "邮箱,密码" || normalized == "account,password"
}

func chatGPTAccountIDFromAccessToken(accessToken string) string {
	claims := jwtClaimsFromToken(accessToken)
	if claims == nil {
		return ""
	}
	if id := claimNestedString(claims, "https://api.openai.com/auth", "chatgpt_account_id", "chatgptAccountId", "chatgpt_account_user_id", "chatgpt_user_id", "user_id", "userId", "poid", "organization_id", "organizationId"); id != "" {
		return id
	}
	return firstStringField(claims, "chatgpt_account_id", "chatgptAccountId", "chatgpt_account_user_id", "chatgpt_user_id", "user_id", "userId")
}

func jwtClaimsFromToken(token string) map[string]any {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		payload, err = base64.URLEncoding.DecodeString(parts[1])
	}
	if err != nil {
		return nil
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil
	}
	return claims
}

func jwtTimeClaim(claims map[string]any, key string) *time.Time {
	if claims == nil {
		return nil
	}
	seconds, ok := int64FromAny(claims[key])
	if !ok || seconds <= 0 {
		return nil
	}
	t := time.Unix(seconds, 0).UTC()
	return &t
}

func claimNestedString(claims map[string]any, nestedKey string, keys ...string) string {
	if claims == nil {
		return ""
	}
	nested, ok := claims[nestedKey].(map[string]any)
	if !ok {
		return ""
	}
	return firstStringField(nested, keys...)
}

func importFirstMapField(obj map[string]any, names ...string) (map[string]any, bool) {
	for _, name := range names {
		value, ok := obj[name]
		if !ok {
			continue
		}
		nested, ok := value.(map[string]any)
		if ok {
			return nested, true
		}
	}
	return nil, false
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

func importStringFromMaps(maps []map[string]any, names ...string) string {
	for _, obj := range maps {
		if obj == nil {
			continue
		}
		if value := importStringField(obj, names...); value != "" {
			return value
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
