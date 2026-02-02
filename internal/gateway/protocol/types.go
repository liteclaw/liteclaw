// Package protocol defines the Clawdbot Gateway WebSocket protocol types.
// This mirrors the TypeScript protocol defined in src/gateway/protocol/.
package protocol

// ProtocolVersion is the current protocol version.
const ProtocolVersion = 3

// Frame types.
const (
	FrameTypeRequest  = "req"
	FrameTypeResponse = "res"
	FrameTypeEvent    = "event"
)

// Client modes.
const (
	ClientModeBackend = "backend"
	ClientModeTUI     = "cli" // Changed from "tui" to "cli" to match Clawdbot validation
	ClientModeNode    = "node"
	ClientModeProbe   = "probe"
)

// Client names.
const (
	ClientNameGateway = "gateway-client"
	ClientNameTUI     = "cli" // Changed from "liteclaw-tui" to "cli" to match Clawdbot validation
	ClientNameAgent   = "liteclaw-agent"
	ClientNameNode    = "liteclaw-node"
)

// Event names.
const (
	EventTick             = "tick"
	EventShutdown         = "shutdown"
	EventConnectChallenge = "connect.challenge"
	EventChat             = "chat"
	EventAgent            = "agent"
	EventNode             = "node"
	EventState            = "state"
	EventPresence         = "presence"
)

// Method names.
const (
	MethodConnect         = "connect"
	MethodPoll            = "poll"
	MethodAgent           = "agent"
	MethodAgentWait       = "agent.wait"
	MethodAgentAbort      = "agent.abort"
	MethodAgentsList      = "agents.list"
	MethodChatSend        = "chat.send"
	MethodChatHistory     = "chat.history"
	MethodChatAbort       = "chat.abort"
	MethodSessionsList    = "sessions.list"
	MethodSessionsPreview = "sessions.preview"
	MethodSessionsResolve = "sessions.resolve"
	MethodSessionsPatch   = "sessions.patch"
	MethodSessionsReset   = "sessions.reset"
	MethodSessionsDelete  = "sessions.delete"
	MethodNodesList       = "nodes.list"
	MethodNodeInvoke      = "node.invoke"
	MethodNodeDescribe    = "node.describe"
	MethodCronList        = "cron.list"
	MethodCronStatus      = "cron.status"
	MethodCronAdd         = "cron.add"
	MethodCronUpdate      = "cron.update"
	MethodCronRemove      = "cron.remove"
	MethodCronRun         = "cron.run"
	MethodConfigGet       = "config.get"
	MethodConfigSet       = "config.set"
	MethodConfigPatch     = "config.patch"
	MethodConfigApply     = "config.apply"
	MethodModelsList      = "models.list"
	MethodChannelsStatus  = "channels.status"
	MethodChannelsLogout  = "channels.logout"
	MethodSkillsStatus    = "skills.status"
	MethodSkillsInstall   = "skills.install"
	MethodLogsTail        = "logs.tail"
)

// Error codes.
const (
	ErrorCodeUnknown          = "UNKNOWN"
	ErrorCodeInvalidRequest   = "INVALID_REQUEST"
	ErrorCodeMethodNotFound   = "METHOD_NOT_FOUND"
	ErrorCodeNotAuthorized    = "NOT_AUTHORIZED"
	ErrorCodeNotFound         = "NOT_FOUND"
	ErrorCodeRateLimited      = "RATE_LIMITED"
	ErrorCodeInternal         = "INTERNAL"
	ErrorCodeProtocolMismatch = "PROTOCOL_MISMATCH"
)

// RequestFrame is a client request to the server.
type RequestFrame struct {
	Type   string      `json:"type"` // "req"
	ID     string      `json:"id"`
	Method string      `json:"method"`
	Params interface{} `json:"params,omitempty"`
}

// ResponseFrame is a server response to a request.
type ResponseFrame struct {
	Type    string      `json:"type"` // "res"
	ID      string      `json:"id"`
	OK      bool        `json:"ok"`
	Payload interface{} `json:"payload,omitempty"`
	Error   *ErrorShape `json:"error,omitempty"`
}

// EventFrame is a server-pushed event.
type EventFrame struct {
	Type         string        `json:"type"` // "event"
	Event        string        `json:"event"`
	Payload      interface{}   `json:"payload,omitempty"`
	Seq          int           `json:"seq,omitempty"`
	StateVersion *StateVersion `json:"stateVersion,omitempty"`
}

// ErrorShape represents an error from the server.
type ErrorShape struct {
	Code         string      `json:"code"`
	Message      string      `json:"message"`
	Details      interface{} `json:"details,omitempty"`
	Retryable    bool        `json:"retryable,omitempty"`
	RetryAfterMs int         `json:"retryAfterMs,omitempty"`
}

// StateVersion tracks state mutations.
type StateVersion struct {
	Sessions int64 `json:"sessions,omitempty"`
	Nodes    int64 `json:"nodes,omitempty"`
	Cron     int64 `json:"cron,omitempty"`
	Config   int64 `json:"config,omitempty"`
}

// ConnectParams represents the connect request parameters.
type ConnectParams struct {
	MinProtocol int             `json:"minProtocol"`
	MaxProtocol int             `json:"maxProtocol"`
	Client      ClientInfo      `json:"client"`
	Caps        []string        `json:"caps,omitempty"`
	Commands    []string        `json:"commands,omitempty"`
	Permissions map[string]bool `json:"permissions,omitempty"`
	PathEnv     string          `json:"pathEnv,omitempty"`
	Role        string          `json:"role,omitempty"`
	Scopes      []string        `json:"scopes,omitempty"`
	Auth        *AuthInfo       `json:"auth,omitempty"`
	Device      *DeviceInfo     `json:"device,omitempty"`
}

// ClientInfo contains client identification.
type ClientInfo struct {
	ID              string `json:"id"`
	DisplayName     string `json:"displayName,omitempty"`
	Version         string `json:"version"`
	Platform        string `json:"platform"`
	DeviceFamily    string `json:"deviceFamily,omitempty"`
	ModelIdentifier string `json:"modelIdentifier,omitempty"`
	Mode            string `json:"mode"`
	InstanceID      string `json:"instanceId,omitempty"`
}

// AuthInfo contains authentication credentials.
type AuthInfo struct {
	Token    string `json:"token,omitempty"`
	Password string `json:"password,omitempty"`
}

// DeviceInfo contains device identity for device auth.
type DeviceInfo struct {
	ID        string `json:"id"`
	PublicKey string `json:"publicKey"`
	Signature string `json:"signature"`
	SignedAt  int64  `json:"signedAt"`
	Nonce     string `json:"nonce,omitempty"`
}

// HelloOk is the successful connect response.
type HelloOk struct {
	Type          string      `json:"type"` // "hello-ok"
	Protocol      int         `json:"protocol"`
	Server        ServerInfo  `json:"server"`
	Features      Features    `json:"features"`
	Snapshot      interface{} `json:"snapshot,omitempty"`
	Auth          *AuthResult `json:"auth,omitempty"`
	Policy        PolicyInfo  `json:"policy"`
	CanvasHostURL string      `json:"canvasHostUrl,omitempty"`
}

// ServerInfo contains server information.
type ServerInfo struct {
	Version string `json:"version"`
	Commit  string `json:"commit,omitempty"`
	Host    string `json:"host,omitempty"`
	ConnID  string `json:"connId"`
}

// Features lists available methods and events.
type Features struct {
	Methods []string `json:"methods"`
	Events  []string `json:"events"`
}

// PolicyInfo contains connection policy.
type PolicyInfo struct {
	MaxPayload       int `json:"maxPayload"`
	MaxBufferedBytes int `json:"maxBufferedBytes"`
	TickIntervalMs   int `json:"tickIntervalMs"`
}

// AuthResult contains authentication result.
type AuthResult struct {
	DeviceToken string   `json:"deviceToken,omitempty"`
	Role        string   `json:"role,omitempty"`
	Scopes      []string `json:"scopes,omitempty"`
	IssuedAtMs  int64    `json:"issuedAtMs,omitempty"`
}

// TickEvent is the tick heartbeat event payload.
type TickEvent struct {
	Ts int64 `json:"ts"`
}

// ShutdownEvent is the shutdown event payload.
type ShutdownEvent struct {
	Reason            string `json:"reason"`
	RestartExpectedMs int    `json:"restartExpectedMs,omitempty"`
}

// ChatEvent is a chat message event.
type ChatEvent struct {
	SessionKey string      `json:"sessionKey"`
	RunID      string      `json:"runId,omitempty"`
	Message    interface{} `json:"message,omitempty"`
	Delta      interface{} `json:"delta,omitempty"`
	Status     string      `json:"status,omitempty"`
}

// AgentEvent is an agent event.
type AgentEvent struct {
	SessionKey string      `json:"sessionKey"`
	RunID      string      `json:"runId"`
	Event      string      `json:"event"`
	Payload    interface{} `json:"payload,omitempty"`
}

// SessionEntry represents a session in listings.
type SessionEntry struct {
	Key          string `json:"key"`
	Label        string `json:"label,omitempty"`
	AgentID      string `json:"agentId,omitempty"`
	Model        string `json:"model,omitempty"`
	Channel      string `json:"channel,omitempty"`
	CreatedAt    int64  `json:"createdAt,omitempty"`
	UpdatedAt    int64  `json:"updatedAt,omitempty"`
	MessageCount int    `json:"messageCount,omitempty"`
}

// NodeEntry represents a node in listings.
type NodeEntry struct {
	ID         string   `json:"id"`
	Type       string   `json:"type"`
	Name       string   `json:"name,omitempty"`
	Status     string   `json:"status"`
	Caps       []string `json:"caps,omitempty"`
	LastSeenAt int64    `json:"lastSeenAt,omitempty"`
}

// CronJob represents a scheduled job.
type CronJob struct {
	ID          string      `json:"id"`
	Schedule    string      `json:"schedule,omitempty"`
	IntervalMs  int64       `json:"intervalMs,omitempty"`
	Enabled     bool        `json:"enabled"`
	Description string      `json:"description,omitempty"`
	Payload     interface{} `json:"payload,omitempty"`
	NextRunAt   int64       `json:"nextRunAt,omitempty"`
	LastRunAt   int64       `json:"lastRunAt,omitempty"`
}

// PresenceEntry represents a connected client.
type PresenceEntry struct {
	ConnID      string `json:"connId"`
	ClientID    string `json:"clientId"`
	DisplayName string `json:"displayName,omitempty"`
	Mode        string `json:"mode"`
	Role        string `json:"role,omitempty"`
	ConnectedAt int64  `json:"connectedAt,omitempty"`
}
