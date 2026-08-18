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
	HTTP       HTTP
	Users      Users
	Entities   Entities
	Tasks      Tasks
	TaskHunter TaskHunter
	Media      Media
	JWT        JWT
}

type HTTP struct {
	Address         string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
	CORSOrigins     []string
	RequestLogPath  string
}

type Users struct {
	GraphQLURL string
	HealthURL  string
	Timeout    time.Duration
}
type Entities struct {
	URL     string
	Timeout time.Duration
}

type Tasks struct {
	URL     string
	Timeout time.Duration
}

type TaskHunter struct {
	URL     string
	Token   string
	Timeout time.Duration
}

type Media struct {
	URL     string
	Token   string
	Timeout time.Duration
}

type JWT struct {
	Secret       string
	Issuer       string
	AdminUserIDs []string
}

// Load читает конфигурацию api-gateway из environment.
// На вход не получает параметров, на выход возвращает нормализованный Config или ошибку валидации.
func Load() (Config, error) {
	port := strings.TrimPrefix(env("PORT", "8081"), ":")
	graphqlURL := env("USERS_GRAPHQL_URL", "http://localhost:8080/query")

	cfg := Config{
		HTTP: HTTP{
			Address:         ":" + port,
			ReadTimeout:     envDuration("HTTP_READ_TIMEOUT", 5*time.Second),
			WriteTimeout:    envDuration("HTTP_WRITE_TIMEOUT", 15*time.Second),
			IdleTimeout:     envDuration("HTTP_IDLE_TIMEOUT", 60*time.Second),
			ShutdownTimeout: envDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
			CORSOrigins:     envCSV("CORS_ORIGINS", []string{"http://localhost:5173"}),
			RequestLogPath:  env("REQUEST_LOG_PATH", ""),
		},
		Users: Users{
			GraphQLURL: graphqlURL,
			HealthURL:  env("USERS_HEALTH_URL", deriveHealthURL(graphqlURL)),
			Timeout:    envDuration("USERS_TIMEOUT", 5*time.Second),
		},
		Entities: Entities{
			URL:     env("ENTITIES_URL", "http://localhost:8082"),
			Timeout: envDuration("ENTITIES_TIMEOUT", 5*time.Second),
		},
		Tasks: Tasks{
			URL:     env("TASKS_URL", "http://localhost:8083"),
			Timeout: envDuration("TASKS_TIMEOUT", 5*time.Second),
		},
		TaskHunter: TaskHunter{
			URL:     env("TASK_HUNTER_URL", "http://localhost:8084"),
			Token:   strings.TrimSpace(os.Getenv("TASK_HUNTER_TOKEN")),
			Timeout: envDuration("TASK_HUNTER_TIMEOUT", 10*time.Second),
		},
		Media: Media{
			URL:     env("MEDIA_URL", "http://localhost:8085"),
			Token:   strings.TrimSpace(os.Getenv("MEDIA_TOKEN")),
			Timeout: envDuration("MEDIA_TIMEOUT", 10*time.Second),
		},
		JWT: JWT{
			Secret:       env("JWT_SECRET", "local-development-secret-change-me"),
			Issuer:       env("JWT_ISSUER", "users"),
			AdminUserIDs: envCSV("ADMIN_USER_IDS", nil),
		},
	}

	if _, err := strconv.ParseUint(port, 10, 16); err != nil {
		return Config{}, fmt.Errorf("PORT must be a valid TCP port: %w", err)
	}

	parsedGraphQLURL, err := url.Parse(cfg.Users.GraphQLURL)
	if err != nil {
		return Config{}, fmt.Errorf("USERS_GRAPHQL_URL: %w", err)
	}

	if parsedGraphQLURL.Host == "" || (parsedGraphQLURL.Scheme != "http" && parsedGraphQLURL.Scheme != "https") {
		return Config{}, fmt.Errorf("USERS_GRAPHQL_URL must be an absolute HTTP(S) URL")
	}

	if cfg.Users.Timeout <= 0 {
		return Config{}, fmt.Errorf("USERS_TIMEOUT must be positive")
	}
	parsedEntitiesURL, err := url.Parse(cfg.Entities.URL)
	if err != nil || parsedEntitiesURL.Host == "" || (parsedEntitiesURL.Scheme != "http" && parsedEntitiesURL.Scheme != "https") {
		return Config{}, fmt.Errorf("ENTITIES_URL must be an absolute HTTP(S) URL")
	}
	if cfg.Entities.Timeout <= 0 {
		return Config{}, fmt.Errorf("ENTITIES_TIMEOUT must be positive")
	}
	parsedTasksURL, err := url.Parse(cfg.Tasks.URL)
	if err != nil || parsedTasksURL.Host == "" || (parsedTasksURL.Scheme != "http" && parsedTasksURL.Scheme != "https") {
		return Config{}, fmt.Errorf("TASKS_URL must be an absolute HTTP(S) URL")
	}
	if cfg.Tasks.Timeout <= 0 {
		return Config{}, fmt.Errorf("TASKS_TIMEOUT must be positive")
	}
	parsedTaskHunterURL, err := url.Parse(cfg.TaskHunter.URL)
	if err != nil || parsedTaskHunterURL.Host == "" || (parsedTaskHunterURL.Scheme != "http" && parsedTaskHunterURL.Scheme != "https") {
		return Config{}, fmt.Errorf("TASK_HUNTER_URL must be an absolute HTTP(S) URL")
	}
	if cfg.TaskHunter.Token == "" {
		return Config{}, fmt.Errorf("TASK_HUNTER_TOKEN must not be empty")
	}
	if cfg.TaskHunter.Timeout <= 0 {
		return Config{}, fmt.Errorf("TASK_HUNTER_TIMEOUT must be positive")
	}
	parsedMediaURL, err := url.Parse(cfg.Media.URL)
	if err != nil || parsedMediaURL.Host == "" || (parsedMediaURL.Scheme != "http" && parsedMediaURL.Scheme != "https") {
		return Config{}, fmt.Errorf("MEDIA_URL must be an absolute HTTP(S) URL")
	}
	if cfg.Media.Token == "" {
		return Config{}, fmt.Errorf("MEDIA_TOKEN must not be empty")
	}
	if cfg.Media.Timeout <= 0 {
		return Config{}, fmt.Errorf("MEDIA_TIMEOUT must be positive")
	}

	if cfg.JWT.Secret == "" {
		return Config{}, fmt.Errorf("JWT_SECRET must not be empty")
	}

	return cfg, nil
}

// deriveHealthURL строит health endpoint из GraphQL URL Users.
// На вход получает GraphQL URL, на выход возвращает URL healthcheck endpoint.
func deriveHealthURL(graphqlURL string) string {
	parsed, err := url.Parse(graphqlURL)
	if err != nil {
		return "http://localhost:8080/health"
	}

	parsed.Path = "/health"
	parsed.RawQuery = ""

	return parsed.String()
}

// env возвращает значение environment variable или fallback.
// На вход получает имя переменной и fallback, на выход возвращает непустое значение.
func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

// envDuration читает duration из environment variable.
// На вход получает имя переменной и fallback, на выход возвращает parsed duration или fallback при ошибке.
func envDuration(key string, fallback time.Duration) time.Duration {
	value, err := time.ParseDuration(env(key, fallback.String()))
	if err != nil {
		return fallback
	}

	return value
}

// envCSV читает CSV-список из environment variable.
// На вход получает имя переменной и fallback, на выход возвращает очищенный список или fallback.
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
