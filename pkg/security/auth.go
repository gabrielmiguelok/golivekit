package security

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Common auth errors.
var (
	ErrUnauthorized    = errors.New("unauthorized")
	ErrForbidden       = errors.New("forbidden")
	ErrSessionExpired  = errors.New("session expired")
	ErrInvalidSession  = errors.New("invalid session")
	ErrUserNotFound    = errors.New("user not found")
)

// AuthContext contains authentication information.
type AuthContext struct {
	// UserID is the unique identifier for the user.
	UserID string

	// Username is the user's display name.
	Username string

	// Email is the user's email address.
	Email string

	// Roles are the user's assigned roles.
	Roles []string

	// Permissions are the user's specific permissions.
	Permissions []string

	// SessionID is the current session identifier.
	SessionID string

	// ExpiresAt is when the session expires.
	ExpiresAt time.Time

	// Metadata contains additional user data.
	Metadata map[string]any
}

// HasRole returns true if the user has the specified role.
func (ac *AuthContext) HasRole(role string) bool {
	for _, r := range ac.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// HasAnyRole returns true if the user has any of the specified roles.
func (ac *AuthContext) HasAnyRole(roles ...string) bool {
	for _, role := range roles {
		if ac.HasRole(role) {
			return true
		}
	}
	return false
}

// HasAllRoles returns true if the user has all specified roles.
func (ac *AuthContext) HasAllRoles(roles ...string) bool {
	for _, role := range roles {
		if !ac.HasRole(role) {
			return false
		}
	}
	return true
}

// HasPermission returns true if the user has the specified permission.
func (ac *AuthContext) HasPermission(perm string) bool {
	for _, p := range ac.Permissions {
		if p == perm {
			return true
		}
	}
	return false
}

// IsExpired returns true if the session has expired.
func (ac *AuthContext) IsExpired() bool {
	return time.Now().After(ac.ExpiresAt)
}

// IsAuthenticated returns true if there is a valid, non-expired user.
func (ac *AuthContext) IsAuthenticated() bool {
	return ac != nil && ac.UserID != "" && !ac.IsExpired()
}

// Context key for auth context.
type authContextKey struct{}

// WithAuthContext adds authentication context to a context.
func WithAuthContext(ctx context.Context, auth *AuthContext) context.Context {
	return context.WithValue(ctx, authContextKey{}, auth)
}

// AuthFromContext retrieves authentication context.
func AuthFromContext(ctx context.Context) *AuthContext {
	ac, _ := ctx.Value(authContextKey{}).(*AuthContext)
	return ac
}

// CurrentUser retrieves the current authenticated user.
// Returns nil if not authenticated.
func CurrentUser(ctx context.Context) *AuthContext {
	return AuthFromContext(ctx)
}

// IsAuthenticated returns true if the context has a valid authenticated user.
func IsAuthenticated(ctx context.Context) bool {
	ac := AuthFromContext(ctx)
	return ac != nil && ac.IsAuthenticated()
}

// RequireAuth middleware requires authentication.
func RequireAuth(onUnauthorized func(http.ResponseWriter, *http.Request)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !IsAuthenticated(r.Context()) {
				if onUnauthorized != nil {
					onUnauthorized(w, r)
				} else {
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
				}
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireRoles middleware requires specific roles.
func RequireRoles(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ac := AuthFromContext(r.Context())
			if ac == nil || !ac.HasAnyRole(roles...) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequirePermission middleware requires a specific permission.
func RequirePermission(perm string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ac := AuthFromContext(r.Context())
			if ac == nil || !ac.HasPermission(perm) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// SessionStore interface for session storage.
type SessionStore interface {
	Get(sessionID string) (*AuthContext, error)
	Set(sessionID string, auth *AuthContext) error
	Delete(sessionID string) error
	Cleanup() error
}

// MemorySessionStore is an in-memory session store.
type MemorySessionStore struct {
	sessions map[string]*AuthContext
	mu       sync.RWMutex
}

// NewMemorySessionStore creates a new memory session store.
func NewMemorySessionStore() *MemorySessionStore {
	return &MemorySessionStore{
		sessions: make(map[string]*AuthContext),
	}
}

// Get retrieves a session.
func (s *MemorySessionStore) Get(sessionID string) (*AuthContext, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	auth, ok := s.sessions[sessionID]
	if !ok {
		return nil, ErrSessionExpired
	}

	if auth.IsExpired() {
		return nil, ErrSessionExpired
	}

	return auth, nil
}

// Set stores a session.
func (s *MemorySessionStore) Set(sessionID string, auth *AuthContext) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sessionID] = auth
	return nil
}

// Delete removes a session.
func (s *MemorySessionStore) Delete(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
	return nil
}

// Cleanup removes expired sessions.
func (s *MemorySessionStore) Cleanup() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, auth := range s.sessions {
		if auth.IsExpired() {
			delete(s.sessions, id)
		}
	}
	return nil
}

// SessionManager manages user sessions.
type SessionManager struct {
	store         SessionStore
	cookieName    string
	cookiePath    string
	cookieDomain  string
	cookieSecure  bool
	sessionTTL    time.Duration
}

// SessionManagerConfig configures the session manager.
type SessionManagerConfig struct {
	Store        SessionStore
	CookieName   string
	CookiePath   string
	CookieDomain string
	CookieSecure bool
	SessionTTL   time.Duration
}

// NewSessionManager creates a new session manager.
func NewSessionManager(config SessionManagerConfig) *SessionManager {
	if config.Store == nil {
		config.Store = NewMemorySessionStore()
	}
	if config.CookieName == "" {
		config.CookieName = "session"
	}
	if config.CookiePath == "" {
		config.CookiePath = "/"
	}
	if config.SessionTTL == 0 {
		config.SessionTTL = 24 * time.Hour
	}

	return &SessionManager{
		store:        config.Store,
		cookieName:   config.CookieName,
		cookiePath:   config.CookiePath,
		cookieDomain: config.CookieDomain,
		cookieSecure: config.CookieSecure,
		sessionTTL:   config.SessionTTL,
	}
}

// Middleware adds session context to requests.
func (sm *SessionManager) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get session ID from cookie
			cookie, err := r.Cookie(sm.cookieName)
			if err == nil && cookie.Value != "" {
				// Load session
				auth, err := sm.store.Get(cookie.Value)
				if err == nil && auth != nil {
					// Add to context
					ctx := WithAuthContext(r.Context(), auth)
					r = r.WithContext(ctx)
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Login creates a session for a user.
func (sm *SessionManager) Login(w http.ResponseWriter, auth *AuthContext) (string, error) {
	// Generate session ID
	sessionID := generateSessionID()

	// Set expiration
	auth.SessionID = sessionID
	auth.ExpiresAt = time.Now().Add(sm.sessionTTL)

	// Store session
	if err := sm.store.Set(sessionID, auth); err != nil {
		return "", err
	}

	// Set cookie
	http.SetCookie(w, &http.Cookie{
		Name:     sm.cookieName,
		Value:    sessionID,
		Path:     sm.cookiePath,
		Domain:   sm.cookieDomain,
		MaxAge:   int(sm.sessionTTL.Seconds()),
		Secure:   sm.cookieSecure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	return sessionID, nil
}

// Logout destroys a session.
func (sm *SessionManager) Logout(w http.ResponseWriter, r *http.Request) error {
	cookie, err := r.Cookie(sm.cookieName)
	if err != nil {
		return nil
	}

	// Delete from store
	sm.store.Delete(cookie.Value)

	// Clear cookie
	http.SetCookie(w, &http.Cookie{
		Name:     sm.cookieName,
		Value:    "",
		Path:     sm.cookiePath,
		Domain:   sm.cookieDomain,
		MaxAge:   -1,
		Secure:   sm.cookieSecure,
		HttpOnly: true,
	})

	return nil
}

// generateSessionID creates a secure random session ID.
func generateSessionID() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

// BearerToken extracts a bearer token from the Authorization header.
func BearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(auth, "Bearer ")
}

// BasicAuth extracts basic auth credentials.
func BasicAuth(r *http.Request) (username, password string, ok bool) {
	return r.BasicAuth()
}
