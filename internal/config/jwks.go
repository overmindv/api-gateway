package config

import (
	"errors"
	"os"
	"strings"
	"time"

	"github.com/overmindv/api-gateway/internal/middleware"
)

const defaultJWTAudience = "overmindv-gateway"

func configureJWKS(c *Config) error {
	c.JWT.JWKSURL = os.Getenv("JWT_JWKS_URL")
	c.JWT.Audience = os.Getenv("JWT_AUDIENCE")
	deadline := os.Getenv("JWT_LEGACY_HS256_UNTIL")
	c.Users.LoginContextSecret = os.Getenv("USERS_LOGIN_CONTEXT_SECRET")
	if c.JWT.JWKSURL == "" {
		if c.JWT.Audience != "" || deadline != "" {
			return errors.New("JWT_JWKS_URL required for asymmetric authentication")
		}
		if c.Users.LoginContextSecret == "" {
			c.Users.LoginContextSecret = c.JWT.Secret
		}

		return nil
	}
	c.JWT.Secret = os.Getenv("JWT_SECRET")
	if c.JWT.Audience == "" {
		c.JWT.Audience = defaultJWTAudience
	}
	c.JWT.LegacyUntil = time.Time{}
	var err error
	if deadline != "" {
		c.JWT.LegacyUntil, err = time.Parse(time.RFC3339, deadline)
		if err != nil {
			return errors.New("invalid legacy JWT deadline")
		}
	}
	secret := c.Users.LoginContextSecret
	if len(secret) < 32 || len(secret) > 4096 || strings.ContainsAny(secret, " \r\n\t") || secret == c.JWT.Secret {
		return errors.New("JWKS requires a separate USERS_LOGIN_CONTEXT_SECRET")
	}
	_, err = middleware.NewJWKSAuthenticator(c.JWT.JWKSURL, c.JWT.Issuer, c.JWT.Audience, c.JWT.Secret, c.JWT.LegacyUntil)

	return err
}
