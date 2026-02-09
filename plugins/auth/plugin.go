// Package auth provides authentication plugin for GoliveKit.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Common errors.
var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrSessionNotFound    = errors.New("session not found")
	ErrSessionExpired     = errors.New("session expired")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrProviderNotFound   = errors.New("auth provider not found")
)

// Plugin is the authentication plugin.
type Plugin struct {
	providers map[string]Provider
	sessions  SessionStore
	config    *Config
	mu        sync.RWMutex
}

// Config configures the auth plugin.
type Config struct {
	SessionTTL      time.Duration
	CookieName      string
	CookieSecure    bool
	CookieHTTPOnly  bool
	CookieSameSite  http.SameSite
	LoginPath       string
	LogoutPath      string
	CallbackPath    string
	DefaultRedirect string
}

// DefaultConfig returns default configuration.
func DefaultConfig() *Config {
	return &Config{
		SessionTTL:      24 * time.Hour,
		CookieName:      "golivekit_session",
		CookieSecure:    true,
		CookieHTTPOnly:  true,
		CookieSameSite:  http.SameSiteLaxMode,
		LoginPath:       "/auth/login",
		LogoutPath:      "/auth/logout",
		CallbackPath:    "/auth/callback",
		DefaultRedirect: "/",
	}
}

// New creates a new auth plugin.
func New(config *Config) *Plugin {
	if config == nil {
		config = DefaultConfig()
	}
	return &Plugin{
		providers: make(map[string]Provider),
		sessions:  NewMemorySessionStore(),
		config:    config,
	}
}

// Provider represents an authentication provider.
type Provider interface {
	Name() string
	Authenticate(ctx context.Context, credentials map[string]string) (*User, error)
	// For OAuth providers
	AuthURL(state string) string
	Exchange(ctx context.Context, code string) (*User, error)
}

// User represents an authenticated user.
type User struct {
	ID          string
	Email       string
	Name        string
	AvatarURL   string
	Roles       []string
	Permissions []string
	Metadata    map[string]any
	CreatedAt   time.Time
}

// Session represents a user session.
type Session struct {
	ID        string
	UserID    string
	User      *User
	Data      map[string]any
	CreatedAt time.Time
	ExpiresAt time.Time
}

// IsExpired checks if the session is expired.
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// SessionStore is the interface for session storage.
type SessionStore interface {
	Get(ctx context.Context, id string) (*Session, error)
	Set(ctx context.Context, session *Session) error
	Delete(ctx context.Context, id string) error
	DeleteByUser(ctx context.Context, userID string) error
}

// RegisterProvider registers an authentication provider.
func (p *Plugin) RegisterProvider(provider Provider) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.providers[provider.Name()] = provider
}

// Authenticate authenticates with credentials.
func (p *Plugin) Authenticate(ctx context.Context, providerName string, credentials map[string]string) (*Session, error) {
	p.mu.RLock()
	provider, ok := p.providers[providerName]
	p.mu.RUnlock()

	if !ok {
		return nil, ErrProviderNotFound
	}

	user, err := provider.Authenticate(ctx, credentials)
	if err != nil {
		return nil, err
	}

	return p.createSession(ctx, user)
}

// GetSession retrieves a session by ID.
func (p *Plugin) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	session, err := p.sessions.Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	if session.IsExpired() {
		p.sessions.Delete(ctx, sessionID)
		return nil, ErrSessionExpired
	}

	return session, nil
}

// Logout invalidates a session.
func (p *Plugin) Logout(ctx context.Context, sessionID string) error {
	return p.sessions.Delete(ctx, sessionID)
}

// LogoutAll invalidates all sessions for a user.
func (p *Plugin) LogoutAll(ctx context.Context, userID string) error {
	return p.sessions.DeleteByUser(ctx, userID)
}

func (p *Plugin) createSession(ctx context.Context, user *User) (*Session, error) {
	sessionID, err := generateSessionID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate session ID: %w", err)
	}

	session := &Session{
		ID:        sessionID,
		UserID:    user.ID,
		User:      user,
		Data:      make(map[string]any),
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(p.config.SessionTTL),
	}

	if err := p.sessions.Set(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to save session: %w", err)
	}

	return session, nil
}

// Middleware returns authentication middleware.
func (p *Plugin) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get session from cookie
			cookie, err := r.Cookie(p.config.CookieName)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			// Get session
			session, err := p.GetSession(r.Context(), cookie.Value)
			if err != nil {
				// Clear invalid cookie
				http.SetCookie(w, &http.Cookie{
					Name:     p.config.CookieName,
					Value:    "",
					MaxAge:   -1,
					Path:     "/",
					HttpOnly: p.config.CookieHTTPOnly,
					Secure:   p.config.CookieSecure,
					SameSite: p.config.CookieSameSite,
				})
				next.ServeHTTP(w, r)
				return
			}

			// Add session to context
			ctx := contextWithSession(r.Context(), session)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAuth requires authentication.
func (p *Plugin) RequireAuth(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session := SessionFromContext(r.Context())
			if session == nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Check roles if specified
			if len(roles) > 0 {
				hasRole := false
				for _, required := range roles {
					for _, userRole := range session.User.Roles {
						if userRole == required {
							hasRole = true
							break
						}
					}
					if hasRole {
						break
					}
				}
				if !hasRole {
					http.Error(w, "Forbidden", http.StatusForbidden)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// SetSessionCookie sets the session cookie.
func (p *Plugin) SetSessionCookie(w http.ResponseWriter, session *Session) {
	http.SetCookie(w, &http.Cookie{
		Name:     p.config.CookieName,
		Value:    session.ID,
		Expires:  session.ExpiresAt,
		Path:     "/",
		HttpOnly: p.config.CookieHTTPOnly,
		Secure:   p.config.CookieSecure,
		SameSite: p.config.CookieSameSite,
	})
}

// ClearSessionCookie clears the session cookie.
func (p *Plugin) ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     p.config.CookieName,
		Value:    "",
		MaxAge:   -1,
		Path:     "/",
		HttpOnly: p.config.CookieHTTPOnly,
		Secure:   p.config.CookieSecure,
		SameSite: p.config.CookieSameSite,
	})
}

// Context helpers

type sessionContextKey struct{}

func contextWithSession(ctx context.Context, session *Session) context.Context {
	return context.WithValue(ctx, sessionContextKey{}, session)
}

// SessionFromContext retrieves the session from context.
func SessionFromContext(ctx context.Context) *Session {
	session, _ := ctx.Value(sessionContextKey{}).(*Session)
	return session
}

// UserFromContext retrieves the user from context.
func UserFromContext(ctx context.Context) *User {
	session := SessionFromContext(ctx)
	if session == nil {
		return nil
	}
	return session.User
}

// MemorySessionStore is an in-memory session store.
type MemorySessionStore struct {
	sessions map[string]*Session
	byUser   map[string][]string // userID -> []sessionID
	mu       sync.RWMutex
}

// NewMemorySessionStore creates a new memory session store.
func NewMemorySessionStore() *MemorySessionStore {
	ms := &MemorySessionStore{
		sessions: make(map[string]*Session),
		byUser:   make(map[string][]string),
	}
	go ms.cleanup()
	return ms
}

// Get retrieves a session.
func (ms *MemorySessionStore) Get(ctx context.Context, id string) (*Session, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	session, ok := ms.sessions[id]
	if !ok {
		return nil, ErrSessionNotFound
	}

	return session, nil
}

// Set stores a session.
func (ms *MemorySessionStore) Set(ctx context.Context, session *Session) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.sessions[session.ID] = session

	// Track by user
	ms.byUser[session.UserID] = append(ms.byUser[session.UserID], session.ID)

	return nil
}

// Delete removes a session.
func (ms *MemorySessionStore) Delete(ctx context.Context, id string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	session, ok := ms.sessions[id]
	if !ok {
		return nil
	}

	delete(ms.sessions, id)

	// Remove from user tracking
	if sessions, ok := ms.byUser[session.UserID]; ok {
		for i, sid := range sessions {
			if sid == id {
				ms.byUser[session.UserID] = append(sessions[:i], sessions[i+1:]...)
				break
			}
		}
	}

	return nil
}

// DeleteByUser removes all sessions for a user.
func (ms *MemorySessionStore) DeleteByUser(ctx context.Context, userID string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if sessions, ok := ms.byUser[userID]; ok {
		for _, sid := range sessions {
			delete(ms.sessions, sid)
		}
		delete(ms.byUser, userID)
	}

	return nil
}

func (ms *MemorySessionStore) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		ms.mu.Lock()
		now := time.Now()
		for id, session := range ms.sessions {
			if now.After(session.ExpiresAt) {
				delete(ms.sessions, id)
				// Remove from user tracking
				if sessions, ok := ms.byUser[session.UserID]; ok {
					for i, sid := range sessions {
						if sid == id {
							ms.byUser[session.UserID] = append(sessions[:i], sessions[i+1:]...)
							break
						}
					}
				}
			}
		}
		ms.mu.Unlock()
	}
}

func generateSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// PasswordProvider is a basic username/password provider.
type PasswordProvider struct {
	name    string
	verify  func(ctx context.Context, username, password string) (*User, error)
}

// NewPasswordProvider creates a new password provider.
func NewPasswordProvider(name string, verify func(ctx context.Context, username, password string) (*User, error)) *PasswordProvider {
	return &PasswordProvider{
		name:   name,
		verify: verify,
	}
}

// Name returns the provider name.
func (p *PasswordProvider) Name() string {
	return p.name
}

// Authenticate authenticates with username and password.
func (p *PasswordProvider) Authenticate(ctx context.Context, credentials map[string]string) (*User, error) {
	username := credentials["username"]
	password := credentials["password"]

	if username == "" || password == "" {
		return nil, ErrInvalidCredentials
	}

	return p.verify(ctx, username, password)
}

// AuthURL is not supported for password provider.
func (p *PasswordProvider) AuthURL(state string) string {
	return ""
}

// Exchange is not supported for password provider.
func (p *PasswordProvider) Exchange(ctx context.Context, code string) (*User, error) {
	return nil, errors.New("exchange not supported for password provider")
}
