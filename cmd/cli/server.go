package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	gops_handler "github.com/AvengeMedia/dgop/api/gops"
	"github.com/AvengeMedia/dgop/api/middleware"
	"github.com/AvengeMedia/dgop/api/server"
	"github.com/AvengeMedia/dgop/config"
	"github.com/AvengeMedia/dgop/errdefs"
	"github.com/AvengeMedia/dgop/gops"
	"github.com/charmbracelet/log"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/schema"
	"github.com/spf13/cobra"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the API server",
	Long:  "Start the REST API server to provide system metrics endpoints.",
	RunE:  runServerCommand,
}

// Adding a format for form data
var decoder = schema.NewDecoder()
var urlEncodedFormat = huma.Format{
	Marshal: nil,
	Unmarshal: func(data []byte, v any) error {
		values, err := url.ParseQuery(string(data))
		if err != nil {
			return err
		}

		// WARNING: Dirty workaround!
		// During validation, Huma first parses the body into []any, map[string]any or equivalent for easy validation,
		// before parsing it into the target struct.
		// However, gorilla/schema requires a struct for decoding, so we need to map `url.Values` to a
		// `map[string]any` if this happens.
		// See: https://github.com/danielgtaylor/huma/blob/main/huma.go#L1264
		if vPtr, ok := v.(*interface{}); ok {
			m := map[string]any{}
			for k, v := range values {
				if len(v) > 1 {
					m[k] = v
				} else if len(v) == 1 {
					m[k] = v[0]
				}
			}
			*vPtr = m
			return nil
		}

		// `v` is a struct, try decode normally
		return decoder.Decode(v, values)
	},
}

func NewHumaConfig(title, version string) huma.Config {
	schemaPrefix := "#/components/schemas/"
	schemasPath := "/schemas"

	registry := huma.NewMapRegistry(schemaPrefix, huma.DefaultSchemaNamer)

	cfg := huma.Config{
		OpenAPI: &huma.OpenAPI{
			OpenAPI: "3.1.0",
			Info: &huma.Info{
				Title:   title,
				Version: version,
			},
			Components: &huma.Components{
				Schemas: registry,
			},
		},
		OpenAPIPath:   "/openapi",
		DocsPath:      "/docs",
		SchemasPath:   schemasPath,
		Formats:       huma.DefaultFormats,
		DefaultFormat: "application/json",
	}
	cfg.Formats["application/x-www-form-urlencoded"] = urlEncodedFormat
	cfg.Formats["x-www-form-urlencoded"] = urlEncodedFormat

	return cfg
}

func runServerCommand(cmd *cobra.Command, args []string) error {
	cfg := config.NewConfig()
	return startAPI(cfg)
}

func startAPI(cfg *config.Config) error {
	// Create a context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-signalCh
		slog.Info("Received shutdown signal", "signal", sig)
		cancel() // This will propagate cancellation to all derived contexts
	}()

	// Implementation
	srvImpl := &server.Server{
		Cfg:  cfg,
		Gops: gops.NewGopsUtil(),
	}

	// New chi router
	r := chi.NewRouter()

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	r.Group(func(r chi.Router) {
		r.Use(middleware.Logger)

		// Register huma error function
		huma.NewError = errdefs.HumaErrorFunc

		config := NewHumaConfig("DankGop API", "1.0.0")
		config.DocsPath = ""
		api := humachi.New(r, config)

		// Create middleware
		mw := middleware.NewMiddleware(cfg, api)

		api.UseMiddleware(mw.Recoverer)

		r.Get("/docs", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<!doctype html>
			<html>
				<head>
					<title>API Reference</title>
					<meta charset="utf-8" />
					<meta
						name="viewport"
						content="width=device-width, initial-scale=1" />
				</head>
				<body>
					<script
						id="api-reference"
						data-url="/openapi.json"></script>
					<script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
				</body>
			</html>`))
		})

		// /gops group
		gopsGroup := huma.NewGroup(api, "/gops")
		gopsGroup.UseModifier(func(op *huma.Operation, next func(*huma.Operation)) {
			op.Tags = []string{"Gops"}
			next(op)
		})
		gops_handler.RegisterHandlers(srvImpl, gopsGroup)
	})

	// Start the server
	addr := ":63484"
	log.Infof(" Starting DankGop API server on %s", addr)
	log.Infof(" API Documentation: http://localhost%s/docs", addr)
	log.Infof(" OpenAPI Spec: http://localhost%s/openapi.json", addr)
	log.Infof(" Health Check: http://localhost%s/health", addr)

	h2s := &http2.Server{}

	httpServer := &http.Server{
		Addr:    addr,
		Handler: h2c.NewHandler(r, h2s),
	}

	// Start the server in a goroutine
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for context cancellation (from signal handler)
	<-ctx.Done()
	log.Info("Shutting down server...")

	// Create a shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Shutdown the HTTP server
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server shutdown error: %v", err)
	}

	log.Info("Server gracefully stopped")
	return nil
}
