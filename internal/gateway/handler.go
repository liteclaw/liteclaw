package gateway

import (
	"context"

	"github.com/liteclaw/liteclaw/internal/channels"
)

// ChannelHandler bridges channel adapters and the gateway
type ChannelHandler struct {
	server *Server
}

func NewChannelHandler(s *Server) *ChannelHandler {
	return &ChannelHandler{server: s}
}

func (h *ChannelHandler) HandleIncoming(ctx context.Context, msg *channels.IncomingMessage) error {
	return h.server.processChannelMessage(ctx, msg)
}
