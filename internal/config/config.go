package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTP  HTTP
	Arcee Arcee
	JWT   JWT
}

type HTTP struct {
	Address         string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
	CORSOrigins     []string
}

type Arcee struct {
	GraphQLURL string
	HealthURL  string
	Timeout    time.Duration
}
type JWT struct {
	Secret string
	Issuer string
}

func Load() (Config, error) {
	port := strings.TrimPrefix(env("PORT", "8081"), ":")
	graphqlURL := env("ARCEE_GRAPHQL_URL", "http://localhost:8080/query")

	cfg := Config{
		HTTP: HTTP{
			Address:         ":" + port,
			ReadTimeout:     envDuration("HTTP_READ_TIMEOUT", 5*time.Second),
			WriteTimeout:    envDuration("HTTP_WRITE_TIMEOUT", 15*time.Second),
			IdleTimeout:     envDuration("HTTP_IDLE_TIMEOUT", 60*time.Second),
			ShutdownTimeout: envDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
			CORSOrigins:     envCSV("CORS_ORIGINS", []string{"http://localhost:5173"}),
		},
		Arcee: Arcee{
			GraphQLURL: graphqlURL,
			HealthURL:  env("ARCEE_HEALTH_URL", deriveHealthURL(graphqlURL)),
			Timeout:    envDuration("ARCEE_TIMEOUT", 5*time.Second),
		},
		JWT: JWT{
			Secret: env("JWT_SECRET", "local-development-secret-change-me"),
			Issuer: env("JWT_ISSUER", "arcee"),
		},
	}

	if _, err := strconv.ParseUint(port, 10, 16); err != nil {
		return Config{}, fmt.Errorf("PORT must be a valid TCP port: %w", err)
	}

	parsedGraphQLURL, err := url.Parse(cfg.Arcee.GraphQLURL)
	if err != nil {
		return Config{}, fmt.Errorf("ARCEE_GRAPHQL_URL: %w", err)
	}

	if parsedGraphQLURL.Host == "" || (parsedGraphQLURL.Scheme != "http" && parsedGraphQLURL.Scheme != "https") {
		return Config{}, fmt.Errorf("ARCEE_GRAPHQL_URL must be an absolute HTTP(S) URL")
	}

	if cfg.Arcee.Timeout <= 0 {
		return Config{}, fmt.Errorf("ARCEE_TIMEOUT must be positive")
	}

	if cfg.JWT.Secret == "" {
		return Config{}, fmt.Errorf("JWT_SECRET must not be empty")
	}

	return cfg, nil
}

func deriveHealthURL(graphqlURL string) string {
	parsed, err := url.Parse(graphqlURL)
	if err != nil {
		return "http://localhost:8080/health"
	}

	parsed.Path = "/health"
	parsed.RawQuery = ""

	return parsed.String()
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}
func envDuration(key string, fallback time.Duration) time.Duration {
	value, err := time.ParseDuration(env(key, fallback.String()))
	if err != nil {
		return fallback
	}

	return value
}
func envCSV(key string, fallback []string) []string {
	result := make([]string, 0)
	for _, item := range strings.Split(env(key, strings.Join(fallback, ",")), ",") {
		if item = strings.TrimSpace(item); item != "" {
			result = append(result, item)
		}
	}

	if len(result) == 0 {
		return fallback
	}

	return result
}
