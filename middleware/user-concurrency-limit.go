package middleware

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

const userConcurrencyLeaseTTL = time.Minute
const userConcurrencyRenewInterval = 10 * time.Second
const userConcurrencyRedisTimeout = 2 * time.Second

// Redis owns the clock. Each request has its own expiring lease; an old
// release cannot decrement a new request, and a crashed process cannot leak
// a slot permanently. Both scripts must respect the deployment's ACL prefix.
var acquireUserConcurrencyScript = redis.NewScript(`
local t = redis.call('TIME')
local now = t[1] * 1000 + math.floor(t[2] / 1000)
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', now)
if redis.call('ZCARD', KEYS[1]) >= tonumber(ARGV[1]) then return 0 end
redis.call('ZADD', KEYS[1], now + tonumber(ARGV[3]), ARGV[2])
redis.call('PEXPIRE', KEYS[1], ARGV[3])
return 1
`)

var renewUserConcurrencyScript = redis.NewScript(`
local t = redis.call('TIME')
local now = t[1] * 1000 + math.floor(t[2] / 1000)
local expiry = redis.call('ZSCORE', KEYS[1], ARGV[1])
if not expiry or tonumber(expiry) <= now then return 0 end
redis.call('ZADD', KEYS[1], 'XX', now + tonumber(ARGV[2]), ARGV[1])
redis.call('PEXPIRE', KEYS[1], ARGV[2])
return 1
`)

var localUserConcurrency = struct {
	sync.Mutex
	counts map[int]int
}{counts: make(map[int]int)}

type userConcurrencyLease struct {
	client *redis.Client
	key    string
	id     string
	userID int
	once   sync.Once
}

func acquireUserConcurrency(ctx context.Context, userID, limit int) (*userConcurrencyLease, error) {
	lease := &userConcurrencyLease{userID: userID}
	if !common.RedisEnabled {
		localUserConcurrency.Lock()
		defer localUserConcurrency.Unlock()
		if localUserConcurrency.counts[userID] >= limit {
			return nil, nil
		}
		localUserConcurrency.counts[userID]++
		return lease, nil
	}
	if common.RDB == nil {
		return nil, fmt.Errorf("user concurrency Redis client is unavailable")
	}
	lease.client = common.RDB
	lease.key = common.RedisPrefixedKey("user_concurrency:" + strconv.Itoa(userID))
	lease.id = uuid.NewString()
	ctx, cancel := context.WithTimeout(ctx, userConcurrencyRedisTimeout)
	defer cancel()
	result, err := acquireUserConcurrencyScript.Run(ctx, lease.client, []string{lease.key}, limit, lease.id, userConcurrencyLeaseTTL.Milliseconds()).Int()
	if err != nil {
		return nil, err
	}
	if result == 0 {
		return nil, nil
	}
	return lease, nil
}

func (lease *userConcurrencyLease) renew(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, userConcurrencyRedisTimeout)
	defer cancel()
	result, err := renewUserConcurrencyScript.Run(ctx, lease.client, []string{lease.key}, lease.id, userConcurrencyLeaseTTL.Milliseconds()).Int()
	if err != nil {
		return err
	}
	if result != 1 {
		return fmt.Errorf("user concurrency lease expired or was removed")
	}
	return nil
}

func (lease *userConcurrencyLease) release() {
	lease.once.Do(func() {
		if lease.client == nil {
			localUserConcurrency.Lock()
			defer localUserConcurrency.Unlock()
			localUserConcurrency.counts[lease.userID]--
			if localUserConcurrency.counts[lease.userID] == 0 {
				delete(localUserConcurrency.counts, lease.userID)
			}
			return
		}
		// Request cancellation must not prevent releasing its slot.
		ctx, cancel := context.WithTimeout(context.Background(), userConcurrencyRedisTimeout)
		defer cancel()
		if err := lease.client.ZRem(ctx, lease.key, lease.id).Err(); err != nil {
			common.SysError("release user concurrency lease: " + err.Error())
		}
	})
}

func (lease *userConcurrencyLease) keepAlive(ctx context.Context, cancel context.CancelFunc, done chan<- struct{}) {
	defer close(done)
	ticker := time.NewTicker(userConcurrencyRenewInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := lease.renew(ctx); err != nil {
				if ctx.Err() == nil {
					common.SysError("renew user concurrency lease: " + err.Error())
					cancel()
				}
				return
			}
		}
	}
}

// WebSocket reads need the underlying connection closed to observe a lost
// lease. HTTP and SSE upstream transports observe Request.Context instead.
type userConcurrencyWriter struct {
	gin.ResponseWriter
	ctx  context.Context
	stop func() bool
}

func (w *userConcurrencyWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	conn, rw, err := w.ResponseWriter.Hijack()
	if err == nil {
		w.stop = context.AfterFunc(w.ctx, func() { _ = conn.Close() })
	}
	return conn, rw, err
}

// UserConcurrencyLimit runs after authentication and before distribution or
// pre-billing. One slot spans all retries and the entire streamed response.
// Async task APIs count the submission request, not the remote job lifetime.
func UserConcurrencyLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetInt("id")
		limit := setting.GetUserConcurrencyLimit(userID, common.GetContextKeyString(c, constant.ContextKeyUserGroup))
		if limit == 0 {
			c.Next()
			return
		}
		if userID <= 0 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": gin.H{"message": i18n.T(c, i18n.MsgUserConcurrencyAuthenticationRequired), "type": "authentication_error"}})
			return
		}
		originalContext := c.Request.Context()
		lease, err := acquireUserConcurrency(originalContext, userID, limit)
		if err != nil {
			common.SysError("acquire user concurrency lease: " + err.Error())
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"message": i18n.T(c, i18n.MsgUserConcurrencyUnavailable), "type": "server_error", "code": "user_concurrency_unavailable"}})
			return
		}
		if lease == nil {
			c.Header("Retry-After", "1")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": gin.H{"message": i18n.T(c, i18n.MsgUserConcurrencyExceeded), "type": "rate_limit_error", "code": "user_concurrency_limit_exceeded"}})
			return
		}
		defer lease.release()
		if lease.client == nil {
			c.Next()
			return
		}
		ctx, cancel := context.WithCancel(originalContext)
		c.Request = c.Request.WithContext(ctx)
		writer := &userConcurrencyWriter{ResponseWriter: c.Writer, ctx: ctx}
		c.Writer = writer
		done := make(chan struct{})
		go lease.keepAlive(ctx, cancel, done)
		defer func() {
			cancel()
			<-done
			if writer.stop != nil {
				writer.stop()
			}
		}()
		c.Next()
		if ctx.Err() != nil && originalContext.Err() == nil && !c.Writer.Written() {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"message": i18n.T(c, i18n.MsgUserConcurrencyUnavailable), "type": "server_error", "code": "user_concurrency_unavailable"}})
		}
	}
}
