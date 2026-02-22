// main is the entry point of the Students API application.
//
// STARTUP SEQUENCE:
//  1. Load configuration from a YAML file
//  2. Initialise the logger
//  3. Connect to (and set up) the SQLite database
//  4. Register all HTTP routes
//  5. Start the HTTP server in a separate goroutine
//  6. Block the main goroutine until an OS signal (Ctrl+C / kill) arrives
//  7. Gracefully shut down: finish in-flight requests, then exit
//
// RUNNING THE SERVER:
//
//	go run ./cmd/students-api --config=config/local.yaml
//
// or (with the environment variable):
//
//	CONFIG_PATH=config/local.yaml go run ./cmd/students-api
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aanand-mishra/students-api/internal/config"
	"github.com/aanand-mishra/students-api/internal/http/handlers/student"
	"github.com/aanand-mishra/students-api/internal/storage/sqlite"
)

func main() {
	// ── 1. Load Config ────────────────────────────────────────────────────
	// MustLoad reads the YAML config and panics if anything is wrong.
	// The name "Must" signals: if this returns, config is guaranteed valid.
	cfg := config.MustLoad()

	// ── 2. Initialise Logger ──────────────────────────────────────────────
	// slog is Go's structured logger (stdlib since Go 1.21).
	// Structured logging writes key=value pairs rather than plain strings,
	// making logs easy to filter/search in tools like Loki or Datadog.
	log := setupLogger(cfg.Env)

	log.Info("starting students-api",
		slog.String("env", cfg.Env),
		slog.String("version", "1.0.0"),
	)

	// ── 3. Initialise Storage (Database) ──────────────────────────────────
	// sqlite.New opens the SQLite file and creates the students table.
	// We store the result as the storage.Storage INTERFACE, not *sqlite.SQLite.
	// This means the rest of the code only knows about the interface —
	// swapping to PostgreSQL later only requires changing this one line.
	storage, err := sqlite.New(cfg)
	if err != nil {
		log.Error("failed to initialise storage",
			slog.String("error", err.Error()))
		os.Exit(1) // non-zero exit code signals failure to the OS / CI system
	}

	log.Info("storage initialised",
		slog.String("path", cfg.StoragePath))

	// ── 4. Register HTTP Routes ───────────────────────────────────────────
	// http.NewServeMux() creates an empty router.
	// HandleFunc maps a METHOD+PATTERN to a handler function.
	//
	// The handler functions (student.New, student.GetByID, etc.) are
	// FACTORIES — they receive `storage` and return the actual handler.
	// This is the dependency injection / closure pattern.
	//
	// Route table:
	//   POST   /api/students        → create a new student
	//   GET    /api/students        → list all students
	//   GET    /api/students/{id}   → get one student by ID
	//   PUT    /api/students/{id}   → update a student
	//   DELETE /api/students/{id}   → delete a student
	router := http.NewServeMux()

	router.HandleFunc("POST /api/students", student.New(storage))
	router.HandleFunc("GET /api/students", student.GetList(storage))
	router.HandleFunc("GET /api/students/{id}", student.GetByID(storage))
	router.HandleFunc("PUT /api/students/{id}", student.Update(storage))
	router.HandleFunc("DELETE /api/students/{id}", student.Delete(storage))

	// ── 5. Create the HTTP Server ─────────────────────────────────────────
	// http.Server is a struct. We configure it here but don't start it yet.
	server := &http.Server{
		Addr:    cfg.HTTPServer.Addr, // e.g. "localhost:8082"
		Handler: router,              // every request goes through our router

		// Production hardening — set timeouts to prevent slow-client attacks.
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// ── 6. Start Server in a Goroutine ────────────────────────────────────
	// ListenAndServe blocks forever (it loops accepting connections).
	// If we called it here in main(), the graceful-shutdown code below
	// would never run. So we run it in a separate goroutine.
	//
	// go func() { ... }() is an immediately-invoked goroutine (anonymous
	// function launched with the `go` keyword).
	go func() {
		log.Info("server started", slog.String("address", cfg.HTTPServer.Addr))

		// ListenAndServe returns http.ErrServerClosed when Shutdown() is
		// called. That's expected — we don't want to log it as an error.
		if err := server.ListenAndServe(); err != nil &&
			err != http.ErrServerClosed {
			log.Error("server encountered an error",
				slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	// ── 7. Wait for Shutdown Signal ───────────────────────────────────────
	// make(chan os.Signal, 1) creates a buffered channel of size 1.
	// Buffered so we don't miss the signal if main is briefly busy.
	done := make(chan os.Signal, 1)

	// signal.Notify registers our channel to receive specific OS signals:
	//   os.Interrupt = Ctrl+C (SIGINT)
	//   syscall.SIGTERM = sent by `kill <pid>` or container orchestrators
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	// <-done blocks (pauses) the main goroutine here.
	// The program stays alive because this goroutine is running.
	// When a signal arrives, done receives it and we unblock.
	<-done

	log.Info("shutdown signal received, stopping server...")

	// ── 8. Graceful Shutdown ──────────────────────────────────────────────
	// context.WithTimeout gives the shutdown a 5-second deadline.
	// If in-flight requests don't finish within 5 seconds,
	// the context cancels and Shutdown returns an error.
	//
	// defer cancel() ensures the context's resources are freed
	// when main() returns, even if Shutdown finishes early.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// server.Shutdown:
	//   • Stops accepting new connections
	//   • Waits for active requests to complete (up to ctx deadline)
	//   • Returns nil on clean shutdown, error if deadline exceeded
	if err := server.Shutdown(ctx); err != nil {
		log.Error("failed to shutdown server gracefully",
			slog.String("error", err.Error()))
		os.Exit(1)
	}

	log.Info("server stopped gracefully")
}

// setupLogger returns a *slog.Logger configured for the given environment.
//
// Development (dev): human-readable text output at DEBUG level.
// Production (prod): machine-readable JSON output at INFO level.
//
//	JSON logs are easy to ingest by log aggregators (Loki, CloudWatch, etc.)
func setupLogger(env string) *slog.Logger {
	switch env {
	case "prod":
		return slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
				Level: slog.LevelInfo, // INFO and above in production
			}),
		)
	case "staging":
		return slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
				Level: slog.LevelDebug, // more verbose in staging
			}),
		)
	default: // "dev" and anything unrecognised
		return slog.New(
			slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
				Level: slog.LevelDebug, // all levels in development
			}),
		)
	}
}
