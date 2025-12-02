package http

import (
	"fmt"
	"net/http"
	"time"

	"rillnet/internal/core/domain"
	"rillnet/internal/core/ports"

	webrtc "github.com/pion/webrtc/v3"

	"github.com/gin-gonic/gin"
)

type StreamHandler struct {
	streamService ports.StreamService
	webrtcService ports.WebRTCService
}

func NewStreamHandler(
	streamService ports.StreamService,
	webrtcService ports.WebRTCService,
) *StreamHandler {
	return &StreamHandler{
		streamService: streamService,
		webrtcService: webrtcService,
	}
}

func (h *StreamHandler) SetupRoutes(router *gin.Engine) {
	api := router.Group("/api/v1")
	{
		api.POST("/streams", h.CreateStream)
		api.GET("/streams/:id", h.GetStream)
		api.POST("/streams/:id/join", h.JoinStream)
		api.POST("/streams/:id/leave", h.LeaveStream)
		api.GET("/streams/:id/stats", h.GetStreamStats)
		api.GET("/streams", h.ListStreams)

		// WebRTC endpoints
		api.POST("/streams/:id/publisher/offer", h.CreatePublisherOffer)
		api.POST("/streams/:id/publisher/answer", h.HandlePublisherAnswer)
		api.POST("/streams/:id/subscriber/offer", h.CreateSubscriberOffer)
		api.POST("/streams/:id/subscriber/answer", h.HandleSubscriberAnswer)
	}
}

func (h *StreamHandler) CreateStream(c *gin.Context) {
	var req struct {
		Name     string        `json:"name" binding:"required,min=3,max=100"`
		Owner    domain.PeerID `json:"owner" binding:"required"`
		MaxPeers int           `json:"max_peers" binding:"min=1,max=1000"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// User ID is already in context from AuthMiddleware
	stream, err := h.streamService.CreateStream(c.Request.Context(), req.Name, req.Owner, req.MaxPeers)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"stream": stream,
	})
}

func (h *StreamHandler) GetStream(c *gin.Context) {
	streamID := domain.StreamID(c.Param("id"))

	stream, err := h.streamService.GetStream(c.Request.Context(), streamID)
	if err != nil {
		if err == domain.ErrStreamNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Stream not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"stream": stream,
	})
}

func (h *StreamHandler) JoinStream(c *gin.Context) {
	streamID := domain.StreamID(c.Param("id"))

	var req struct {
		PeerID       domain.PeerID `json:"peer_id" binding:"required"`
		IsPublisher  bool          `json:"is_publisher"`
		Capabilities struct {
			MaxBitrate int      `json:"max_bitrate" binding:"min=0,max=10000000"`
			Codecs     []string `json:"codecs"`
		} `json:"capabilities"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(req.Capabilities.Codecs) > 20 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "too many codecs specified"})
		return
	}

	peer := &domain.Peer{
		ID:        req.PeerID,
		StreamID:  streamID,
		SessionID: domain.SessionID(generateSessionID()),
		Address:   c.ClientIP(), // In real application, actual address should be obtained
		Capabilities: domain.PeerCapabilities{
			MaxBitrate:      req.Capabilities.MaxBitrate,
			SupportedCodecs: req.Capabilities.Codecs,
			IsPublisher:     req.IsPublisher,
			CanRelay:        true,
		},
		Metrics: domain.PeerMetrics{
			Bandwidth:   req.Capabilities.MaxBitrate,
			PacketLoss:  0.0,
			Latency:     0,
			CPUUsage:    0.0,
			MemoryUsage: 0,
		},
	}

	if err := h.streamService.JoinStream(c.Request.Context(), streamID, peer); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"session_id": peer.SessionID,
		"status":     "joined",
	})
}

func (h *StreamHandler) LeaveStream(c *gin.Context) {
	streamID := domain.StreamID(c.Param("id"))

	var req struct {
		PeerID domain.PeerID `json:"peer_id" binding:"required"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.streamService.LeaveStream(c.Request.Context(), streamID, req.PeerID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "left",
	})
}

func (h *StreamHandler) GetStreamStats(c *gin.Context) {
	streamID := domain.StreamID(c.Param("id"))

	stats, err := h.streamService.GetStreamStats(c.Request.Context(), streamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"stats": stats,
	})
}

func (h *StreamHandler) ListStreams(c *gin.Context) {
	streams, err := h.streamService.ListStreams(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"streams": streams,
	})
}

// WebRTC endpoints
func (h *StreamHandler) CreatePublisherOffer(c *gin.Context) {
	streamID := domain.StreamID(c.Param("id"))

	var req struct {
		PeerID domain.PeerID `json:"peer_id" binding:"required"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	offer, err := h.webrtcService.CreatePublisherOffer(c.Request.Context(), req.PeerID, streamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"type": "offer",
		"sdp":  offer.SDP,
	})
}

func (h *StreamHandler) HandlePublisherAnswer(c *gin.Context) {
	streamID := domain.StreamID(c.Param("id"))

	fmt.Print("Stream ID:", streamID)

	var req struct {
		PeerID domain.PeerID             `json:"peer_id" binding:"required"`
		Answer webrtc.SessionDescription `json:"answer" binding:"required"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.webrtcService.HandlePublisherAnswer(c.Request.Context(), req.PeerID, req.Answer); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "answer_processed",
	})
}

func (h *StreamHandler) CreateSubscriberOffer(c *gin.Context) {
	streamID := domain.StreamID(c.Param("id"))

	var req struct {
		PeerID      domain.PeerID   `json:"peer_id" binding:"required"`
		SourcePeers []domain.PeerID `json:"source_peers"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	offer, err := h.webrtcService.CreateSubscriberOffer(c.Request.Context(), req.PeerID, streamID, req.SourcePeers)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"type": "offer",
		"sdp":  offer.SDP,
	})
}

func (h *StreamHandler) HandleSubscriberAnswer(c *gin.Context) {
	streamID := domain.StreamID(c.Param("id"))

	fmt.Print("Stream ID:", streamID)

	var req struct {
		PeerID domain.PeerID             `json:"peer_id" binding:"required"`
		Answer webrtc.SessionDescription `json:"answer" binding:"required"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.webrtcService.HandleSubscriberAnswer(c.Request.Context(), req.PeerID, req.Answer); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "answer_processed",
	})
}

func generateSessionID() string {
	return fmt.Sprintf("session_%d", time.Now().UnixNano())
}
