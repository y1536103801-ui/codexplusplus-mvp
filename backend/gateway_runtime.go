package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const acquireUpstreamLeaseScript = `
local key = KEYS[1]
local now = tonumber(ARGV[1])
local expires = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
local lease = ARGV[4]
redis.call('ZREMRANGEBYSCORE', key, '-inf', now)
if redis.call('ZSCORE', key, lease) then
  redis.call('ZADD', key, expires, lease)
  redis.call('EXPIRE', key, math.max(1, math.ceil((expires - now) / 1000) * 2))
  return 1
end
if redis.call('ZCARD', key) >= limit then
  return 0
end
redis.call('ZADD', key, expires, lease)
redis.call('EXPIRE', key, math.max(1, math.ceil((expires - now) / 1000) * 2))
return 1
`

const renewUpstreamLeaseScript = `
local key = KEYS[1]
local lease = ARGV[1]
local expires = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
if not redis.call('ZSCORE', key, lease) then
  return 0
end
redis.call('ZADD', key, expires, lease)
redis.call('EXPIRE', key, math.max(1, math.ceil((expires - now) / 1000) * 2))
return 1
`

const releaseOwnedLockScript = `
if redis.call('GET', KEYS[1]) == ARGV[1] then
  return redis.call('DEL', KEYS[1])
end
return 0
`

func (r *RedisRateLimiter) SessionRoute(ctx context.Context, key string) (string, error) {
	if r == nil || r.client == nil {
		return "", nil
	}
	value, err := r.client.Get(ctx, "codexppp:gateway:session:"+key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", nil
		}
		return "", err
	}
	return value, nil
}

func (r *RedisRateLimiter) RememberSessionRoute(ctx context.Context, key, upstreamID string, ttl time.Duration) error {
	if r == nil || r.client == nil {
		return nil
	}
	return r.client.Set(ctx, "codexppp:gateway:session:"+key, upstreamID, ttl).Err()
}

func (r *RedisRateLimiter) AcquireUpstream(ctx context.Context, upstreamID, leaseID string, limit int64, ttl time.Duration) (bool, error) {
	if r == nil || r.client == nil || limit <= 0 {
		return true, nil
	}
	now := time.Now().UTC()
	result, err := r.client.Eval(ctx, acquireUpstreamLeaseScript,
		[]string{"codexppp:gateway:upstream:" + upstreamID},
		now.UnixMilli(), now.Add(ttl).UnixMilli(), limit, leaseID,
	).Int64()
	return result == 1, err
}

func (r *RedisRateLimiter) RenewUpstream(ctx context.Context, upstreamID, leaseID string, ttl time.Duration) (bool, error) {
	if r == nil || r.client == nil {
		return true, nil
	}
	now := time.Now().UTC()
	result, err := r.client.Eval(ctx, renewUpstreamLeaseScript,
		[]string{"codexppp:gateway:upstream:" + upstreamID},
		leaseID, now.Add(ttl).UnixMilli(), now.UnixMilli(),
	).Int64()
	return result == 1, err
}

func (r *RedisRateLimiter) ReleaseUpstream(ctx context.Context, upstreamID, leaseID string) error {
	if r == nil || r.client == nil {
		return nil
	}
	return r.client.ZRem(ctx, "codexppp:gateway:upstream:"+upstreamID, leaseID).Err()
}

func (r *RedisRateLimiter) AcquireLock(ctx context.Context, key, owner string, ttl time.Duration) (bool, error) {
	if r == nil || r.client == nil {
		return true, nil
	}
	return r.client.SetNX(ctx, "codexppp:lock:"+key, owner, ttl).Result()
}

func (r *RedisRateLimiter) ReleaseLock(ctx context.Context, key, owner string) error {
	if r == nil || r.client == nil {
		return nil
	}
	return r.client.Eval(ctx, releaseOwnedLockScript, []string{"codexppp:lock:" + key}, owner).Err()
}

func (r *RedisRateLimiter) Ping(ctx context.Context) error {
	if r == nil || r.client == nil {
		return nil
	}
	return r.client.Ping(ctx).Err()
}

func gatewayUpstreamConcurrencyFromEnv(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultGatewayUpstreamConcurrency, nil
	}
	limit, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || limit <= 0 || limit > 1000 {
		return 0, errors.New("CODEXPPP_GATEWAY_UPSTREAM_CONCURRENCY must be an integer between 1 and 1000")
	}
	return limit, nil
}

func gatewayUpstreamUserLimitFromEnv(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultGatewayUpstreamUserLimit, nil
	}
	limit, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || limit <= 0 || limit > 1000 {
		return 0, errors.New("CODEXPPP_GATEWAY_UPSTREAM_USER_LIMIT must be an integer between 1 and 1000")
	}
	return limit, nil
}

func gatewayRuntimeSessionKey(userID, sessionID string) string {
	return hashString(fmt.Sprintf("%s\x00%s", userID, strings.TrimSpace(sessionID)))
}

func (a *App) rememberGatewayRuntimeSessionRoute(ctx context.Context, userID, sessionID, upstreamID string) {
	if a.gatewayRuntime == nil {
		return
	}
	if err := a.gatewayRuntime.RememberSessionRoute(ctx, gatewayRuntimeSessionKey(userID, sessionID), upstreamID, gatewaySessionRouteTTL); err != nil {
		// PostgreSQL/JSON affinity remains authoritative for restart recovery. A
		// Redis failure is surfaced through request admission on the next call.
		return
	}
}

func (a *App) acquireGatewayUpstreamLease(ctx context.Context, upstreamID, userID, requestID string) (string, bool, error) {
	limit := a.upstreamLimit
	if limit <= 0 {
		limit = defaultGatewayUpstreamConcurrency
	}
	leaseID := hashString(a.gatewayInstanceID + "\x00" + requestID + "\x00" + upstreamID + "\x00" + randomToken(8))
	if a.gatewayRuntime != nil {
		acquired, err := a.gatewayRuntime.AcquireUpstream(ctx, upstreamID, leaseID, limit, gatewayUpstreamLeaseTTL)
		if err != nil || !acquired {
			return leaseID, acquired, err
		}
		a.setGatewayRouteActive(upstreamID, userID, true)
		return leaseID, true, nil
	}

	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	if a.gatewayActive == nil {
		a.gatewayActive = map[string]map[string]int{}
	}
	users := a.gatewayActive[upstreamID]
	active := 0
	for _, count := range users {
		active += count
	}
	if int64(active) >= limit {
		return leaseID, false, nil
	}
	if users == nil {
		users = map[string]int{}
		a.gatewayActive[upstreamID] = users
	}
	users[userID]++
	return leaseID, true, nil
}

func (a *App) maintainGatewayUpstreamLease(upstreamID, userID, leaseID string) func() {
	ctx, cancel := context.WithCancel(context.Background())
	if a.gatewayRuntime != nil {
		go func() {
			ticker := time.NewTicker(gatewayUpstreamLeaseRenewInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					renewCtx, renewCancel := context.WithTimeout(context.Background(), 3*time.Second)
					_, _ = a.gatewayRuntime.RenewUpstream(renewCtx, upstreamID, leaseID, gatewayUpstreamLeaseTTL)
					renewCancel()
				}
			}
		}()
	}
	return func() {
		cancel()
		if a.gatewayRuntime != nil {
			releaseCtx, releaseCancel := context.WithTimeout(context.Background(), 3*time.Second)
			_ = a.gatewayRuntime.ReleaseUpstream(releaseCtx, upstreamID, leaseID)
			releaseCancel()
		}
		a.setGatewayRouteActive(upstreamID, userID, false)
	}
}
