package config

import (
	"strings"
	"testing"
)

func TestJWKSConfiguration(t *testing.T) {
	for _, key := range []string{"JWT_JWKS_URL", "JWT_AUDIENCE", "JWT_LEGACY_HS256_UNTIL", "JWT_SECRET", "USERS_LOGIN_CONTEXT_SECRET"} {
		t.Setenv(key, "")
	}
	c := Config{JWT: JWT{Secret: "legacy", Issuer: "users"}}
	if err := configureJWKS(&c); err != nil || c.Users.LoginContextSecret != "legacy" {
		t.Fatal(err)
	}
	t.Setenv("JWT_JWKS_URL", "https://users.example/.well-known/jwks.json")
	t.Setenv("USERS_LOGIN_CONTEXT_SECRET", strings.Repeat("c", 32))
	if err := configureJWKS(&c); err != nil || c.JWT.Secret != "" || c.JWT.Audience != defaultJWTAudience {
		t.Fatal(err)
	}
	t.Setenv("JWT_JWKS_URL", "http://users.example/jwks")
	if err := configureJWKS(&c); err == nil {
		t.Fatal("insecure remote JWKS accepted")
	}
	t.Setenv("JWT_JWKS_URL", "https://users.example/jwks")
	t.Setenv("JWT_SECRET", "legacy")
	if err := configureJWKS(&c); err == nil {
		t.Fatal("unbounded legacy accepted")
	}
}
