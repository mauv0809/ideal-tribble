package config

// Config holds all configuration for the application.
type Config struct {
	DBName        string
	MigrationsDir string
	Port          string
	TenantID      string
	Turso         TursoConfig
	Web           WebConfig
}

// WebConfig holds configuration for the web UI.
type WebConfig struct {
	SessionSecret     string // 32+ chars recommended
	TOTPEncryptionKey string // Must be exactly 32 chars for AES-256
}
type TursoConfig struct {
	PrimaryURL string
	AuthToken  string
}
