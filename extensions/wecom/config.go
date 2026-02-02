package wecom

// Config holds WeCom (Enterprise WeChat) specific configuration.
type Config struct {
	CorpID         string `json:"corpId" yaml:"corpId"`
	AgentID        int64  `json:"agentId" yaml:"agentId"`
	AgentSecret    string `json:"agentSecret" yaml:"agentSecret"`
	Token          string `json:"token" yaml:"token"`
	EncodingAESKey string `json:"encodingAesKey" yaml:"encodingAesKey"`
	Port           int    `json:"port" yaml:"port"`
	BotID          string `json:"botId" yaml:"botId"` // Added BotID
	ShowThinking   bool   `json:"showThinking" yaml:"showThinking"`
}
