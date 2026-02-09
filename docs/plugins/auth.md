# Auth Plugin

The auth plugin provides authentication and authorization for GoliveKit applications with role-based access control (RBAC).

## Installation

```go
import "github.com/gabrielmiguelok/golivekit/plugins/auth"
```

## Overview

The auth plugin provides:
- Username/password authentication
- OAuth2 provider integration
- Session management with secure tokens
- Role-based access control (RBAC)
- Middleware and context helpers

## Quick Start

```go
// Create auth plugin
authPlugin := auth.New(auth.Config{
    Secret:         []byte("your-32-byte-secret-key-here!!!"),
    SessionTTL:     24 * time.Hour,
    SecureCookies:  true,
})

// Register with application
app.UsePlugin(authPlugin)

// Add provider
authPlugin.AddProvider(auth.NewPasswordProvider(
    userStore,        // implements auth.UserStore
    passwordHasher,   // implements auth.PasswordHasher
))
```

## Providers

### Password Provider

Username/password authentication:

```go
provider := auth.NewPasswordProvider(auth.PasswordConfig{
    UserStore: &MyUserStore{},
    Hasher:    auth.BcryptHasher{Cost: 12},

    // Optional settings
    MaxAttempts:    5,
    LockoutPeriod:  15 * time.Minute,
    MinPasswordLen: 8,
})

authPlugin.AddProvider(provider)
```

### OAuth2 Provider

Support for Google, GitHub, etc.:

```go
// Google OAuth
google := auth.NewOAuth2Provider(auth.OAuth2Config{
    Name:         "google",
    ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
    ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
    AuthURL:      "https://accounts.google.com/o/oauth2/auth",
    TokenURL:     "https://oauth2.googleapis.com/token",
    UserInfoURL:  "https://www.googleapis.com/oauth2/v2/userinfo",
    Scopes:       []string{"email", "profile"},
    RedirectURL:  "https://myapp.com/auth/google/callback",
})

authPlugin.AddProvider(google)

// GitHub OAuth
github := auth.NewOAuth2Provider(auth.OAuth2Config{
    Name:         "github",
    ClientID:     os.Getenv("GITHUB_CLIENT_ID"),
    ClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
    AuthURL:      "https://github.com/login/oauth/authorize",
    TokenURL:     "https://github.com/login/oauth/access_token",
    UserInfoURL:  "https://api.github.com/user",
    Scopes:       []string{"user:email"},
    RedirectURL:  "https://myapp.com/auth/github/callback",
})

authPlugin.AddProvider(github)
```

### Custom Provider

```go
type MyProvider struct{}

func (p *MyProvider) Name() string { return "my-provider" }

func (p *MyProvider) Authenticate(ctx context.Context, credentials map[string]string) (*auth.User, error) {
    // Implement authentication logic
    return &auth.User{
        ID:    "user-123",
        Email: "user@example.com",
        Roles: []string{"user"},
    }, nil
}

authPlugin.AddProvider(&MyProvider{})
```

## User Store

Implement the `UserStore` interface:

```go
type UserStore interface {
    GetByID(ctx context.Context, id string) (*User, error)
    GetByEmail(ctx context.Context, email string) (*User, error)
    Create(ctx context.Context, user *User) error
    Update(ctx context.Context, user *User) error
    Delete(ctx context.Context, id string) error
}

// Example implementation
type SQLUserStore struct {
    db *sql.DB
}

func (s *SQLUserStore) GetByID(ctx context.Context, id string) (*auth.User, error) {
    var user auth.User
    err := s.db.QueryRowContext(ctx,
        "SELECT id, email, password_hash, roles FROM users WHERE id = ?", id,
    ).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Roles)
    if err == sql.ErrNoRows {
        return nil, auth.ErrUserNotFound
    }
    return &user, err
}
```

## Session Management

### Session Creation

```go
// After successful authentication
func (c *LoginComponent) HandleEvent(ctx context.Context, event string, payload map[string]any) error {
    if event == "login" {
        email := payload["email"].(string)
        password := payload["password"].(string)

        user, err := authPlugin.Authenticate(ctx, "password", map[string]string{
            "email":    email,
            "password": password,
        })
        if err != nil {
            return c.setError("Invalid credentials")
        }

        // Create session
        session, err := authPlugin.CreateSession(ctx, user)
        if err != nil {
            return err
        }

        // Set cookie (in HTTP context)
        c.SetCookie(session.Cookie())

        // Redirect
        c.Redirect("/dashboard")
    }
    return nil
}
```

### Session Validation

```go
// Middleware
func AuthMiddleware(authPlugin *auth.Plugin) router.Middleware {
    return func(next router.Handler) router.Handler {
        return router.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            cookie, err := r.Cookie("session")
            if err != nil {
                http.Redirect(w, r, "/login", http.StatusFound)
                return
            }

            session, err := authPlugin.ValidateSession(r.Context(), cookie.Value)
            if err != nil {
                http.Redirect(w, r, "/login", http.StatusFound)
                return
            }

            // Add user to context
            ctx := auth.WithUser(r.Context(), session.User)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

### Session Token Structure

Sessions use HMAC-signed tokens:

```go
// Token contains:
// - User ID
// - Expiration time
// - Session ID (for revocation)
// - HMAC-SHA256 signature
```

## Role-Based Access Control

### Defining Roles

```go
authPlugin := auth.New(auth.Config{
    Roles: auth.RoleConfig{
        "admin": {
            Permissions: []string{"*"}, // All permissions
        },
        "editor": {
            Permissions: []string{
                "articles:read",
                "articles:create",
                "articles:update",
            },
        },
        "viewer": {
            Permissions: []string{"articles:read"},
        },
    },
})
```

### Checking Permissions

```go
// In component
func (c *ArticleEditor) Mount(ctx context.Context, params core.Params, session core.Session) error {
    user := auth.UserFromContext(ctx)

    if !authPlugin.HasPermission(user, "articles:update") {
        return auth.ErrUnauthorized
    }

    // Load article...
    return nil
}
```

### Middleware

```go
// Require specific role
router.Use(auth.RequireRole(authPlugin, "admin"))

// Require specific permission
router.Use(auth.RequirePermission(authPlugin, "articles:create"))

// Custom authorization
router.Use(auth.RequireFunc(func(user *auth.User, r *http.Request) bool {
    // Article owner or admin
    articleID := chi.URLParam(r, "id")
    article := getArticle(articleID)
    return article.AuthorID == user.ID || user.HasRole("admin")
}))
```

## Context Helpers

```go
// Get current user
user := auth.UserFromContext(ctx)
if user == nil {
    // Not authenticated
}

// Check if authenticated
if auth.IsAuthenticated(ctx) {
    // User is logged in
}

// Check role
if auth.HasRole(ctx, "admin") {
    // User is admin
}

// Check permission
if auth.HasPermission(ctx, "articles:delete") {
    // User can delete articles
}
```

## HTTP Endpoints

The plugin registers these endpoints:

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/auth/login` | POST | Login with credentials |
| `/auth/logout` | POST | Logout current session |
| `/auth/session` | GET | Get current session info |
| `/auth/{provider}` | GET | Start OAuth flow |
| `/auth/{provider}/callback` | GET | OAuth callback |

### Custom Endpoints

```go
// Override default endpoints
authPlugin := auth.New(auth.Config{
    Routes: auth.RouteConfig{
        Login:    "/api/auth/login",
        Logout:   "/api/auth/logout",
        Session:  "/api/auth/me",
    },
})
```

## Component Integration

### Login Component

```go
type LoginComponent struct {
    core.BaseComponent
    authPlugin *auth.Plugin
    Error      string
}

func (c *LoginComponent) HandleEvent(ctx context.Context, event string, payload map[string]any) error {
    if event == "submit" {
        email := payload["email"].(string)
        password := payload["password"].(string)

        user, err := c.authPlugin.Authenticate(ctx, "password", map[string]string{
            "email":    email,
            "password": password,
        })

        if err != nil {
            c.Error = "Invalid email or password"
            c.Assigns().Set("error", c.Error)
            return nil
        }

        session, _ := c.authPlugin.CreateSession(ctx, user)
        c.Socket().SetCookie(session.Cookie())
        c.Socket().Redirect("/dashboard")
    }
    return nil
}
```

### Protected Component

```go
type DashboardComponent struct {
    core.BaseComponent
    User *auth.User
}

func (c *DashboardComponent) Mount(ctx context.Context, params core.Params, session core.Session) error {
    c.User = auth.UserFromContext(ctx)
    if c.User == nil {
        c.Socket().Redirect("/login")
        return nil
    }

    c.Assigns().Set("user", c.User)
    return nil
}
```

## Configuration Reference

```go
authPlugin := auth.New(auth.Config{
    // Secret for signing tokens (required, min 32 bytes)
    Secret: []byte("your-32-byte-secret-key-here!!!"),

    // Session settings
    SessionTTL:        24 * time.Hour,
    RefreshThreshold:  time.Hour, // Refresh if less than this remaining

    // Cookie settings
    SecureCookies:     true,      // Require HTTPS
    CookieDomain:      ".example.com",
    CookiePath:        "/",
    CookieSameSite:    http.SameSiteLaxMode,

    // Security
    MaxSessions:       5,         // Max concurrent sessions per user
    RevokeOnLogout:    true,      // Revoke all sessions on logout

    // Hooks
    OnLogin: func(user *auth.User) {
        log.Printf("User %s logged in", user.Email)
    },
    OnLogout: func(user *auth.User) {
        log.Printf("User %s logged out", user.Email)
    },
    OnSessionExpired: func(user *auth.User) {
        log.Printf("Session expired for %s", user.Email)
    },
})
```

## Security Best Practices

1. **Use strong secrets** - Minimum 32 bytes, randomly generated
2. **Enable secure cookies** - Always use HTTPS in production
3. **Limit session duration** - 24 hours is reasonable for most apps
4. **Implement rate limiting** - Prevent brute force attacks
5. **Hash passwords properly** - Use bcrypt with cost >= 12
6. **Validate OAuth state** - Prevent CSRF in OAuth flows
7. **Log authentication events** - Integrate with audit logging
8. **Rotate secrets periodically** - Plan for secret rotation

## Metrics

The plugin exports these metrics:

```
auth_login_total{provider, status}
auth_logout_total
auth_session_created_total
auth_session_validated_total{status}
auth_permission_check_total{permission, result}
```
