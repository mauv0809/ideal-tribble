package config

// Config holds all configuration for the application.
type Config struct {
	DBName        string
	MigrationsDir string
	Port          string
	Slack         SlackConfig
	TenantID      string
	Turso         TursoConfig
	//Inngest        InngestConfig
	Ngrok NgrokConfig
	Web   WebConfig
}

// WebConfig holds configuration for the web UI.
type WebConfig struct {
	SessionSecret     string // 32+ chars recommended
	TOTPEncryptionKey string // Must be exactly 32 chars for AES-256
}
type SlackConfig struct {
	Token         string
	ChannelID     string
	SigningSecret string
}
type TursoConfig struct {
	PrimaryURL string
	AuthToken  string
}
type InngestConfig struct {
	SingingKey string
	EventKey   string
	AppID      string
}
type NgrokConfig struct {
	AuthToken string
}
