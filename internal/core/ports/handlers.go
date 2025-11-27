package ports

import (
	"context"
	"rillnet/internal/core/domain"

	"github.com/gin-gonic/gin"
)

type HTTPHandler interface {
	CreateStream(c *gin.Context)
	GetStream(c *gin.Context)
	JoinStream(c *gin.Context)
	LeaveStream(c *gin.Context)
	GetStreamStats(c *gin.Context)
	ListStreams(c *gin.Context)
}

type WebSocketHandler interface {
	HandleConnection(ctx context.Context, wsConn interface{}) error
	HandleMessage(ctx context.Context, peerID domain.PeerID, message []byte) error
	HandleDisconnect(ctx context.Context, peerID domain.PeerID) error
}
