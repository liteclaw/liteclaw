package wecom

// Config holds WeCom (Enterprise WeChat) specific configuration.
type Config struct {
	Token          string `json:"token" yaml:"token"`
	EncodingAESKey string `json:"encodingAesKey" yaml:"encodingAesKey"`
	Port           int    `json:"port" yaml:"port"`
	BotID          string `json:"botId" yaml:"botId"`
}
