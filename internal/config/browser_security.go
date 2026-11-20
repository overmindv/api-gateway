package config

import (
	"fmt"
	"net/netip"
	"net/url"
	"os"
	"strings"
)

const (
	httpScheme  = "http"
	httpsScheme = "https"
)

func configureBrowserSecurity(cfg *Config) error {
	environment := env("APP_ENV", "development")
	switch environment {
	case "development", "test":
	case "production":
		cfg.HTTP.Production = true
	default:
		return fmt.Errorf("APP_ENV must be development, test or production")
	}
	// Playground использует origin самого gateway, отличный от Vite/frontend.
	if !cfg.HTTP.Production {
		for _, host := range []string{"localhost", "127.0.0.1", "[::1]"} {
			cfg.HTTP.CORSOrigins = append(cfg.HTTP.CORSOrigins, "http://"+host+cfg.HTTP.Address)
		}
	}
	for _, raw := range strings.Split(os.Getenv("TRUSTED_PROXY_CIDRS"), ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(raw)
		if err != nil || prefix.Bits() == 0 {
			return fmt.Errorf("TRUSTED_PROXY_CIDRS must contain explicit bounded CIDRs")
		}
		cfg.HTTP.TrustedProxies = append(cfg.HTTP.TrustedProxies, prefix.Masked())
	}
	for _, origin := range cfg.HTTP.CORSOrigins {
		parsed, err := url.Parse(origin)
		if err != nil || parsed.Hostname() == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || strings.Contains(origin, "*") || (parsed.Scheme != httpScheme && parsed.Scheme != httpsScheme) {
			return fmt.Errorf("CORS_ORIGINS must contain exact HTTP(S) origins")
		}
		if cfg.HTTP.Production && parsed.Scheme != httpsScheme {
			return fmt.Errorf("production origins must use HTTPS")
		}
	}
	if cfg.HTTP.Production && (!cfg.Session.Secure || len(cfg.HTTP.CORSOrigins) == 0) {
		return fmt.Errorf("production requires Secure cookie and explicit CORS origins")
	}

	return nil
}
