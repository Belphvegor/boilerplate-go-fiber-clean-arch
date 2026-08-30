package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"html"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/casper/go-fiber-clean-arch/config"
	authhttp "github.com/casper/go-fiber-clean-arch/internal/auth/delivery/http"
	"github.com/casper/go-fiber-clean-arch/internal/bootstrap"
	userhttp "github.com/casper/go-fiber-clean-arch/internal/user/delivery/http"
	"github.com/casper/go-fiber-clean-arch/pkg/middleware"
)

// The app must listen on exactly the host:port registered as the client's redirect
// URI in deploy/keycloak/realm-clean-arch.json. Keycloak matches redirect URIs by
// exact string, and the cookie jar keys the oidc_* cookies on this host, so
// "localhost" and "127.0.0.1" are not interchangeable here.
const (
	appHost     = "localhost:8080"
	appBaseURL  = "http://" + appHost
	callbackURL = appBaseURL + "/api/v1/auth/callback"
)

// Keycloak renders the login form with the POST target in an action attribute
// pointing at .../login-actions/authenticate.
var loginActionRe = regexp.MustCompile(`action="([^"]*login-actions/authenticate[^"]*)"`)

// TestKeycloakLoginFlow drives the real Authorization Code + PKCE flow against a live
// Keycloak: login redirect -> Keycloak credential form -> callback -> ID token
// verification -> user provisioning -> locally signed API token -> protected route.
//
// Prerequisites (see docs/configuration.md):
//
//	docker compose up -d database
//	docker compose --profile keycloak up -d keycloak
//	MIGRATION_CMD="migrate -path=/migrations -database postgres://app:app@database:5432/app?sslmode=disable up" \
//	  docker compose --profile migrate run --rm migrator
//	KEYCLOAK_E2E=true go test ./test/integration -run TestKeycloak -v
func TestKeycloakLoginFlow(t *testing.T) {
	if os.Getenv("KEYCLOAK_E2E") != "true" {
		t.Skip("keycloak e2e disabled; set KEYCLOAK_E2E=true to enable")
	}

	cfg := keycloakConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	container, err := bootstrap.Build(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Shutdown(context.Background()) })

	app := fiber.New()
	middleware.Register(app, cfg, container.Logger)
	require.NoError(t, container.RegisterRoutes(app, authhttp.Register, userhttp.Register))
	// cmd/api/main.go registers this outside the modules; the post-logout redirect
	// below lands here, so the test app needs it too.
	app.Get("/healthz", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	ln, err := net.Listen("tcp", appHost)
	require.NoError(t, err, "port 8080 must be free; stop any locally running API first")
	go func() { _ = app.Listener(ln) }()
	t.Cleanup(func() { _ = app.ShutdownWithContext(context.Background()) })
	requireReachable(t, appBaseURL+"/healthz")

	cleanupUser(t, container, "ada@example.com")
	cleanupUser(t, container, "grace@example.com")

	t.Run("verified user logs in and receives a usable API token", func(t *testing.T) {
		body, status := keycloakLogin(t, "ada", "ada")
		require.Equal(t, http.StatusOK, status, "callback body: %s", body)

		var callback struct {
			Data tokenPair `json:"data"`
		}
		require.NoError(t, json.Unmarshal([]byte(body), &callback))

		data := callback.Data
		assert.Equal(t, "Bearer", data.TokenType)
		assert.Equal(t, int64(cfg.JWT.AccessTTL.Seconds()), data.ExpiresIn)
		assert.Equal(t, "ada@example.com", data.User.Email)
		assert.Equal(t, "Ada Lovelace", data.User.Name)
		require.NotEmpty(t, data.User.ID)

		// The token must verify under our own secret, not Keycloak's, and carry the
		// claims pkg/middleware.JWTProtected checks.
		claims := jwt.MapClaims{}
		token, err := jwt.ParseWithClaims(data.AccessToken, claims,
			func(*jwt.Token) (interface{}, error) { return []byte(cfg.JWT.Secret), nil },
			jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
			jwt.WithIssuer(cfg.JWT.Issuer),
			jwt.WithAudience(cfg.JWT.Audience),
			jwt.WithExpirationRequired(),
		)
		require.NoError(t, err)
		require.True(t, token.Valid)
		assert.Equal(t, data.User.ID, claims["sub"])
		assert.Equal(t, "ada@example.com", claims["email"])
		assert.Equal(t, cfg.OIDC.IssuerURL, claims["provider"])
		assert.NotEmpty(t, claims["provider_subject"])

		// JWT middleware is enabled, so this proves the issued token opens a guarded route.
		req, err := http.NewRequest(http.MethodGet, appBaseURL+"/api/v1/users/"+data.User.ID, nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+data.AccessToken)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode, "issued token must open a JWT-guarded route")

		// Same route without the token must stay closed.
		unauth, err := http.Get(appBaseURL + "/api/v1/users/" + data.User.ID)
		require.NoError(t, err)
		defer unauth.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, unauth.StatusCode)
	})

	t.Run("second login reuses the same local user", func(t *testing.T) {
		body, status := keycloakLogin(t, "ada", "ada")
		require.Equal(t, http.StatusOK, status, "callback body: %s", body)

		var callback struct {
			Data struct {
				User struct {
					ID string `json:"id"`
				} `json:"user"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal([]byte(body), &callback))

		var count int
		require.NoError(t, container.DB.SQL.Get(&count,
			container.DB.SQL.Rebind("SELECT COUNT(*) FROM auth_identities WHERE email = ?"),
			"ada@example.com"))
		assert.Equal(t, 1, count, "repeat login must reuse the identity, not create a second one")
	})

	t.Run("unverified email is rejected", func(t *testing.T) {
		// grace has emailVerified=false in the realm; auto-provisioning must refuse.
		body, status := keycloakLogin(t, "grace", "grace")
		assert.Equal(t, http.StatusUnauthorized, status, "callback body: %s", body)
		assert.NotContains(t, body, "access_token")

		var count int
		require.NoError(t, container.DB.SQL.Get(&count,
			container.DB.SQL.Rebind("SELECT COUNT(*) FROM users WHERE email = ?"),
			"grace@example.com"))
		assert.Equal(t, 0, count, "rejected login must not leave a provisioned user behind")
	})

	t.Run("refresh rotates the token pair and the old token dies", func(t *testing.T) {
		first := loginTokens(t)

		second := postJSON[tokenPair](t, appBaseURL+"/api/v1/auth/refresh",
			map[string]string{"refresh_token": first.RefreshToken}, http.StatusOK)
		require.NotEmpty(t, second.RefreshToken)
		assert.NotEqual(t, first.RefreshToken, second.RefreshToken, "refresh must rotate")
		assert.NotEqual(t, first.AccessToken, second.AccessToken)
		assert.Equal(t, first.User.ID, second.User.ID)

		// The new access token opens a guarded route.
		req, err := http.NewRequest(http.MethodGet, appBaseURL+"/api/v1/users/"+second.User.ID, nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+second.AccessToken)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Replaying the rotated-away token revokes the whole family, so the token the
		// client legitimately holds stops working too.
		postJSON[struct{}](t, appBaseURL+"/api/v1/auth/refresh",
			map[string]string{"refresh_token": first.RefreshToken}, http.StatusUnauthorized)
		postJSON[struct{}](t, appBaseURL+"/api/v1/auth/refresh",
			map[string]string{"refresh_token": second.RefreshToken}, http.StatusUnauthorized)
	})

	t.Run("logout revokes the session and ends the Keycloak session", func(t *testing.T) {
		tokens := loginTokens(t)

		var result struct {
			LogoutURL string `json:"logout_url"`
		}
		result = postJSON[struct {
			LogoutURL string `json:"logout_url"`
		}](t, appBaseURL+"/api/v1/auth/logout",
			map[string]string{"refresh_token": tokens.RefreshToken}, http.StatusOK)

		require.NotEmpty(t, result.LogoutURL)
		assert.Contains(t, result.LogoutURL, "/protocol/openid-connect/logout")
		assert.Contains(t, result.LogoutURL, "id_token_hint=")

		// The refresh token is dead immediately.
		postJSON[struct{}](t, appBaseURL+"/api/v1/auth/refresh",
			map[string]string{"refresh_token": tokens.RefreshToken}, http.StatusUnauthorized)

		// Visiting the logout URL must end Keycloak's own session with no confirmation
		// page, and land on the registered post-logout redirect.
		jar, err := cookiejar.New(nil)
		require.NoError(t, err)
		client := &http.Client{Jar: localhostJar{jar}, Timeout: 30 * time.Second}
		resp, err := client.Get(result.LogoutURL)
		require.NoError(t, err)
		body := readAll(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, appBaseURL+"/healthz", resp.Request.URL.String(),
			"id_token_hint must make logout silent and redirect, not render a confirmation page; got body %s", truncate(body))
	})

	t.Run("logout ends the Keycloak SSO session, not just ours", func(t *testing.T) {
		browser := newBrowser(t)

		body, status := keycloakLoginWith(t, browser, "ada", "ada")
		require.Equal(t, http.StatusOK, status, "callback body: %s", body)
		var callback struct {
			Data tokenPair `json:"data"`
		}
		require.NoError(t, json.Unmarshal([]byte(body), &callback))

		// Baseline: while Keycloak's SSO cookie lives, a second /auth/login completes
		// with no credential prompt at all. This is exactly why revoking only our own
		// session would be a false logout.
		resp, err := browser.Get(appBaseURL + "/api/v1/auth/login")
		require.NoError(t, err)
		page := readAll(t, resp)
		require.NotContains(t, page, "login-actions/authenticate",
			"expected silent SSO re-login while the Keycloak session is alive")
		require.True(t, strings.HasPrefix(resp.Request.URL.String(), callbackURL))

		// Log out, then walk the provider logout URL in the same browser.
		result := postJSON[struct {
			LogoutURL string `json:"logout_url"`
		}](t, appBaseURL+"/api/v1/auth/logout",
			map[string]string{"refresh_token": callback.Data.RefreshToken}, http.StatusOK)
		require.NotEmpty(t, result.LogoutURL)

		logoutResp, err := browser.Get(result.LogoutURL)
		require.NoError(t, err)
		logoutResp.Body.Close()

		// Now the same browser must be challenged for credentials again.
		resp, err = browser.Get(appBaseURL + "/api/v1/auth/login")
		require.NoError(t, err)
		page = readAll(t, resp)
		assert.Contains(t, page, "login-actions/authenticate",
			"after RP-initiated logout Keycloak must prompt for credentials again")
	})

	t.Run("callback without cookies is rejected", func(t *testing.T) {
		resp, err := http.Get(callbackURL + "?code=forged&state=forged")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Cleanup(func() {
		cleanupUser(t, container, "ada@example.com")
		cleanupUser(t, container, "grace@example.com")
	})
}

// keycloakLogin walks the browser half of the flow with a fresh cookie jar: hit
// /auth/login, follow the redirect to Keycloak, submit the credential form, and let
// the redirect chain land back on /auth/callback. A fresh jar per call matters —
// reusing one would hit Keycloak's SSO session and skip the form entirely.
func keycloakLogin(t *testing.T, username, password string) (string, int) {
	t.Helper()
	return keycloakLoginWith(t, newBrowser(t), username, password)
}

// newBrowser is one simulated browser: a client whose cookie jar persists across
// requests, which is what makes Keycloak's SSO session observable.
func newBrowser(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	return &http.Client{Jar: localhostJar{jar}, Timeout: 30 * time.Second}
}

func keycloakLoginWith(t *testing.T, client *http.Client, username, password string) (string, int) {
	t.Helper()

	resp, err := client.Get(appBaseURL + "/api/v1/auth/login")
	require.NoError(t, err)
	page := readAll(t, resp)
	require.Equal(t, http.StatusOK, resp.StatusCode, "expected Keycloak login page, got: %s", page)

	match := loginActionRe.FindStringSubmatch(page)
	require.Len(t, match, 2, "could not find Keycloak login form action in page: %s", truncate(page))
	action := html.UnescapeString(match[1])

	form := url.Values{"username": {username}, "password": {password}}
	submit, err := client.PostForm(action, form)
	require.NoError(t, err)
	body := readAll(t, submit)

	require.Truef(t, strings.HasPrefix(submit.Request.URL.String(), callbackURL),
		"redirect chain did not end at the callback, ended at %s with body %s",
		submit.Request.URL, truncate(body))

	return body, submit.StatusCode
}

// tokenPair is the auth endpoints' success payload.
type tokenPair struct {
	AccessToken      string `json:"access_token"`
	TokenType        string `json:"token_type"`
	ExpiresIn        int64  `json:"expires_in"`
	RefreshToken     string `json:"refresh_token"`
	RefreshExpiresIn int64  `json:"refresh_expires_in"`
	User             struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"user"`
}

// loginTokens runs a full browser login and returns the resulting token pair.
func loginTokens(t *testing.T) tokenPair {
	t.Helper()
	body, status := keycloakLogin(t, "ada", "ada")
	require.Equal(t, http.StatusOK, status, "callback body: %s", body)

	var callback struct {
		Data tokenPair `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &callback))
	require.NotEmpty(t, callback.Data.RefreshToken)
	return callback.Data
}

// postJSON posts a JSON body, asserts the status, and decodes the "data" envelope.
func postJSON[T any](t *testing.T, url string, payload map[string]string, wantStatus int) T {
	t.Helper()

	encoded, err := json.Marshal(payload)
	require.NoError(t, err)
	resp, err := http.Post(url, "application/json", bytes.NewReader(encoded))
	require.NoError(t, err)
	body := readAll(t, resp)
	require.Equal(t, wantStatus, resp.StatusCode, "POST %s returned: %s", url, truncate(body))

	var envelope struct {
		Data T `json:"data"`
	}
	if wantStatus < 300 {
		require.NoError(t, json.Unmarshal([]byte(body), &envelope), "body: %s", body)
	}
	return envelope.Data
}

// localhostJar sends Keycloak's session cookies back over plain HTTP.
//
// Keycloak marks AUTH_SESSION_ID and KC_RESTART "Secure" because it pairs them with
// SameSite=None, which the cookie spec requires be Secure. Browsers treat
// http://localhost as a secure context and send them anyway; Go's cookiejar has no
// such exception and silently drops them, so Keycloak sees no auth session and just
// re-renders the login form. Stripping Secure for localhost matches browser behavior.
// It is a test-harness concession to plain-HTTP dev mode, not something the app does.
type localhostJar struct{ *cookiejar.Jar }

func (j localhostJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	if isLocalhost(u) {
		for _, c := range cookies {
			c.Secure = false
		}
	}
	j.Jar.SetCookies(u, cookies)
}

func isLocalhost(u *url.URL) bool {
	host := u.Hostname()
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func keycloakConfig(t *testing.T) *config.AppConfig {
	t.Helper()

	// config.Load() validates before returning, so the secret has to exist first.
	if os.Getenv("APP_JWT_SECRET") == "" {
		t.Setenv("APP_JWT_SECRET", "keycloak-e2e-secret-that-is-long-enough")
	}

	cfg, err := config.Load()
	require.NoError(t, err)

	cfg.Middleware.JWT = true
	// The suite makes far more auth calls from one IP than a real client would;
	// the limiter itself is covered by a handler test instead.
	cfg.Middleware.AuthRateLimit = 10000
	cfg.OIDC.Enabled = true
	cfg.OIDC.IssuerURL = envOr("APP_OIDC_ISSUER_URL", "http://localhost:8081/realms/clean-arch")
	cfg.OIDC.ClientID = envOr("APP_OIDC_CLIENT_ID", "clean-arch-api")
	cfg.OIDC.ClientSecret = envOr("APP_OIDC_CLIENT_SECRET", "clean-arch-dev-secret")
	cfg.OIDC.RedirectURL = envOr("APP_OIDC_REDIRECT_URL", callbackURL)
	// Registered in the realm under the client's post.logout.redirect.uris attribute.
	// /healthz is just a route that already exists and answers 200.
	cfg.OIDC.PostLogoutRedirectURL = envOr("APP_OIDC_POST_LOGOUT_REDIRECT_URL", appBaseURL+"/healthz")
	require.NoError(t, cfg.Validate())

	return cfg
}

func cleanupUser(t *testing.T, container *bootstrap.Container, email string) {
	t.Helper()
	if container.DB.SQL == nil {
		return
	}
	db := container.DB.SQL
	_, _ = db.Exec(db.Rebind(
		"DELETE FROM auth_identities WHERE user_id IN (SELECT id FROM users WHERE email = ?)"), email)
	_, _ = db.Exec(db.Rebind("DELETE FROM users WHERE email = ?"), email)
}

func requireReachable(t *testing.T, url string) {
	t.Helper()
	for i := 0; i < 50; i++ {
		if resp, err := http.Get(url); err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("app never became reachable at %s", url)
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func readAll(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(body)
}

func truncate(s string) string {
	if len(s) > 600 {
		return s[:600] + "…"
	}
	return s
}
