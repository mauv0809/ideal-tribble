package config

// Config holds all configuration for the application.
type Config struct {
	DBName         string
	Port           string
	SlackBotToken  string
	SlackChannelID string
	TenantID       string
	Turso          TursoConfig
	Inngest        InngestConfig
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
