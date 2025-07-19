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
	ProjectID string
	Ngrok     NgrokConfig
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
