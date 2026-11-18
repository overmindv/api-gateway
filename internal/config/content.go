package config

import (
	"errors"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

func loadContent(production bool) (Content, error) {
	cfg := Content{
		Address: strings.TrimSpace(os.Getenv("CONTENT_GRPC_ADDR")), ServiceToken: os.Getenv("CONTENT_GRPC_SERVICE_TOKEN"),
		Timeout: 3 * time.Second,
	}
	if cfg.Address == "" && cfg.ServiceToken == "" {
		return cfg, nil
	}
	host, port, err := net.SplitHostPort(cfg.Address)
	number, portErr := strconv.ParseUint(port, 10, 16)
	if err != nil || host == "" || portErr != nil || number == 0 || strings.ContainsAny(host, "/@?#") ||
		strings.IndexFunc(host, func(r rune) bool { return r <= 32 || r >= 127 }) >= 0 {
		return Content{}, errors.New("CONTENT_GRPC_ADDR must be a host and TCP port")
	}
	if len(cfg.ServiceToken) < 32 || len(cfg.ServiceToken) > 256 ||
		strings.IndexFunc(cfg.ServiceToken, func(r rune) bool { return r <= 32 || r >= 127 }) >= 0 {
		return Content{}, errors.New("CONTENT_GRPC_SERVICE_TOKEN must be a separate printable service token")
	}
	if raw := os.Getenv("CONTENT_GRPC_TIMEOUT"); raw != "" {
		cfg.Timeout, err = time.ParseDuration(raw)
		if err != nil || cfg.Timeout <= 0 || cfg.Timeout > 10*time.Second {
			return Content{}, errors.New("CONTENT_GRPC_TIMEOUT must be positive and at most 10s")
		}
	}
	if raw := os.Getenv("CONTENT_GRPC_INSECURE"); raw != "" {
		cfg.Insecure, err = strconv.ParseBool(raw)
		if err != nil || (cfg.Insecure && production) {
			return Content{}, errors.New("CONTENT_GRPC_INSECURE is only allowed outside production")
		}
	}
	return cfg, nil
}
