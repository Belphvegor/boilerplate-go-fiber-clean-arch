# Configuration Matrix

The starter reads configuration exclusively from environment variables (loadable via `.env` in local environments). This matrix summarises available options and default values.

| Variable | Description | Default | Notes |
|----------|-------------|---------|-------|
| `APP_NAME` | Service identifier used in logs and metrics. | `clean-arch-starter` | Displayed in log entries and Fiber banner. |
| `APP_ENV` | Runtime environment (`development`, `staging`, `production`). | `development` | Toggle middleware defaults and logging verbosity. |
| `APP_HTTP_PORT` | HTTP listening port. | `8080` | Used by Fiber `Listen`. |
| `APP_HTTP_READ_TIMEOUT` | Max time to read request body. | `10s` | Parsed as Go duration string. |
| `APP_HTTP_WRITE_TIMEOUT` | Max time to write response. | `10s` | Parsed as Go duration string. |
| `APP_DATABASE_DRIVER` | Database backend (`postgres`, `mysql`, `mongo`). | `postgres` | Controls which repository implementation is resolved. |
| `APP_DATABASE_DSN` | SQL DSN (Postgres/MySQL). | `postgres://app:app@localhost:5432/app?sslmode=disable` | Required when driver is `postgres` or `mysql`. |
| `APP_DATABASE_MAX_OPEN_CONNS` | Max open SQL connections. | `10` | Tune per workload. |
| `APP_DATABASE_MAX_IDLE_CONNS` | Max idle SQL connections. | `5` | |
| `APP_DATABASE_CONN_MAX_LIFE` | Max lifetime for SQL connections. | `30m` | |
| `APP_DATABASE_METRICS_ENABLED` | Toggle database metrics instrumentation. | `true` | Placeholder for future metrics exports. |
| `APP_MONGO_URI` | MongoDB connection URI. | `mongodb://app:app@localhost:27017` | Required when driver is `mongo`. |
| `APP_MONGO_DATABASE` | MongoDB database name. | `app` | |
| `APP_JWT_SECRET` | HMAC secret for JWT middleware. | _none_ | Must be set for any environment. |
| `APP_JWT_ISSUER` | JWT issuer claim. | `clean-arch-starter` | |
| `APP_JWT_AUDIENCE` | JWT audience claim for this API. | `clean-arch-starter-api` | Protected routes reject tokens for other audiences. |
| `APP_JWT_ACCESS_TTL` | Access token lifetime. | `15m` | Parsed as Go duration. |
| `APP_JWT_REFRESH_TTL` | Refresh token lifetime. | `168h` | Rotation does not extend it; must be >= access TTL. |
| `APP_OIDC_ENABLED` | Enable OAuth2/OIDC login endpoints. | `false` | When `true`, OIDC client settings are required. |
| `APP_OIDC_ISSUER_URL` | External OIDC issuer URL. | _none_ | Used for discovery and ID token issuer validation. |
| `APP_OIDC_CLIENT_ID` | OIDC client ID. | _none_ | Must match provider registration. |
| `APP_OIDC_CLIENT_SECRET` | OIDC client secret. | _none_ | Keep secret; never commit real values. |
| `APP_OIDC_REDIRECT_URL` | Callback URL registered at the provider. | _none_ | Example: `http://localhost:8080/api/v1/auth/callback`. |
| `APP_OIDC_POST_LOGOUT_REDIRECT_URL` | Where the provider sends the browser after logout. | _empty_ | Must be registered on the client; empty means the provider's own logged-out page. |
| `APP_OIDC_SCOPES` | OIDC scopes requested at login. | `openid profile email` | Must include `openid`; `email` is needed for auto-provisioning. |
| `APP_OIDC_LOGIN_STATE_TTL` | Lifetime for temporary state/nonce/PKCE cookies. | `5m` | Parsed as Go duration. |
| `APP_LOGGING_LEVEL` | Zerolog level (`trace`, `debug`, `info`, `warn`, `error`). | `info` | |
| `APP_LOGGING_PRETTY` | Enable human-readable console logs. | `true` | Set to `false` for JSON logs. |
| `APP_REQUEST_LOGGER_ENABLED` | Enable request logging middleware. | `true` | |
| `APP_RECOVERY_ENABLED` | Enable panic recovery middleware. | `true` | |
| `APP_CORS_ENABLED` | Enable permissive CORS headers. | `true` | Configure or replace for stricter policies. |
| `APP_HTTP_TRUSTED_PROXIES` | Proxy addresses allowed to set `X-Forwarded-For`. | _empty_ | Required behind a proxy, or the per-IP auth limit degrades to one global bucket. |
| `APP_AUTH_RATE_LIMIT` | Requests per minute per IP across the `/auth` endpoints. | `60` | Shared by clients behind one NAT. |
| `APP_JWT_MIDDLEWARE_ENABLED` | Enable route-level JWT protection. | `true` | Set to `false` for public endpoints or local development. |

## OAuth2 / OpenID Connect

The API acts as an OIDC client. `GET /api/v1/auth/login` starts Authorization Code + PKCE login, and `GET /api/v1/auth/callback` validates the ID token, links the stable provider/subject identity, and returns a local access/refresh token pair.

For production:

- Register the exact `APP_OIDC_REDIRECT_URL` with the provider.
- Use HTTPS for the redirect URL and a strong `APP_JWT_SECRET` of at least 32 characters.
- Keep `APP_JWT_MIDDLEWARE_ENABLED=true` so protected routes require the local Bearer token.
- Keep the default scopes unless the provider requires additional values; auto-provisioning needs a verified email claim.

### Sessions, refresh, and logout

Login returns a pair: a short-lived access token and an opaque refresh token.

| Endpoint | Purpose |
|---|---|
| `GET /api/v1/auth/login` | Start the browser flow |
| `GET /api/v1/auth/callback` | Finish it; returns the first token pair |
| `POST /api/v1/auth/refresh` | `{"refresh_token": "..."}` -> a new pair |
| `POST /api/v1/auth/logout` | `{"refresh_token": "..."}` -> `{"logout_url": "..."}` |

How the refresh token behaves, and why:

- **Opaque and hashed at rest.** It is 256 bits of randomness; only its SHA-256 is
  stored. A database leak yields hashes, not usable credentials. Keycloak's own refresh
  token is deliberately *not* stored — that would put a real provider credential in the
  database.
- **Single use, rotated on every refresh.** The response always carries a new refresh
  token; the one you sent is spent.
- **Replay revokes the whole family.** Presenting an already-spent token is treated as
  evidence the token leaked, so every token descended from that login is revoked and the
  user must sign in again (RFC 9700). A client that fires two refreshes concurrently will
  trigger this on itself — single-flight the call.
- **Rotation does not extend the deadline.** The successor inherits the original expiry,
  so `APP_JWT_REFRESH_TTL` is a hard ceiling on a session, not a sliding window.
- **Expired history is pruned on successful login.** This keeps the starter free of a
  scheduler while preventing unbounded growth. Move it to a periodic database job only
  if cleanup time becomes measurable at production scale.
- **The API never rejects a replay differently.** Replay and "unknown token" both return
  401 with the same body; the distinction only appears in the server log.
- **A refresh that overlaps a logout loses.** Consuming a token requires the row to be
  both unused and unrevoked, and after the successor is written the predecessor is
  re-read; if a logout revoked the family mid-rotation, the successor is revoked too and
  the caller gets a 401. Without that second check a logout could be silently undone by
  an in-flight refresh.

If you run the API behind a proxy or load balancer, set `APP_HTTP_TRUSTED_PROXIES`.
The auth rate limit is keyed on the client IP, and without it every request appears to
come from the proxy, so all users share a single budget.

Logout does two things, and both matter:

1. Revokes the session family locally, so the refresh token dies immediately.
2. Returns `logout_url` — the provider's RP-initiated logout endpoint with
   `id_token_hint`. **The client must send the browser there.** Skipping it leaves
   Keycloak's SSO cookie alive, and the next `/auth/login` signs the same user straight
   back in with no password prompt. That is a false logout, and the E2E test asserts
   against exactly this.

The access token itself is not revocable — it stays valid until it expires. Revocation
latency is therefore `APP_JWT_ACCESS_TTL`. Shorten it if that window is too wide; a
proper denylist would key on the `jti` claim, which is now present on every token.

### Keycloak (local development)

`docker compose --profile keycloak up -d keycloak` starts Keycloak 26 in dev mode on
port 8081 and imports `deploy/keycloak/realm-clean-arch.json`, which defines:

| Item | Value |
|---|---|
| Realm | `clean-arch` |
| Issuer (`iss`) | `http://localhost:8081/realms/clean-arch` |
| Client | `clean-arch-api`, confidential, secret `clean-arch-dev-secret` |
| PKCE | Required (`pkce.code.challenge.method: S256` client attribute) |
| Redirect URI | `http://localhost:8080/api/v1/auth/callback` (exact match, no wildcard) |
| Admin console | <http://localhost:8081> — `admin` / `admin` |
| Test user | `ada` / `ada` — email verified, logs in successfully |
| Test user | `grace` / `grace` — email **not** verified, rejected with 401 |

Copy the OIDC block from `.env.example` into `.env`, set `APP_OIDC_ENABLED=true`, run the
migrations, then start the API and open <http://localhost:8080/api/v1/auth/login>.

Verify the whole flow, including the ID token signature, PKCE, and the JWT-guarded route:

```bash
KEYCLOAK_E2E=true go test ./test/integration -run TestKeycloak -v
```

Things that break a Keycloak setup, in the order they usually bite:

- **Issuer mismatch.** `iss` comes from Keycloak's `KC_HOSTNAME`, not from the URL you
  dialed. If the API runs inside Docker it reaches Keycloak at `http://keycloak:8080`
  while the browser uses `http://localhost:8081`, and the two disagree — ID token
  verification then fails. Keep `KC_HOSTNAME` and `APP_OIDC_ISSUER_URL` identical, and
  make that host resolvable from both the browser and the API.
- **Unverified email.** `email_verified` is a plain read of the user's stored
  `emailVerified` attribute. Every admin-created user is `false` until someone sets it,
  and auto-provisioning refuses those logins.
- **No `name` claim.** Keycloak omits `name` entirely when a user has no first or last
  name; the display name then falls back to `preferred_username`, then the email local part.
- **Realm edits do not re-import.** Startup import skips a realm that already exists, and
  dev mode keeps its H2 store in the container layer. After editing the realm file run
  `docker compose --profile keycloak up -d --force-recreate keycloak`.
- **`sub` is realm-scoped.** Identities are keyed on (issuer, subject), so re-creating the
  realm issues new subjects and provisions new local users.

For production, replace dev mode with `start`, a real database, TLS, and a rotated client
secret; the committed realm file is a development fixture, not a deployment artifact.

## Switching Databases

1. Update `.env`:
   - For **PostgreSQL**:
     ```env
     APP_DATABASE_DRIVER=postgres
     APP_DATABASE_DSN=postgres://app:app@localhost:5432/app?sslmode=disable
     ```
   - For **MySQL**:
     ```env
     APP_DATABASE_DRIVER=mysql
     APP_DATABASE_DSN=app:app@tcp(localhost:3306)/app?parseTime=true
     ```
   - For **MongoDB**:
     ```env
     APP_DATABASE_DRIVER=mongo
     APP_MONGO_URI=mongodb://app:app@localhost:27017
     APP_MONGO_DATABASE=app
     ```

2. Start the corresponding Docker Compose service:
   ```bash
   docker compose up -d database        # PostgreSQL (default)
   docker compose --profile mysql up -d mysql
   docker compose --profile mongo up -d mongo
   ```

3. Run migrations or seed scripts matching the selected backend.

4. Restart the API process to apply changes. No code modifications are required—the database factory resolves the correct repository based on `APP_DATABASE_DRIVER`.

## Fail-Fast Validation

The configuration loader validates required settings on startup:

- Missing `APP_JWT_SECRET` returns an actionable error before Fiber starts.
- Production rejects `APP_JWT_SECRET=change-me` and secrets shorter than 32 characters.
- Enabling OIDC without issuer, client ID, client secret, redirect URL, or a positive state TTL stops boot.
- SQL drivers require `APP_DATABASE_DSN`; MongoDB requires `APP_MONGO_URI` and `APP_MONGO_DATABASE`.
- Duration values are parsed using `time.ParseDuration`; invalid formats stop boot.

Review the startup logs when adjusting configuration—errors surface with explicit messages and recommended fixes.
