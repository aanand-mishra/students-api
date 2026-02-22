// Package config handles loading and parsing application configuration.
// It supports two sources (in priority order):
//  1. An environment variable:  CONFIG_PATH=/path/to/config.yaml
//  2. A command-line flag:      --config=/path/to/config.yaml
//
// The parsed values are returned as a *Config pointer so the struct is
// shared by reference rather than copied everywhere.
package config

import (
	"flag"
	"log"
	"os"

	"github.com/ilyakaznacheev/cleanenv"
)

// Config is the root configuration structure.
// Every field maps to a key in the YAML file AND can be overridden
// by the corresponding environment variable (env:"...").
//
// env-required:"true" means the app refuses to start if that value is
// missing — better to crash at boot than to silently use a wrong default.
type Config struct {
	// Env controls log format, verbosity, and feature flags.
	// Valid values: "dev", "staging", "prod"
	Env string `yaml:"env" env:"ENV" env-required:"true"`

	// StoragePath is the filesystem path to the SQLite .db file.
	StoragePath string `yaml:"storage_path" env:"STORAGE_PATH" env-required:"true"`

	// HTTPServer is embedded (not a pointer) so its fields are accessible
	// directly on Config:  cfg.HTTPServer.Addr  or after promotion cfg.Addr
	HTTPServer `yaml:"http_server"`
}

// HTTPServer holds settings specific to the HTTP server.
// Nested under http_server: in the YAML file.
type HTTPServer struct {
	// Addr is the TCP address the server listens on, e.g. "localhost:8082".
	Addr string `yaml:"address" env:"HTTP_SERVER_ADDR" env-required:"true"`
}

// MustLoad reads, validates, and returns the application config.
//
// The name "MustLoad" follows a Go convention: functions prefixed with
// "Must" are allowed to panic/fatal on failure. Callers do not need to
// check a returned error — if this function returns, the config is valid.
func MustLoad() *Config {
	var configPath string

	// ── Source 1: environment variable ───────────────────────────────
	// Useful in Docker / Kubernetes where env vars are the standard way
	// to pass config to a container.
	configPath = os.Getenv("CONFIG_PATH")

	// ── Source 2: command-line flag ───────────────────────────────────
	// Useful when running locally:
	//   go run ./cmd/students-api --config=config/local.yaml
	if configPath == "" {
		// flag.String registers a new string flag.
		// Arguments: name, default-value, usage-description
		flags := flag.String("config", "", "Path to the configuration YAML file")
		flag.Parse()        // actually reads os.Args and populates registered flags
		configPath = *flags // dereference pointer to get the string value
	}

	// Neither source provided a path — we cannot continue.
	if configPath == "" {
		log.Fatal("config path is not set: use --config flag or CONFIG_PATH env var")
	}

	// Verify the file exists before trying to read it.
	// os.Stat returns file info; if it errors with IsNotExist we give a
	// clear message rather than a cryptic "open: no such file" later.
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Fatalf("config file does not exist: %s", configPath)
	}

	// cleanenv.ReadConfig reads the YAML file and populates the struct.
	// It also reads any env:"..." tagged fields from the environment,
	// and validates env-required:"true" constraints.
	var cfg Config
	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		log.Fatalf("cannot read config: %s", err.Error())
	}

	return &cfg
}
