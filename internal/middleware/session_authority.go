package middleware

import (
	"context"
	"crypto/sha256"
	"sync"
	"time"

	"github.com/overmindv/api-gateway/internal/apperror"
)

// SessionState содержит только подтверждённый Users контекст доступа.
type SessionState struct {
	UserID    string    `json:"user_id"`
	SessionID string    `json:"session_id"`
	Roles     []string  `json:"roles"`
	ExpiresAt time.Time `json:"expires_at"`
}

// SessionVerifier проверяет и отзывает сессии у их владельца.
type SessionVerifier interface {
	CheckSession(context.Context, string) (SessionState, error)
	RevokeSession(context.Context, string) error
}

type sessionEntry struct {
	state   SessionState
	expires time.Time
}
type sessionCache struct {
	mu     sync.Mutex
	values map[[32]byte]sessionEntry
}

// SetSessionVerifier вызывается при сборке до старта HTTP.
func (a *JWTAuthenticator) SetSessionVerifier(verifier SessionVerifier) {
	a.verifier = verifier
	a.cache = &sessionCache{values: make(map[[32]byte]sessionEntry)}
}

// ParseContext проверяет локальную подпись до обращения к владельцу сессий.
func (a *JWTAuthenticator) ParseContext(ctx context.Context, header string) (AuthInfo, error) {
	info, err := a.parse(ctx, header)
	if err != nil || a.verifier == nil {
		return info, err
	}
	info.authority = a

	return a.checkSession(ctx, info, false)
}

func (a *JWTAuthenticator) checkSession(ctx context.Context, info AuthInfo, fresh bool) (AuthInfo, error) {
	info.freshSession = false
	key := sha256.Sum256([]byte(info.Token))
	now := a.now()
	a.cache.mu.Lock()
	entry, ok := a.cache.values[key]
	a.cache.mu.Unlock()
	if !fresh && ok && now.Before(entry.expires) {
		info.Roles = append([]string(nil), entry.state.Roles...)

		return info, nil
	}
	state, err := a.verifier.CheckSession(ctx, info.Token)
	if err == nil && (state.UserID != info.UserID || state.SessionID == "" || state.SessionID != info.sessionID) {
		err = apperror.ErrUnauthenticated
	}
	a.cache.mu.Lock()
	defer a.cache.mu.Unlock()
	if err != nil {
		delete(a.cache.values, key)
		info.Err = err

		return info, err
	}
	if len(a.cache.values) >= 1024 {
		clear(a.cache.values)
	}
	state.Roles = append([]string(nil), state.Roles...)
	// TTL отсчитывается от начала запроса: медленный check не продлевает stale access.
	a.cache.values[key] = sessionEntry{state: state, expires: now.Add(5 * time.Second)}
	info.Roles = append([]string(nil), state.Roles...)
	info.Err = nil
	info.freshSession = true

	return info, nil
}

// RefreshAuth обновляет контекст перед mutation; ошибка запрещает защищённые резолверы.
func RefreshAuth(ctx context.Context) context.Context {
	info, ok := Auth(ctx)
	if !ok || info.authority == nil {
		return ctx
	}
	info, err := info.authority.checkSession(ctx, info, true)
	info.Err = err

	return ContextWithAuth(ctx, info)
}

// RevokeAuth отзывает предъявленную сессию даже если её проверка ранее вернула ошибку.
func RevokeAuth(ctx context.Context) error {
	info, ok := Auth(ctx)
	if !ok || info.authority == nil || info.Token == "" {
		return nil
	}
	key := sha256.Sum256([]byte(info.Token))
	info.authority.cache.mu.Lock()
	delete(info.authority.cache.values, key)
	info.authority.cache.mu.Unlock()

	return info.authority.verifier.RevokeSession(ctx, info.Token)
}
