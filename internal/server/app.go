// Package server wires the Gin HTTP server: OpenTelemetry middleware,
// JSON-RPC POST / endpoint, and healthcheck routes.
package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"gorm.io/gorm"

	"github.com/steemit/conveyor/internal/config"
	"github.com/steemit/conveyor/internal/drafts"
	applog "github.com/steemit/conveyor/internal/log"
	"github.com/steemit/conveyor/internal/featureflags"
	"github.com/steemit/conveyor/internal/jsonrpc"
	"github.com/steemit/conveyor/internal/models"
	"github.com/steemit/conveyor/internal/prices"
	"github.com/steemit/conveyor/internal/store"
	"github.com/steemit/conveyor/internal/summarizer"
	"github.com/steemit/conveyor/internal/tags"
	"github.com/steemit/conveyor/internal/telemetry"
	"github.com/steemit/conveyor/internal/userdata"
	"github.com/steemit/conveyor/internal/usersearch"
)

// App holds the configured HTTP server and its dependencies.
type App struct {
	cfg       *config.Config
	log       zerolog.Logger
	engine    *gin.Engine
	httpSrv   *http.Server
	shutdowns []func()
}

// New builds the App from config: logger, telemetry, routes, and the RPC
// registry. RPC handlers are registered via the returned *jsonrpc.Server so
// the caller (or future milestones) can add methods.
func New(cfg *config.Config) (*App, error) {
	log := applog.New(cfg.Name, cfg.Log)

	a := &App{cfg: cfg, log: log}

	// Set up telemetry first (non-fatal on error). Only register the otelgin
	// middleware if Setup actually succeeded, otherwise every request would
	// produce span-export errors.
	telemetryOK := false
	if cfg.Telemetry.Enabled {
		shutdown, err := telemetry.Setup(cfg.Telemetry, log)
		if err != nil {
			log.Error().Err(err).Msg("Failed to initialize OpenTelemetry")
		} else {
			a.shutdowns = append(a.shutdowns, shutdown)
			telemetryOK = true
		}
	} else {
		log.Info().Msg("OpenTelemetry disabled by configuration")
	}

	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())

	// Trusted proxies: only the configured direct upstream (e.g. openresty) is
	// trusted to set X-Forwarded-For. Empty = trust no proxy (use TCP peer
	// only), which is the safe default and prevents client-IP spoofing. This
	// matters because ctx.IP feeds the audit memo in tags.assign_tag.
	if err := engine.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		return nil, fmt.Errorf("set trusted proxies: %w", err)
	}

	if telemetryOK {
		engine.Use(otelgin.Middleware(cfg.Telemetry.ServiceName))
	}
	a.engine = engine

	// Healthcheck routes.
	engine.GET("/", healthcheck)
	engine.GET("/.well-known/healthcheck.json", healthcheck)

	// JSON-RPC endpoint.
	rpc := jsonrpc.NewServer()
	rpc.SetAuthenticator(newAuthenticator(cfg.RpcNode))
	engine.POST("/", rpc.Handler(log))

	// Initialize BlobStore (drafts, feature-flags) and database (user-data, tags).
	ctx := context.Background()
	blobStore, err := store.NewStore(ctx, cfg.Storage)
	if err != nil {
		return nil, fmt.Errorf("init blob store: %w", err)
	}
	db, err := models.NewDB(cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("init database: %w", err)
	}

	// Initialize user-search CachingClient + AccountNameTrie.
	cacheTTL := time.Duration(cfg.CacheClient.TTL) * time.Second
	cacheCleanup := time.Duration(cfg.CacheClient.Interval) * time.Second
	if cacheTTL == 0 {
		cacheTTL = 600 * time.Second
	}
	if cacheCleanup == 0 {
		cacheCleanup = 60 * time.Second
	}
	usClient := usersearch.NewCachingClient(cfg.RpcNode, cacheTTL, cacheCleanup)
	accountNames := usersearch.LoadAccountNames("user-data/accounts/accounts.js")
	if len(accountNames) == 0 {
		log.Warn().Msg("no account names loaded (user-data/accounts/accounts.js missing or empty); autocomplete will be limited until refresh populates the trie — run 'make user-accounts' to generate")
	} else {
		log.Info().Int("count", len(accountNames)).Msg("loaded account names for autocomplete trie")
	}
	refreshInterval := time.Duration(cfg.AccountsRefreshInterval) * time.Millisecond
	if refreshInterval == 0 {
		refreshInterval = 600000 * time.Millisecond
	}
	trie := usersearch.NewAccountNameTrie(accountNames, usClient, refreshInterval)
	trie.StartRefreshing()
	a.shutdowns = append(a.shutdowns, trie.StopRefreshing)

	a.registerMethods(rpc, blobStore, db, usClient, trie)

	a.httpSrv = &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: engine,
	}
	return a, nil
}

// registerMethods registers conveyor's RPC methods with the "conveyor."
// namespace prefix (matching the TS JsonRpcAuth namespace and the schema).
func (a *App) registerMethods(rpc *jsonrpc.Server, blobStore store.BlobStore, db *gorm.DB, usClient *usersearch.CachingClient, trie *usersearch.AccountNameTrie) {
	// Public methods.
	rpc.Register("conveyor.hello", hello)
	rpc.RegisterAuthenticated("conveyor.whoami", whoami)

	// Prices (steemgosdk).
	prices.New(a.cfg.RpcNode).Register(rpc)

	// Drafts (BlobStore).
	drafts.New(blobStore, a.cfg.Name).Register(rpc)

	// Feature flags (BlobStore + MT19937).
	featureflags.New(blobStore, a.cfg.Name, a.cfg.AdminRole).Register(rpc)

	// User data (GORM).
	userdata.New(db, a.cfg.AdminRole).Register(rpc)

	// Tags (GORM).
	tags.New(db, a.cfg.AdminRole).Register(rpc)

	// User search (steemgosdk + trie).
	usersearch.New(usClient, trie).Register(rpc)

	// Summarizer (HTTP fetch + metadata extraction).
	summarizer.New().Register(rpc)
}

// hello is the M0 smoke-test method, mirroring the original TS hello.
func hello(ctx *jsonrpc.Context, req *jsonrpc.Request) (any, error) {
	var p struct {
		Name string `json:"name"`
	}
	_ = req.UnmarshalParams(&p) // optional params
	name := p.Name
	if name == "" {
		name = "Anonymous"
	}
	ctx.Log.Info().Str("name", name).Msg("hello")
	return fmt.Sprintf("I'm sorry, %s, I can't do that.", name), nil
}

// whoami is the M1 authenticated smoke-test method, mirroring the original TS
// whoami. Returns the authenticated account name from the verified __signed
// request.
func whoami(ctx *jsonrpc.Context, req *jsonrpc.Request) (any, error) {
	ctx.Log.Info().Str("account", ctx.Account).Msg("whoami")
	return ctx.Account, nil
}

// Run starts the HTTP server and blocks until a termination signal is
// received, then shuts down gracefully.
func (a *App) Run() error {
	errCh := make(chan error, 1)
	go func() {
		a.log.Info().Str("port", a.cfg.Port).Msg("conveyor listening")
		if err := a.httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case <-quit:
	}

	a.log.Info().Msg("Shutting down server")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := a.httpSrv.Shutdown(ctx); err != nil {
		a.log.Error().Err(err).Msg("Server forced to shutdown")
	}
	for _, fn := range a.shutdowns {
		fn()
	}
	return nil
}
