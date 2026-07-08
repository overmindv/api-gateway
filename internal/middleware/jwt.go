package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/overmindv/laserbeak/internal/apperror"
)

type JWTAuthenticator struct {
	secret []byte
	issuer string
	now    func() time.Time
}

func NewJWTAuthenticator(secret, issuer string) *JWTAuthenticator {
	return &JWTAuthenticator{
		secret: []byte(secret),
		issuer: issuer,
		now:    time.Now,
	}
}

func (a *JWTAuthenticator) Parse(header string) (AuthInfo, error) {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return AuthInfo{}, apperror.ErrUnauthenticated
	}

	claims := &jwt.RegisteredClaims{}

	token, err := jwt.ParseWithClaims(parts[1], claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected jwt signing method")
		}
		return a.secret, nil
	}, jwt.WithIssuer(a.issuer), jwt.WithExpirationRequired(), jwt.WithIssuedAt(), jwt.WithTimeFunc(a.now))
	if err != nil || !token.Valid || claims.Subject == "" {
		return AuthInfo{}, apperror.ErrUnauthenticated
	}

	return AuthInfo{
		UserID: claims.Subject,
		Token:  parts[1],
	}, nil
}

func JWT(authenticator *JWTAuthenticator, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if header == "" {
			next.ServeHTTP(w, r)
			return
		}

		info, err := authenticator.Parse(header)
		if err != nil {
			info.Err = err
		}

		next.ServeHTTP(w, r.WithContext(ContextWithAuth(r.Context(), info)))
	})
}

func RequireAuth(ctx context.Context) (AuthInfo, error) {
	info, ok := Auth(ctx)
	if !ok || info.Err != nil || info.UserID == "" || info.Token == "" {
		return AuthInfo{}, apperror.ErrUnauthenticated
	}

	return info, nil
}
