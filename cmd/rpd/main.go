package main

import (
	"context"
	"fmt"
	stdlog "log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/akhenakh/rp"
	log "github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/namsral/flag"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/sync/errgroup"
)

const appName = "rpd"

var (
	version = "no version from LDFLAGS"

	httpMetricsPort = flag.Int("httpMetricsPort", 8888, "http metrics port")
	httpPort        = flag.Int("httpPort", 8080, "http proxy port")
	backendAddrs    = flag.String("backendAddrs", "", "backend adresses, comma separated")

	useCache  = flag.Bool("useCache", false, "enable responses caching")
	cacheSize = flag.Int("cacheSize", 64, "LRU cache size")

	httpServer        *http.Server
	httpMetricsServer *http.Server
)

func main() {
	flag.Parse()

	logger := log.NewJSONLogger(log.NewSyncWriter(os.Stdout))
	logger = log.With(logger, "caller", log.DefaultCaller, "ts", log.DefaultTimestampUTC)
	logger = log.With(logger, "app", appName)
	logger = level.NewFilter(logger, level.AllowAll())

	stdlog.SetOutput(log.NewStdlibAdapter(logger))

	level.Info(logger).Log("msg", "Starting app", "version", version)

	var backends []string
	for _, b := range strings.Split(*backendAddrs, ",") {
		if bc := strings.TrimSpace(b); bc != "" {
			backends = append(backends, bc)
		}
	}

	if len(backends) < 1 {
		level.Info(logger).Log("error", "no backend address provided")
		os.Exit(2)
	}

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Preparing the proxy
	ps, err := rp.New(backends,
		&rp.Options{
			Cache:     *useCache,
			CacheSize: *cacheSize,
			Logger:    logger,
		})
	if err != nil {
		level.Error(logger).Log("error", err)
		os.Exit(2)
	}

	g, ctx := errgroup.WithContext(ctx)

	// HTTP metric server
	g.Go(func() error {
		httpMetricsServer = &http.Server{
			Addr:         fmt.Sprintf(":%d", *httpMetricsPort),
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		}

		// Register Prometheus metrics handler.
		http.Handle("/metrics", promhttp.Handler())

		level.Info(logger).Log("msg", fmt.Sprintf("HTTP Metrics server serving at :%d", *httpMetricsPort))

		if err := httpMetricsServer.ListenAndServe(); err != http.ErrServerClosed {
			return err
		}

		return nil
	})

	// HTTP proxy server
	g.Go(func() error {
		httpServer = &http.Server{
			Addr:         fmt.Sprintf(":%d", *httpPort),
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			Handler:      ps,
		}

		level.Info(logger).Log("msg", fmt.Sprintf("HTTP proxy server serving at :%d", *httpPort))
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			return err
		}

		return nil
	})

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(interrupt)

	select {
	case <-interrupt:
		break
	case <-ctx.Done():
		break
	}

	level.Warn(logger).Log("msg", "received shutdown signal")

	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if httpMetricsServer != nil {
		_ = httpMetricsServer.Shutdown(shutdownCtx)
	}
	if httpServer != nil {
		_ = httpServer.Shutdown(shutdownCtx)
	}

	err = g.Wait()
	if err != nil {
		level.Error(logger).Log("msg", "server returning an error", "error", err)
		os.Exit(2)
	}
}
