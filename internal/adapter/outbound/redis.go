package outbound

import (
	"context"
	"encoding/json"
	"fmt"
	"obs-brutal/internal/core/domain"
	"obs-brutal/internal/core/port/outbound"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisConfigProvider provides configuration from Redis
type RedisConfigProvider struct {
	client      *redis.Client
	keyPrefix   string
	cache       map[string]interface{}
	cacheMu     sync.RWMutex
	subscribers []func()
	subMu       sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

// NewRedisConfigProvider creates a new Redis config provider
func NewRedisConfigProvider(addr, password string, db int, keyPrefix string) (outbound.ConfigSrc, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	// Test connection
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	provider := &RedisConfigProvider{
		client:    client,
		keyPrefix: keyPrefix,
		cache:     make(map[string]interface{}),
		ctx:       ctx,
		cancel:    cancel,
	}

	// Start watching for changes
	provider.wg.Add(1)
	go provider.watchChanges()

	// Initial load
	if err := provider.loadConfig(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to load initial config: %w", err)
	}

	return provider, nil
}

func (p *RedisConfigProvider) Level(module, tenant string) domain.Level {
	p.cacheMu.RLock()
	defer p.cacheMu.RUnlock()

	// Check tenant-specific level first
	if tenant != "" {
		key := fmt.Sprintf("level:tenant:%s:%s", tenant, module)
		if level, ok := p.cache[key]; ok {
			return parseLevel(level.(string))
		}

		// Check tenant default
		key = fmt.Sprintf("level:tenant:%s", tenant)
		if level, ok := p.cache[key]; ok {
			return parseLevel(level.(string))
		}
	}

	// Check module-specific level
	if module != "" {
		key := fmt.Sprintf("level:module:%s", module)
		if level, ok := p.cache[key]; ok {
			return parseLevel(level.(string))
		}
	}

	// Default level
	if level, ok := p.cache["level:default"]; ok {
		return parseLevel(level.(string))
	}

	return domain.InfoLevel
}

func (p *RedisConfigProvider) Rules() []domain.FilterRule {
	p.cacheMu.RLock()
	defer p.cacheMu.RUnlock()

	var rules []domain.FilterRule

	// Get filter rules from cache
	if rulesData, ok := p.cache["filter:rules"]; ok {
		if rulesJSON, ok := rulesData.(string); ok {
			json.Unmarshal([]byte(rulesJSON), &rules)
		}
	}

	return rules
}

func (p *RedisConfigProvider) Sample(module string) float32 {
	p.cacheMu.RLock()
	defer p.cacheMu.RUnlock()

	// Check module-specific rate
	if module != "" {
		key := fmt.Sprintf("sampling:module:%s", module)
		if rate, ok := p.cache[key]; ok {
			switch v := rate.(type) {
			case float64:
				return float32(v)
			case string:
				var f float32
				fmt.Sscanf(v, "%f", &f)
				return f
			}
		}
	}

	// Default rate
	if rate, ok := p.cache["sampling:default"]; ok {
		switch v := rate.(type) {
		case float64:
			return float32(v)
		case string:
			var f float32
			fmt.Sscanf(v, "%f", &f)
			return f
		}
	}

	return 1.0 // No sampling by default
}

func (p *RedisConfigProvider) Sub(callback func()) error {
	p.subMu.Lock()
	defer p.subMu.Unlock()

	p.subscribers = append(p.subscribers, callback)
	return nil
}

func (p *RedisConfigProvider) Health() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return p.client.Ping(ctx).Err()
}

func (p *RedisConfigProvider) Close() error {
	p.cancel()
	p.wg.Wait()
	return p.client.Close()
}

func (p *RedisConfigProvider) loadConfig() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get all keys with prefix
	keys, err := p.client.Keys(ctx, p.keyPrefix+"*").Result()
	if err != nil {
		return fmt.Errorf("failed to get keys: %w", err)
	}

	if len(keys) == 0 {
		return nil
	}

	// Get all values
	values, err := p.client.MGet(ctx, keys...).Result()
	if err != nil {
		return fmt.Errorf("failed to get values: %w", err)
	}

	// Update cache
	p.cacheMu.Lock()
	defer p.cacheMu.Unlock()

	for i, key := range keys {
		if values[i] != nil {
			// Remove prefix from key
			cacheKey := strings.TrimPrefix(key, p.keyPrefix)
			p.cache[cacheKey] = values[i]
		}
	}

	return nil
}

func (p *RedisConfigProvider) watchChanges() {
	defer p.wg.Done()

	// Subscribe to key changes
	pubsub := p.client.PSubscribe(p.ctx, fmt.Sprintf("__keyspace@*__:%s*", p.keyPrefix))
	defer pubsub.Close()

	ch := pubsub.Channel()

	// Reload timer
	reloadTimer := time.NewTimer(time.Hour)
	defer reloadTimer.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return

		case <-ch:
			// Key changed, reload config
			if err := p.loadConfig(); err != nil {
				// Log error but continue
				fmt.Printf("Failed to reload config: %v\n", err)
				continue
			}

			// Notify subscribers
			p.notifySubscribers()

		case <-reloadTimer.C:
			// Periodic reload
			if err := p.loadConfig(); err != nil {
				fmt.Printf("Failed to reload config: %v\n", err)
			}
			reloadTimer.Reset(time.Hour)
		}
	}
}

func (p *RedisConfigProvider) notifySubscribers() {
	p.subMu.RLock()
	subscribers := make([]func(), len(p.subscribers))
	copy(subscribers, p.subscribers)
	p.subMu.RUnlock()

	for _, callback := range subscribers {
		go callback()
	}
}

// RedisCacheClient implements CacheClient interface using Redis
type RedisCacheClient struct {
	client *redis.Client
}

// NewRedisCacheClient creates a new Redis cache client
func NewRedisCacheClient(addr, password string, db int) (outbound.Cache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	// Test connection
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisCacheClient{
		client: client,
	}, nil
}

func (c *RedisCacheClient) Get(ctx context.Context, key string) ([]byte, error) {
	val, err := c.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil // Key not found
	}
	return val, err
}

func (c *RedisCacheClient) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return c.client.Set(ctx, key, value, ttl).Err()
}

func (c *RedisCacheClient) Del(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}

func (c *RedisCacheClient) Close() error {
	return c.client.Close()
}

// Helper functions

func parseLevel(level string) domain.Level {
	switch strings.ToLower(level) {
	case "debug":
		return domain.DebugLevel
	case "info":
		return domain.InfoLevel
	case "warn", "warning":
		return domain.WarnLevel
	case "error":
		return domain.ErrorLevel
	case "fatal":
		return domain.FatalLevel
	default:
		return domain.InfoLevel
	}
}
