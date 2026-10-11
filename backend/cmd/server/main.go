// Command server serves the calculator API.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"calculator/internal/api"
)

const (
	defaultAddr = ":8080"

	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	writeTimeout      = 10 * time.Second
	idleTimeout       = 60 * time.Second
	// shutdownTimeout is how long requests in flight get to finish. docker
	// stop kills the process 10 seconds after it sends SIGTERM.
	shutdownTimeout = 5 * time.Second
)

// shutdownSignals stop the server gracefully: Ctrl+C sends os.Interrupt, and
// docker stop sends SIGTERM.
var shutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), shutdownSignals...)
	err := run(ctx, os.Args[1:], os.Getenv, os.Stderr)
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, "server:", err)
		os.Exit(1)
	}
}

// run serves the API until ctx is cancelled, then shuts down gracefully and
// returns nil. args are the command-line arguments without the program name,
// getenv reads environment variables, and logs go to stderr as JSON.
func run(ctx context.Context, args []string, getenv func(string) string, stderr io.Writer) error {
	cfg, err := parseConfig(args, getenv, stderr)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return err
	}
	log := slog.New(slog.NewJSONHandler(stderr, nil))

	ln, err := net.Listen("tcp", cfg.addr)
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.Handle("/api/", api.NewHandler(log))
	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelError),
	}
	log.Info("listening", "addr", ln.Addr().String())

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()
	select {
	case err := <-serveErr:
		// Shutdown has not been called, so Serve failed.
		return err
	case <-ctx.Done():
	}

	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	log.Info("stopped")
	return nil
}

type config struct {
	addr string
}

// parseConfig reads the listen address from the -addr flag, then the ADDR
// environment variable, then defaultAddr. Usage and flag errors are written
// to stderr; -h returns flag.ErrHelp.
func parseConfig(args []string, getenv func(string) string, stderr io.Writer) (config, error) {
	addr := getenv("ADDR")
	if addr == "" {
		addr = defaultAddr
	}
	var cfg config
	fs := flag.NewFlagSet("server", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&cfg.addr, "addr", addr, "listen `address`, such as :8080 (env ADDR)")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	if fs.NArg() > 0 {
		return config{}, fmt.Errorf("unexpected arguments: %q", fs.Args())
	}
	// An empty address would listen on a random port.
	if cfg.addr == "" {
		return config{}, errors.New("-addr must not be empty")
	}
	return cfg, nil
}
