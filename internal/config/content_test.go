package config

import (
	"testing"
	"time"
)

const contentTestToken = "fixture-material-service-credential"

func TestContentConfig(t *testing.T) {
	t.Setenv("CONTENT_GRPC_ADDR", "")
	t.Setenv("CONTENT_GRPC_SERVICE_TOKEN", "")
	t.Setenv("CONTENT_GRPC_INSECURE", "")
	t.Setenv("CONTENT_GRPC_TIMEOUT", "")
	t.Setenv("CONTENT_GRPC_TLS_CA_FILE", "")
	t.Setenv("CONTENT_GRPC_MODERATION_TOKEN", "")
	if disabled, err := loadContent(false); err != nil || disabled.Address != "" {
		t.Errorf("content should be optional: %v", err)
	}
	t.Setenv("CONTENT_GRPC_ADDR", "content:9090")
	if _, err := loadContent(false); err == nil {
		t.Error("content address accepted without credential")
	}
	t.Setenv("CONTENT_GRPC_SERVICE_TOKEN", contentTestToken)
	if cfg, err := loadContent(true); err != nil || cfg.Insecure || cfg.Timeout != 3*time.Second {
		t.Errorf("expected TLS and default timeout: %v", err)
	}
	t.Setenv("CONTENT_GRPC_INSECURE", "true")
	if _, err := loadContent(true); err == nil {
		t.Error("plaintext accepted in production")
	}
	t.Setenv("CONTENT_GRPC_TIMEOUT", "11s")
	if _, err := loadContent(false); err == nil {
		t.Error("unbounded deadline accepted")
	}
	t.Setenv("CONTENT_GRPC_TIMEOUT", "600ms")
	if cfg, err := loadContent(false); err != nil || !cfg.Insecure || cfg.Timeout != 600*time.Millisecond {
		t.Errorf("local config: %v", err)
	}
	t.Setenv("CONTENT_GRPC_TLS_CA_FILE", "ca.pem")
	if _, err := loadContent(false); err == nil {
		t.Error("CA file accepted with plaintext transport")
	}
	t.Setenv("CONTENT_GRPC_INSECURE", "false")
	if cfg, err := loadContent(true); err != nil || cfg.CAFile != "ca.pem" {
		t.Errorf("expected TLS config with private CA: %v", err)
	}
	t.Setenv("CONTENT_GRPC_ADDR", "https://content:9090")
	if _, err := loadContent(false); err == nil {
		t.Error("invalid gRPC address accepted")
	}
}

func TestModerationTokenConfig(t *testing.T) {
	const moderator = "fixture-moderation-service-credential"
	t.Setenv("CONTENT_GRPC_MODERATION_TOKEN", moderator)
	t.Setenv("CONTENT_GRPC_ADDR", "")
	t.Setenv("CONTENT_GRPC_SERVICE_TOKEN", "")
	t.Setenv("CONTENT_GRPC_INSECURE", "false")
	t.Setenv("CONTENT_GRPC_TIMEOUT", "")
	t.Setenv("CONTENT_GRPC_TLS_CA_FILE", "")
	if _, err := loadContent(false); err == nil {
		t.Error("accepted a moderation token without a Content endpoint")
	}
	t.Setenv("CONTENT_GRPC_ADDR", "content:9090")
	t.Setenv("CONTENT_GRPC_SERVICE_TOKEN", contentTestToken)
	if cfg, err := loadContent(true); err != nil || cfg.ModerationToken != moderator || cfg.Insecure {
		t.Error("valid moderation configuration rejected", err)
	}
	for _, invalid := range []string{"short", contentTestToken, moderator + "\n"} {
		t.Setenv("CONTENT_GRPC_MODERATION_TOKEN", invalid)
		if _, err := loadContent(true); err == nil {
			t.Error("accepted an invalid moderation token")
		}
	}
}
