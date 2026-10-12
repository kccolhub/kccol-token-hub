package middleware

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/setting"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func configureUserConcurrencyTest(t *testing.T, value string) {
	t.Helper()
	require.NoError(t, i18n.Init())
	previous := setting.UserConcurrencyConfigJSON()
	enabled, client := common.RedisEnabled, common.RDB
	require.NoError(t, setting.UpdateUserConcurrencyConfig(value))
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserConcurrencyConfig(previous))
		common.RedisEnabled, common.RDB = enabled, client
	})
}

func concurrencyRedisClient(t *testing.T) *redis.Client {
	t.Helper()
	options := &redis.Options{MaxRetries: -1}
	if address := os.Getenv("USER_CONCURRENCY_REDIS_TEST_ADDR"); address != "" {
		options.Addr = address
	} else {
		options.Addr = miniredis.RunT(t).Addr()
	}
	client := redis.NewClient(options)
	require.NoError(t, client.Ping(context.Background()).Err())
	t.Setenv("REDIS_KEY_PREFIX", "concurrency-test:"+strconv.FormatInt(time.Now().UnixNano(), 10))
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	return client
}

func TestUserConcurrencyConfiguration(t *testing.T) {
	configureUserConcurrencyTest(t, `{"enabled":true,"default_limit":3,"groups":{"vip":8},"users":{"12":2,"13":0}}`)
	assert.Equal(t, 3, setting.GetUserConcurrencyLimit(10, "default"))
	assert.Equal(t, 8, setting.GetUserConcurrencyLimit(10, "vip"))
	assert.Equal(t, 2, setting.GetUserConcurrencyLimit(12, "vip"))
	assert.Zero(t, setting.GetUserConcurrencyLimit(13, "vip"))
	for _, invalid := range []string{`null`, `[]`, `{"default_limit":-1}`, `{"default_limit":1.5}`, `{"groups":{"vip":100001}}`, `{"users":{"01":2}}`, `{"users":{"abc":1}}`, `{"groups":{" ":2}}`} {
		require.Error(t, setting.UpdateUserConcurrencyConfig(invalid), invalid)
		assert.Equal(t, 2, setting.GetUserConcurrencyLimit(12, "vip"), "invalid updates leave the active configuration intact")
	}
	require.NoError(t, setting.UpdateUserConcurrencyConfig(`{"enabled":false,"default_limit":3}`))
	assert.Zero(t, setting.GetUserConcurrencyLimit(12, "vip"))
}

func TestUserConcurrencyHTTPStreamAcrossKeysAndInstances(t *testing.T) {
	for _, backend := range []string{"memory", "redis"} {
		t.Run(backend, func(t *testing.T) {
			configureUserConcurrencyTest(t, `{"enabled":true,"default_limit":1,"groups":{"vip":8},"users":{}}`)
			common.RedisEnabled = backend == "redis"
			if common.RedisEnabled {
				common.RDB = concurrencyRedisClient(t)
			}
			completed := make(chan struct{}, 1)
			newServer := func() *httptest.Server {
				router := gin.New()
				router.Use(func(c *gin.Context) {
					id, _ := strconv.Atoi(c.GetHeader("X-Test-User"))
					c.Set("id", id)
					c.Set("token_id", c.GetHeader("X-Test-Key"))
					common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
					c.Set("token_group", "vip") // Token group must not bypass the account cap.
					c.Next()
					if c.Query("stream") == "true" {
						completed <- struct{}{}
					}
				})
				router.Use(UserConcurrencyLimit())
				router.POST("/relay", func(c *gin.Context) {
					if c.Query("stream") == "true" {
						c.Header("Content-Type", "text/event-stream")
						c.String(http.StatusOK, "data: active\n\n")
						c.Writer.Flush()
						<-c.Request.Context().Done()
						return
					}
					c.Status(http.StatusNoContent)
				})
				server := httptest.NewServer(router)
				t.Cleanup(server.Close)
				return server
			}
			first, second := newServer(), newServer()
			request := func(url, user, key string) *http.Response {
				t.Helper()
				req, err := http.NewRequest(http.MethodPost, url, nil)
				require.NoError(t, err)
				req.Header.Set("X-Test-User", user)
				req.Header.Set("X-Test-Key", key)
				response, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				t.Cleanup(func() { response.Body.Close() })
				return response
			}
			active := request(first.URL+"/relay?stream=true", "12001", "key-a")
			require.Equal(t, http.StatusOK, active.StatusCode)
			blocked := request(second.URL+"/relay", "12001", "key-b")
			assert.Equal(t, http.StatusTooManyRequests, blocked.StatusCode)
			body, err := io.ReadAll(blocked.Body)
			require.NoError(t, err)
			assert.Contains(t, string(body), "user_concurrency_limit_exceeded")
			assert.Equal(t, "1", blocked.Header.Get("Retry-After"))
			other := request(second.URL+"/relay", "12002", "key-c")
			assert.Equal(t, http.StatusNoContent, other.StatusCode)
			require.NoError(t, active.Body.Close())
			select {
			case <-completed:
			case <-time.After(5 * time.Second):
				t.Fatal("stream cancellation did not finish")
			}
			next := request(second.URL+"/relay", "12001", "key-b")
			assert.Equal(t, http.StatusNoContent, next.StatusCode, "disconnect releases the slot")
		})
	}
}

func TestUserConcurrencyLeaseRenewalExpiryAndRelease(t *testing.T) {
	configureUserConcurrencyTest(t, `{"enabled":true,"default_limit":1}`)
	common.RedisEnabled = true
	common.RDB = concurrencyRedisClient(t)
	ctx := context.Background()
	first, err := acquireUserConcurrency(ctx, 22001, 1)
	require.NoError(t, err)
	require.NotNil(t, first)
	t.Cleanup(first.release)
	// A second client shares the same cap, as separate gateway replicas do.
	secondClient := redis.NewClient(common.RDB.Options())
	t.Cleanup(func() { secondClient.Close() })
	common.RDB = secondClient
	blocked, err := acquireUserConcurrency(ctx, 22001, 1)
	require.NoError(t, err)
	require.Nil(t, blocked)
	require.NoError(t, first.renew(ctx))
	assert.Greater(t, secondClient.PTTL(ctx, first.key).Val(), 30*time.Second)
	// Expire a crashed request without wall-clock sleeps.
	require.NoError(t, secondClient.ZAdd(ctx, first.key, &redis.Z{Score: 0, Member: first.id}).Err())
	replacement, err := acquireUserConcurrency(ctx, 22001, 1)
	require.NoError(t, err)
	require.NotNil(t, replacement)
	t.Cleanup(replacement.release)
	require.Error(t, first.renew(ctx), "renewal must never resurrect an expired request")
	first.release()
	first.release()
	blocked, err = acquireUserConcurrency(ctx, 22001, 1)
	require.NoError(t, err)
	assert.Nil(t, blocked, "late or repeated release cannot free another request's slot")
	replacement.release()
	next, err := acquireUserConcurrency(ctx, 22001, 1)
	require.NoError(t, err)
	require.NotNil(t, next)
	next.release()
	assert.Equal(t, common.RedisPrefixedKey("user_concurrency:22001"), first.key, "keys remain within the deployment Redis ACL namespace")
}

func TestUserConcurrencyFailureAndPanicRelease(t *testing.T) {
	configureUserConcurrencyTest(t, `{"enabled":true,"default_limit":1}`)
	common.RedisEnabled = false
	router := gin.New()
	router.Use(gin.Recovery(), func(c *gin.Context) { c.Set("id", 32001) }, UserConcurrencyLimit())
	router.POST("/panic", func(c *gin.Context) { panic("upstream handler failed") })
	router.POST("/ok", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	failed := httptest.NewRecorder()
	router.ServeHTTP(failed, httptest.NewRequest(http.MethodPost, "/panic", nil))
	require.Equal(t, http.StatusInternalServerError, failed.Code)
	next := httptest.NewRecorder()
	router.ServeHTTP(next, httptest.NewRequest(http.MethodPost, "/ok", nil))
	require.Equal(t, http.StatusNoContent, next.Code)
	common.RedisEnabled = true
	common.RDB = nil
	unavailable := httptest.NewRecorder()
	router.ServeHTTP(unavailable, httptest.NewRequest(http.MethodPost, "/ok", nil))
	assert.Equal(t, http.StatusServiceUnavailable, unavailable.Code, "Redis failure must not silently switch to per-process counters")
}

func TestUserConcurrencyLostLeaseCancelsRequest(t *testing.T) {
	configureUserConcurrencyTest(t, `{"enabled":true,"default_limit":1}`)
	common.RedisEnabled = true
	common.RDB = concurrencyRedisClient(t)
	entered := make(chan struct{})
	finished := make(chan struct{})
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("id", 42001) }, UserConcurrencyLimit())
	router.POST("/relay", func(c *gin.Context) {
		close(entered)
		<-c.Request.Context().Done()
	})
	response := httptest.NewRecorder()
	go func() {
		defer close(finished)
		router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/relay", nil))
	}()
	<-entered
	require.NoError(t, common.RDB.Del(context.Background(), common.RedisPrefixedKey("user_concurrency:42001")).Err())
	select {
	case <-finished:
	case <-time.After(userConcurrencyRenewInterval + 5*time.Second):
		t.Fatal("lost lease must cancel the upstream request")
	}
	assert.Equal(t, http.StatusServiceUnavailable, response.Code)
	assert.Contains(t, response.Body.String(), "user_concurrency_unavailable")
}
