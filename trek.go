package trek

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
)

var (
	globalInstance *Instance
	globalMu       sync.RWMutex
)

// Instance holds the initialized Trek SDK state.
type Instance struct {
	config     Config
	client     *Client
	cache      *Cache
	extractor  ContextExtractor
	globalCaps *GlobalCaps
}

// Init initializes the global Trek instance with the given configuration.
// If initialization fails and StrictInit is false, Trek operates in disabled mode.
func Init(cfg Config) error {
	if err := cfg.Validate(); err != nil {
		if cfg.StrictInit {
			return err
		}
		slog.Warn("trek: initialization failed, operating in disabled mode", "error", err)
		return nil
	}

	client := NewClient(cfg.APIEndpoint, cfg.APIToken, cfg.OrgID, cfg.Env)
	cache := NewCache(client, cfg.ServiceName, cfg.PollInterval)

	var globalCaps *GlobalCaps
	if cfg.EnableGlobalCaps {
		globalCaps = NewGlobalCaps(client)
		slog.Info("trek: global caps enabled")
	}

	instance := &Instance{
		config:     cfg,
		client:     client,
		cache:      cache,
		extractor:  DefaultExtractor(),
		globalCaps: globalCaps,
	}

	cache.Start(context.Background())

	globalMu.Lock()
	globalInstance = instance
	globalMu.Unlock()

	slog.Info("trek: initialized",
		"service", cfg.ServiceName,
		"org", cfg.OrgID,
		"env", cfg.Env,
		"global_caps", cfg.EnableGlobalCaps,
	)

	return nil
}

// InitWithConfig initializes Trek with explicit configuration and extractor.
func InitWithConfig(cfg Config, extractor ContextExtractor) error {
	if err := cfg.Validate(); err != nil {
		if cfg.StrictInit {
			return err
		}
		slog.Warn("trek: initialization failed, operating in disabled mode", "error", err)
		return nil
	}

	client := NewClient(cfg.APIEndpoint, cfg.APIToken, cfg.OrgID, cfg.Env)
	cache := NewCache(client, cfg.ServiceName, cfg.PollInterval)

	var globalCaps *GlobalCaps
	if cfg.EnableGlobalCaps {
		globalCaps = NewGlobalCaps(client)
		slog.Info("trek: global caps enabled")
	}

	instance := &Instance{
		config:     cfg,
		client:     client,
		cache:      cache,
		extractor:  extractor,
		globalCaps: globalCaps,
	}

	cache.Start(context.Background())

	globalMu.Lock()
	globalInstance = instance
	globalMu.Unlock()

	return nil
}

// Shutdown gracefully shuts down the Trek SDK.
func Shutdown() {
	globalMu.Lock()
	instance := globalInstance
	globalInstance = nil
	globalMu.Unlock()

	if instance != nil && instance.cache != nil {
		instance.cache.Stop()
	}
}

// HTTPMiddleware returns the standard HTTP middleware using the global instance.
func HTTPMiddleware(next http.Handler) http.Handler {
	globalMu.RLock()
	instance := globalInstance
	globalMu.RUnlock()

	if instance == nil {
		return next
	}

	return Middleware(instance.cache, instance.config.ServiceName, instance.extractor)(next)
}

// WrapDefaultHandler wraps the provided slog.Handler with Trek behavior.
// Uses global config for redaction if set.
func WrapDefaultHandler(h slog.Handler) *Handler {
	globalMu.RLock()
	instance := globalInstance
	globalMu.RUnlock()

	if instance == nil {
		return WrapHandler(h)
	}

	return WrapHandlerWithRedaction(h, instance.config.RedactAttr, instance.config.RedactEvent)
}

// APIClient returns the API client from the global instance.
// Returns nil if Trek is not initialized.
func APIClient() *Client {
	globalMu.RLock()
	defer globalMu.RUnlock()

	if globalInstance == nil {
		return nil
	}
	return globalInstance.client
}

// GlobalCache returns the session cache from the global instance.
// Returns nil if Trek is not initialized.
func GlobalCache() *Cache {
	globalMu.RLock()
	defer globalMu.RUnlock()

	if globalInstance == nil {
		return nil
	}
	return globalInstance.cache
}

// IsEnabled returns true if Trek is initialized and operational.
func IsEnabled() bool {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalInstance != nil
}

// SetExtractor updates the context extractor on the global instance.
func SetExtractor(extractor ContextExtractor) {
	globalMu.Lock()
	defer globalMu.Unlock()

	if globalInstance != nil {
		globalInstance.extractor = extractor
	}
}
