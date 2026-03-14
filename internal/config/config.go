// Package config loads application configuration from environment variables.
package config

// Version, Commit, and BuildTime are set at build time via ldflags.
// See the Makefile for the exact -X flags used.
var (
	Version   = "dev"
	Commit    = "none"
	BuildTime = "unknown"
)

// Config holds all runtime configuration for the service.
type Config struct {
	// Port is the HTTP listen port (default "8080").
	Port string

	// Env is the runtime environment: "dev" or "prod".
	Env string

	// APIKey is the shared secret required for /api/v1/* routes.
	APIKey string

	// Version is the build version tag.
	Version string

	// Commit is the short git SHA of the build.
	Commit string

	// BuildTime is the UTC timestamp of the build.
	BuildTime string

	// DatabaseURL is the Postgres connection string.
	DatabaseURL string
}

// Load returns a Config populated from environment variables.
// The getenv function is typically os.Getenv but can be substituted in tests.
func Load(getenv func(string) string) *Config {
	return &Config{
		Port:      envOr(getenv, "PORT", "8080"),
		Env:       envOr(getenv, "ENV", "dev"),
		APIKey:    getenv("API_KEY"),
		Version:   Version,
		Commit:    Commit,
		BuildTime: BuildTime,
		DatabaseURL: getenv("DATABASE_URL"),
	}
}

func envOr(getenv func(string) string, key, fallback string) string {
	if v := getenv(key); v != "" {
		return v
	}
	return fallback
}
