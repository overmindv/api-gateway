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
	secret       []byte
	issuer       string
	adminUserIDs map[string]struct{}
	now          func() time.Time
}

type claims struct {
	Roles []string `json:"roles"`
	Role  string   `json:"role"`
	jwt.RegisteredClaims
}

func NewJWTAuthenticator(secret, issuer string, adminUserIDLists ...[]string) *JWTAuthenticator {
	adminUserIDs := make(map[string]struct{})
	for _, list := range adminUserIDLists {
		for _, userID := range list {
			adminUserIDs[userID] = struct{}{}
		}
	}

	return &JWTAuthenticator{
		secret:       []byte(secret),
		issuer:       issuer,
		adminUserIDs: adminUserIDs,
		now:          time.Now,
	}
}

func (a *JWTAuthenticator) Parse(header string) (AuthInfo, error) {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return AuthInfo{}, apperror.ErrUnauthenticated
	}

	parsedClaims := &claims{}

	token, err := jwt.ParseWithClaims(parts[1], parsedClaims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected jwt signing method")
		}
		return a.secret, nil
	}, jwt.WithIssuer(a.issuer), jwt.WithExpirationRequired(), jwt.WithIssuedAt(), jwt.WithTimeFunc(a.now))
	if err != nil || !token.Valid || parsedClaims.Subject == "" {
		return AuthInfo{}, apperror.ErrUnauthenticated
	}
	roles := append([]string(nil), parsedClaims.Roles...)
	if parsedClaims.Role != "" {
		roles = append(roles, parsedClaims.Role)
	}
	if _, ok := a.adminUserIDs[parsedClaims.Subject]; ok {
		roles = append(roles, "admin")
	}

	return AuthInfo{
		UserID: parsedClaims.Subject,
		Token:  parts[1],
		Roles:  roles,
	}, nil
}

func RequireAdmin(ctx context.Context) (AuthInfo, error) {
	info, err := RequireAuth(ctx)
	if err != nil {
		return AuthInfo{}, err
	}
	for _, role := range info.Roles {
		if strings.EqualFold(role, "admin") || strings.EqualFold(role, "superuser") {
			return info, nil
		}
	}

	return AuthInfo{}, apperror.ErrPermissionDenied
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
