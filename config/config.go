package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

const (
	driverPostgres = "postgres"
	driverMySQL    = "mysql"
	driverMongo    = "mongo"
)

var supportedDrivers = map[string]struct{}{
	driverPostgres: {},
	driverMySQL:    {},
	driverMongo:    {},
}

// AppConfig represents the normalized application configuration.
type AppConfig struct {
	Name        string
	Environment string
	HTTP        HTTPConfig
	Database    DatabaseConfig
	Mongo       MongoConfig
	JWT         JWTConfig
	OIDC        OIDCConfig
	Logging     LoggingConfig
	Middleware  MiddlewareConfig
}

// HTTPConfig captures HTTP server configuration options.
type HTTPConfig struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// DatabaseConfig captures relational database connectivity details.
type DatabaseConfig struct {
	Driver         string
	DSN            string
	MaxOpenConns   int
	MaxIdleConns   int
	ConnMaxLife    time.Duration
	MetricsEnabled bool
}

// MongoConfig stores MongoDB specific configuration.
type MongoConfig struct {
	URI      string
	Database string
}

// JWTConfig defines authentication token defaults.
type JWTConfig struct {
	Secret    string
	Issuer    string
	Audience  string
	AccessTTL time.Duration
}

// OIDCConfig defines external OpenID Connect client configuration.
type OIDCConfig struct {
	Enabled       bool
	IssuerURL     string
	ClientID      string
	ClientSecret  string
	RedirectURL   string
	Scopes        []string
	LoginStateTTL time.Duration
}

// LoggingConfig declares logging behaviour.
type LoggingConfig struct {
	Level  string
	Pretty bool
}

// MiddlewareConfig toggles optional middleware components.
type MiddlewareConfig struct {
	RequestLogger bool
	Recovery      bool
	CORS          bool
	JWT           bool
}

// Load reads environment variables (optionally loading .env files) and produces
// a validated AppConfig instance.
func Load(envFiles ...string) (*AppConfig, error) {
	if len(envFiles) == 0 {
		_ = godotenv.Load()
	} else {
		for _, path := range envFiles {
			if _, err := os.Stat(path); err == nil {
				if err := godotenv.Overload(path); err != nil {
					return nil, fmt.Errorf("load env file %s: %w", path, err)
				}
			}
		}
	}

	v := viper.New()
	v.SetEnvPrefix("APP")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	setDefaults(v)

	httpCfg, err := parseHTTPConfig(v)
	if err != nil {
		return nil, err
	}

	dbCfg, err := parseDatabaseConfig(v)
	if err != nil {
		return nil, err
	}

	mongoCfg := MongoConfig{
		URI:      strings.TrimSpace(v.GetString("MONGO.URI")),
		Database: strings.TrimSpace(v.GetString("MONGO.DATABASE")),
	}

	jwtCfg, err := parseJWTConfig(v)
	if err != nil {
		return nil, err
	}

	oidcCfg, err := parseOIDCConfig(v)
	if err != nil {
		return nil, err
	}

	loggingCfg := LoggingConfig{
		Level:  strings.ToLower(strings.TrimSpace(v.GetString("LOGGING.LEVEL"))),
		Pretty: v.GetBool("LOGGING.PRETTY"),
	}

	mwCfg := MiddlewareConfig{
		RequestLogger: v.GetBool("REQUEST_LOGGER_ENABLED"),
		Recovery:      v.GetBool("RECOVERY_ENABLED"),
		CORS:          v.GetBool("CORS_ENABLED"),
		JWT:           v.GetBool("JWT_MIDDLEWARE_ENABLED"),
	}

	cfg := &AppConfig{
		Name:        strings.TrimSpace(v.GetString("NAME")),
		Environment: strings.ToLower(strings.TrimSpace(v.GetString("ENV"))),
		HTTP:        httpCfg,
		Database:    dbCfg,
		Mongo:       mongoCfg,
		JWT:         jwtCfg,
		OIDC:        oidcCfg,
		Logging:     loggingCfg,
		Middleware:  mwCfg,
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate ensures the configuration is internally consistent.
func (c *AppConfig) Validate() error {
	if c.Name == "" {
		return errors.New("config: APP_NAME is required")
	}

	if _, ok := supportedDrivers[c.Database.Driver]; !ok {
		return fmt.Errorf("config: unsupported APP_DATABASE_DRIVER %q", c.Database.Driver)
	}

	if c.Database.Driver != driverMongo && c.Database.DSN == "" {
		return errors.New("config: APP_DATABASE_DSN is required for SQL backends")
	}

	if c.Database.Driver == driverMongo && c.Mongo.URI == "" {
		return errors.New("config: APP_MONGO_URI required when driver is mongo")
	}

	if c.Database.Driver == driverMongo && c.Mongo.Database == "" {
		return errors.New("config: APP_MONGO_DATABASE required when driver is mongo")
	}

	if c.JWT.Secret == "" {
		return errors.New("config: APP_JWT_SECRET is required")
	}

	if c.Environment == "production" && (c.JWT.Secret == "change-me" || len(c.JWT.Secret) < 32) {
		return errors.New("config: APP_JWT_SECRET must be a strong secret in production")
	}

	if c.JWT.Audience == "" {
		return errors.New("config: APP_JWT_AUDIENCE is required")
	}

	if c.JWT.AccessTTL <= 0 {
		return errors.New("config: APP_JWT_ACCESS_TTL must be positive duration")
	}

	if c.OIDC.Enabled {
		if c.OIDC.IssuerURL == "" {
			return errors.New("config: APP_OIDC_ISSUER_URL is required when OIDC is enabled")
		}
		if c.OIDC.ClientID == "" {
			return errors.New("config: APP_OIDC_CLIENT_ID is required when OIDC is enabled")
		}
		if c.OIDC.ClientSecret == "" {
			return errors.New("config: APP_OIDC_CLIENT_SECRET is required when OIDC is enabled")
		}
		if c.OIDC.RedirectURL == "" {
			return errors.New("config: APP_OIDC_REDIRECT_URL is required when OIDC is enabled")
		}
		if c.OIDC.LoginStateTTL <= 0 {
			return errors.New("config: APP_OIDC_LOGIN_STATE_TTL must be positive duration")
		}
		if !contains(c.OIDC.Scopes, "openid") {
			return errors.New("config: APP_OIDC_SCOPES must include openid")
		}
	}

	if c.HTTP.Port == "" {
		return errors.New("config: APP_HTTP_PORT is required")
	}

	if c.HTTP.ReadTimeout <= 0 || c.HTTP.WriteTimeout <= 0 {
		return errors.New("config: HTTP timeouts must be positive durations")
	}

	return nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("NAME", "clean-arch-starter")
	v.SetDefault("ENV", "development")
	v.SetDefault("HTTP.PORT", "8080")
	v.SetDefault("HTTP.READ_TIMEOUT", "10s")
	v.SetDefault("HTTP.WRITE_TIMEOUT", "10s")

	v.SetDefault("DATABASE.DRIVER", driverPostgres)
	v.SetDefault("DATABASE.DSN", "postgres://app:app@localhost:5432/app?sslmode=disable")
	v.SetDefault("DATABASE.MAX_OPEN_CONNS", 10)
	v.SetDefault("DATABASE.MAX_IDLE_CONNS", 5)
	v.SetDefault("DATABASE.CONN_MAX_LIFE", "30m")
	v.SetDefault("DATABASE.METRICS_ENABLED", true)

	v.SetDefault("MONGO.URI", "mongodb://app:app@localhost:27017")
	v.SetDefault("MONGO.DATABASE", "app")

	v.SetDefault("JWT.SECRET", "")
	v.SetDefault("JWT.ISSUER", "clean-arch-starter")
	v.SetDefault("JWT.AUDIENCE", "clean-arch-starter-api")
	v.SetDefault("JWT.ACCESS_TTL", "15m")

	v.SetDefault("OIDC.ENABLED", false)
	v.SetDefault("OIDC.ISSUER_URL", "")
	v.SetDefault("OIDC.CLIENT_ID", "")
	v.SetDefault("OIDC.CLIENT_SECRET", "")
	v.SetDefault("OIDC.REDIRECT_URL", "")
	v.SetDefault("OIDC.SCOPES", "openid profile email")
	v.SetDefault("OIDC.LOGIN_STATE_TTL", "5m")

	v.SetDefault("LOGGING.LEVEL", "info")
	v.SetDefault("LOGGING.PRETTY", true)

	v.SetDefault("REQUEST_LOGGER_ENABLED", true)
	v.SetDefault("RECOVERY_ENABLED", true)
	v.SetDefault("CORS_ENABLED", true)
	v.SetDefault("JWT_MIDDLEWARE_ENABLED", true)
}

func parseHTTPConfig(v *viper.Viper) (HTTPConfig, error) {
	readTimeout, err := time.ParseDuration(v.GetString("HTTP.READ_TIMEOUT"))
	if err != nil {
		return HTTPConfig{}, fmt.Errorf("config: invalid APP_HTTP_READ_TIMEOUT: %w", err)
	}

	writeTimeout, err := time.ParseDuration(v.GetString("HTTP.WRITE_TIMEOUT"))
	if err != nil {
		return HTTPConfig{}, fmt.Errorf("config: invalid APP_HTTP_WRITE_TIMEOUT: %w", err)
	}

	return HTTPConfig{
		Port:         strings.TrimSpace(v.GetString("HTTP.PORT")),
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	}, nil
}

func parseDatabaseConfig(v *viper.Viper) (DatabaseConfig, error) {
	connMaxLife, err := time.ParseDuration(v.GetString("DATABASE.CONN_MAX_LIFE"))
	if err != nil {
		return DatabaseConfig{}, fmt.Errorf("config: invalid APP_DATABASE_CONN_MAX_LIFE: %w", err)
	}

	maxOpen := v.GetInt("DATABASE.MAX_OPEN_CONNS")
	if maxOpen == 0 {
		maxOpen = 10
	}
	maxIdle := v.GetInt("DATABASE.MAX_IDLE_CONNS")
	if maxIdle == 0 {
		maxIdle = 5
	}

	metricsEnabled := parseBool(v.GetString("DATABASE.METRICS_ENABLED"))

	return DatabaseConfig{
		Driver:         strings.ToLower(strings.TrimSpace(v.GetString("DATABASE.DRIVER"))),
		DSN:            strings.TrimSpace(v.GetString("DATABASE.DSN")),
		MaxOpenConns:   maxOpen,
		MaxIdleConns:   maxIdle,
		ConnMaxLife:    connMaxLife,
		MetricsEnabled: metricsEnabled,
	}, nil
}

func parseJWTConfig(v *viper.Viper) (JWTConfig, error) {
	ttl, err := time.ParseDuration(v.GetString("JWT.ACCESS_TTL"))
	if err != nil {
		return JWTConfig{}, fmt.Errorf("config: invalid APP_JWT_ACCESS_TTL: %w", err)
	}

	return JWTConfig{
		Secret:    strings.TrimSpace(v.GetString("JWT.SECRET")),
		Issuer:    strings.TrimSpace(v.GetString("JWT.ISSUER")),
		Audience:  strings.TrimSpace(v.GetString("JWT.AUDIENCE")),
		AccessTTL: ttl,
	}, nil
}

func parseOIDCConfig(v *viper.Viper) (OIDCConfig, error) {
	ttl, err := time.ParseDuration(v.GetString("OIDC.LOGIN_STATE_TTL"))
	if err != nil {
		return OIDCConfig{}, fmt.Errorf("config: invalid APP_OIDC_LOGIN_STATE_TTL: %w", err)
	}

	return OIDCConfig{
		Enabled:       v.GetBool("OIDC.ENABLED"),
		IssuerURL:     strings.TrimRight(strings.TrimSpace(v.GetString("OIDC.ISSUER_URL")), "/"),
		ClientID:      strings.TrimSpace(v.GetString("OIDC.CLIENT_ID")),
		ClientSecret:  strings.TrimSpace(v.GetString("OIDC.CLIENT_SECRET")),
		RedirectURL:   strings.TrimSpace(v.GetString("OIDC.REDIRECT_URL")),
		Scopes:        parseScopes(v.GetString("OIDC.SCOPES")),
		LoginStateTTL: ttl,
	}, nil
}

func parseScopes(value string) []string {
	seen := make(map[string]struct{})
	scopes := make([]string, 0)
	for _, scope := range strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\n' || r == '\t'
	}) {
		scope = strings.TrimSpace(scope)
		if scope == "" {
			continue
		}
		if _, ok := seen[scope]; ok {
			continue
		}
		seen[scope] = struct{}{}
		scopes = append(scopes, scope)
	}
	if !contains(scopes, "openid") {
		scopes = append([]string{"openid"}, scopes...)
	}
	return scopes
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func parseBool(val string) bool {
	if val == "" {
		return false
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		return false
	}
	return b
}
