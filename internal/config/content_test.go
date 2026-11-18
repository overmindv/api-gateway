package config

import (
	"testing"
	"time"
)

func TestContentConfig(t *testing.T) {
	t.Setenv("CONTENT_GRPC_ADDR", "")
	t.Setenv("CONTENT_GRPC_SERVICE_TOKEN", "")
	t.Setenv("CONTENT_GRPC_INSECURE", "")
	t.Setenv("CONTENT_GRPC_TIMEOUT", "")
	t.Setenv("CONTENT_GRPC_MODERATION_TOKEN", "")
	if disabled, err := loadContent(false); err != nil || disabled.Address != "" {
		t.Errorf("content should be optional: %v", err)
	}
	t.Setenv("CONTENT_GRPC_ADDR", "content:9090")
	if _, err := loadContent(false); err == nil {
		t.Error("content address accepted without credential")
	}
	t.Setenv("CONTENT_GRPC_SERVICE_TOKEN", "fixture-material-service-credential")
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
	t.Setenv("CONTENT_GRPC_ADDR", "https://content:9090")
	if _, err := loadContent(false); err == nil {
		t.Error("invalid gRPC address accepted")
	}
}

func TestContentModerationConfig(t *testing.T) {
	t.Setenv("CONTENT_GRPC_ADDR", "")
	t.Setenv("CONTENT_GRPC_SERVICE_TOKEN", "")
	t.Setenv("CONTENT_GRPC_MODERATION_TOKEN", "fixture-moderation-service-credential")
	t.Setenv("CONTENT_GRPC_INSECURE", "")
	t.Setenv("CONTENT_GRPC_TIMEOUT", "")
	if _, err := loadContent(false); err == nil {
		t.Error("moderation without Content address was accepted")
	}
	t.Setenv("CONTENT_GRPC_ADDR", "content:9090")
	t.Setenv("CONTENT_GRPC_SERVICE_TOKEN", "fixture-moderation-service-credential")
	if _, err := loadContent(false); err == nil {
		t.Error("reused service credential was accepted")
	}
	t.Setenv("CONTENT_GRPC_SERVICE_TOKEN", "fixture-material-service-credential")
	if cfg, err := loadContent(false); err != nil || cfg.ModerationToken == cfg.ServiceToken {
		t.Errorf("distinct credentials rejected: %v", err)
	}
}
