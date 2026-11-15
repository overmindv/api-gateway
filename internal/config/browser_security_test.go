package config

import "testing"

func TestProductionBrowserConfiguration(t *testing.T) {
	t.Setenv("TASK_HUNTER_TOKEN", "test")
	t.Setenv("MEDIA_TOKEN", "test")
	t.Setenv("FEED_TOKEN", "test")
	t.Setenv("APP_ENV", "production")
	t.Setenv("CORS_ORIGINS", "https://learn.example")
	t.Setenv("SESSION_COOKIE_SECURE", "true")
	t.Setenv("TRUSTED_PROXY_CIDRS", "10.0.0.0/24")
	cfg, err := Load()
	if err != nil || !cfg.HTTP.Production || len(cfg.HTTP.TrustedProxies) != 1 {
		t.Fatalf("config: %+v %v", cfg.HTTP, err)
	}
	for _, tc := range []struct{ key, value string }{
		{"SESSION_COOKIE_SECURE", "false"}, {"APP_ENV", "prodution"},
		{"CORS_ORIGINS", "*"}, {"CORS_ORIGINS", "https://*.example"}, {"CORS_ORIGINS", "http://learn.example"},
		{"CORS_ORIGINS", "https://user:password@learn.example"}, {"CORS_ORIGINS", "https://learn.example/path"},
		{"CORS_ORIGINS", "https://learn.example?query"}, {"TRUSTED_PROXY_CIDRS", "0.0.0.0/0"}, {"TRUSTED_PROXY_CIDRS", "bad"},
	} {
		t.Run(tc.key+tc.value, func(t *testing.T) {
			t.Setenv(tc.key, tc.value)
			if _, err := Load(); err == nil {
				t.Fatal("unsafe configuration accepted")
			}
		})
	}
}
