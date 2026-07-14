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

	"github.com/steemit/conveyor/internal/config"
	applog "github.com/steemit/conveyor/internal/log"
	"github.com/steemit/conveyor/internal/jsonrpc"
	"github.com/steemit/conveyor/internal/telemetry"
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

	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())

	// OpenTelemetry server-span middleware (only if enabled).
	if cfg.Telemetry.Enabled {
		engine.Use(otelgin.Middleware(cfg.Telemetry.ServiceName))
	}

	a := &App{cfg: cfg, log: log, engine: engine}

	// Set up telemetry (non-fatal on error).
	if cfg.Telemetry.Enabled {
		shutdown, err := telemetry.Setup(
			cfg.Telemetry.ServiceName,
			cfg.Telemetry.Endpoint,
			log,
		)
		if err != nil {
			log.Error().Err(err).Msg("Failed to initialize OpenTelemetry")
		} else {
			a.shutdowns = append(a.shutdowns, shutdown)
		}
	} else {
		log.Info().Msg("OpenTelemetry disabled by configuration")
	}

	// Healthcheck routes.
	engine.GET("/", healthcheck)
	engine.GET("/.well-known/healthcheck.json", healthcheck)

	// JSON-RPC endpoint.
	rpc := jsonrpc.NewServer()
	engine.POST("/", rpc.Handler(log))
	a.registerMethods(rpc)

	a.httpSrv = &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: engine,
	}
	return a, nil
}

// registerMethods registers M0's public methods. Future milestones add
// authenticated methods here.
func (a *App) registerMethods(rpc *jsonrpc.Server) {
	rpc.Register("hello", hello)
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
