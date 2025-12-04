package services

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"rillnet/internal/core/domain"
	"rillnet/internal/core/ports"
	"rillnet/pkg/config"
	"go.uber.org/zap"
)

type meshService struct {
	peerRepo ports.PeerRepository
	meshRepo ports.MeshRepository
	config   config.MeshConfig
	logger   *zap.SugaredLogger
	
	// Rebalancing state
	rebalanceTicker *time.Ticker
	rebalanceStop   chan struct{}
	rebalanceMu     sync.RWMutex
}

func NewMeshService(peerRepo ports.PeerRepository, meshRepo ports.MeshRepository, cfg config.MeshConfig, logger *zap.SugaredLogger) ports.MeshService {
	ms := &meshService{
		peerRepo: peerRepo,
		meshRepo: meshRepo,
		config:   cfg,
		logger:   logger,
		rebalanceStop: make(chan struct{}),
	}

	// Start periodic rebalancing
	if cfg.RebalanceInterval > 0 {
		ms.rebalanceTicker = time.NewTicker(cfg.RebalanceInterval)
		go ms.rebalanceLoop()
	}

	return ms
}

// rebalanceLoop periodically rebalances the mesh network
func (m *meshService) rebalanceLoop() {
	for {
		select {
		case <-m.rebalanceTicker.C:
			m.rebalanceAllStreams()
		case <-m.rebalanceStop:
			return
		}
	}
}

// rebalanceAllStreams rebalances all active streams
func (m *meshService) rebalanceAllStreams() {
	// This would need access to stream repository to get all streams
	// For now, we'll implement per-stream rebalancing when called
	m.logger.Debug("mesh rebalancing triggered")
}

func (m *meshService) AddPeer(ctx context.Context, peer *domain.Peer) error {
	if err := m.peerRepo.Add(ctx, peer); err != nil {
		return err
	}

	// Trigger mesh rebuild for the stream
	go func() {
		if err := m.BuildOptimalMesh(ctx, peer.StreamID); err != nil {
			m.logger.Warnw("failed to rebuild mesh after peer addition",
				"peer_id", peer.ID,
				"stream_id", peer.StreamID,
				"error", err,
			)
		}
	}()

	return nil
}

func (m *meshService) RemovePeer(ctx context.Context, peerID domain.PeerID) error {
	// Get peer info before removal
	peer, err := m.peerRepo.GetByID(ctx, peerID)
	if err != nil {
		return err
	}

	streamID := peer.StreamID

	// Get all peer connections
	connections, err := m.meshRepo.GetConnections(ctx, peerID)
	if err != nil {
		return err
	}

	// Remove all connections
	for _, conn := range connections {
		if err := m.meshRepo.RemoveConnection(ctx, conn.FromPeer, conn.ToPeer); err != nil {
			m.logger.Warnw("failed to remove connection",
				"from_peer", conn.FromPeer,
				"to_peer", conn.ToPeer,
				"error", err,
			)
		}
	}

	// Remove peer
	if err := m.peerRepo.Remove(ctx, peerID); err != nil {
		return err
	}

	// Rebalance mesh after peer removal
	go func() {
		if err := m.rebalanceStream(ctx, streamID); err != nil {
			m.logger.Warnw("failed to rebalance mesh after peer removal",
				"peer_id", peerID,
				"stream_id", streamID,
				"error", err,
			)
		}
	}()

	return nil
}

func (m *meshService) UpdatePeerMetrics(ctx context.Context, peerID domain.PeerID, metrics domain.NetworkMetrics) error {
	return m.peerRepo.UpdateMetrics(ctx, peerID, metrics)
}

// FindOptimalSources finds the best source peers for a target peer using improved scoring
func (m *meshService) FindOptimalSources(ctx context.Context, streamID domain.StreamID, targetPeer domain.PeerID, count int) ([]*domain.Peer, error) {
	// Get all peers in the stream
	allPeers, err := m.peerRepo.FindByStream(ctx, streamID)
	if err != nil {
		return nil, err
	}

	// Get target peer to calculate relative metrics
	targetPeerData, err := m.peerRepo.GetByID(ctx, targetPeer)
	if err != nil {
		return nil, err
	}

	// Get existing connections to avoid duplicates
	existingConnections, err := m.meshRepo.GetConnections(ctx, targetPeer)
	if err != nil {
		return nil, err
	}
	connectedPeers := make(map[domain.PeerID]bool)
	for _, conn := range existingConnections {
		if conn.FromPeer == targetPeer {
			connectedPeers[conn.ToPeer] = true
		} else {
			connectedPeers[conn.FromPeer] = true
		}
	}

	// Exclude target peer and unsuitable candidates
	var candidates []*scoredPeer
	for _, peer := range allPeers {
		if peer.ID == targetPeer {
			continue
		}
		if !peer.Capabilities.IsPublisher && !peer.Capabilities.CanRelay {
			continue
		}
		if peer.Metrics.Bandwidth <= 0 {
			continue
		}

		// Check if peer already has max connections
		peerConnections, err := m.meshRepo.GetConnections(ctx, peer.ID)
		if err == nil && len(peerConnections) >= m.config.MaxConnectionsPerPeer {
			continue
		}

		// Calculate score for this candidate
		score := m.calculatePeerScore(peer, targetPeerData)
		candidates = append(candidates, &scoredPeer{
			Peer:  peer,
			Score: score,
		})
	}

	if len(candidates) == 0 {
		return nil, domain.ErrPeerNotFound
	}

	// Sort by score (descending)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	// Return best candidates, avoiding already connected peers
	var result []*domain.Peer
	for _, candidate := range candidates {
		if len(result) >= count {
			break
		}
		if !connectedPeers[candidate.Peer.ID] {
			result = append(result, candidate.Peer)
		}
	}

	return result, nil
}

type scoredPeer struct {
	Peer  *domain.Peer
	Score float64
}

// calculatePeerScore calculates a comprehensive score for a peer using weighted metrics
func (m *meshService) calculatePeerScore(peer *domain.Peer, targetPeer *domain.Peer) float64 {
	score := 0.0

	// Latency component (lower is better, normalized)
	latencyScore := 1.0
	if peer.Metrics.Latency > 0 {
		// Normalize latency: 0ms = 1.0, 200ms+ = 0.0
		latencyMs := float64(peer.Metrics.Latency) / float64(time.Millisecond)
		if latencyMs < 200 {
			latencyScore = 1.0 - (latencyMs / 200.0)
		} else {
			latencyScore = 0.0
		}
	}
	score += latencyScore * m.config.LatencyWeight * 100.0

	// Bandwidth component (higher is better, normalized)
	bandwidthScore := 0.0
	if peer.Metrics.Bandwidth > 0 {
		// Normalize bandwidth: assume max 10000 kbps = 1.0
		maxBandwidth := 10000.0
		bandwidthScore = math.Min(float64(peer.Metrics.Bandwidth)/maxBandwidth, 1.0)
	}
	score += bandwidthScore * m.config.BandwidthWeight * 100.0

	// Reliability component (lower packet loss = higher score)
	reliabilityScore := 1.0 - peer.Metrics.PacketLoss
	if reliabilityScore < 0 {
		reliabilityScore = 0
	}
	score += reliabilityScore * m.config.ReliabilityWeight * 100.0

	// Publisher bonus
	if peer.Capabilities.IsPublisher {
		score += 20.0
	}

	// Relay capability bonus
	if peer.Capabilities.CanRelay {
		score += 10.0
	}

	// Penalty for high CPU usage (indicates overload)
	if peer.Metrics.CPUUsage > 80.0 {
		score -= 15.0
	} else if peer.Metrics.CPUUsage > 60.0 {
		score -= 5.0
	}

	return score
}

// BuildOptimalMesh builds an optimized mesh network for a stream
func (m *meshService) BuildOptimalMesh(ctx context.Context, streamID domain.StreamID) error {
	peers, err := m.peerRepo.FindByStream(ctx, streamID)
	if err != nil {
		return err
	}

	if len(peers) == 0 {
		return nil
	}

	// Separate publishers and subscribers
	var publishers []*domain.Peer
	var subscribers []*domain.Peer
	for _, peer := range peers {
		if peer.Capabilities.IsPublisher {
			publishers = append(publishers, peer)
		} else {
			subscribers = append(subscribers, peer)
		}
	}

	if len(publishers) == 0 {
		return fmt.Errorf("no publishers found for stream %s", streamID)
	}

	// Build connections for each subscriber
	for _, subscriber := range subscribers {
		// Get current connections for this subscriber
		currentConnections, err := m.meshRepo.GetConnections(ctx, subscriber.ID)
		if err != nil {
			m.logger.Warnw("failed to get current connections",
				"peer_id", subscriber.ID,
				"error", err,
			)
			continue
		}

		currentCount := len(currentConnections)

		// Determine target connection count
		targetCount := m.config.MaxConnections
		if currentCount < m.config.MinConnections {
			targetCount = m.config.MinConnections
		}

		// If we have enough connections, check if we need to optimize
		if currentCount >= targetCount {
			// Check if we should rebalance (replace poor connections)
			if err := m.optimizeSubscriberConnections(ctx, streamID, subscriber, currentConnections); err != nil {
				m.logger.Warnw("failed to optimize subscriber connections",
					"peer_id", subscriber.ID,
					"error", err,
				)
			}
			continue
		}

		// Find optimal sources
		neededCount := targetCount - currentCount
		sources, err := m.FindOptimalSources(ctx, streamID, subscriber.ID, neededCount)
		if err != nil {
			m.logger.Warnw("failed to find optimal sources",
				"peer_id", subscriber.ID,
				"error", err,
			)
			continue
		}

		// Create connections with found sources
		for _, source := range sources {
			conn := &domain.PeerConnection{
				FromPeer:  source.ID,
				ToPeer:    subscriber.ID,
				Direction: domain.DirectionOutbound,
				Quality:   domain.StreamQuality{Quality: "auto"},
				OpenedAt:  time.Now(),
				Bitrate:   source.Metrics.Bandwidth,
			}

			if err := m.meshRepo.AddConnection(ctx, conn); err != nil {
				m.logger.Warnw("failed to add connection",
					"from_peer", source.ID,
					"to_peer", subscriber.ID,
					"error", err,
				)
			}
		}
	}

	return nil
}

// optimizeSubscriberConnections replaces poor connections with better ones
func (m *meshService) optimizeSubscriberConnections(ctx context.Context, streamID domain.StreamID, subscriber *domain.Peer, currentConnections []*domain.PeerConnection) error {
	// Score current connections
	type connScore struct {
		Conn  *domain.PeerConnection
		Score float64
	}

	var scoredConns []connScore
	for _, conn := range currentConnections {
		// Get peer data for scoring
		peerID := conn.FromPeer
		if peerID == subscriber.ID {
			peerID = conn.ToPeer
		}

		peer, err := m.peerRepo.GetByID(ctx, peerID)
		if err != nil {
			continue
		}

		score := m.calculatePeerScore(peer, subscriber)
		scoredConns = append(scoredConns, connScore{
			Conn:  conn,
			Score: score,
		})
	}

	// Sort by score (ascending - worst first)
	sort.Slice(scoredConns, func(i, j int) bool {
		return scoredConns[i].Score < scoredConns[j].Score
	})

	// Find better alternatives for worst connections
	allPeers, err := m.peerRepo.FindByStream(ctx, streamID)
	if err != nil {
		return err
	}

	// Create set of currently connected peer IDs
	connectedSet := make(map[domain.PeerID]bool)
	for _, conn := range currentConnections {
		peerID := conn.FromPeer
		if peerID == subscriber.ID {
			peerID = conn.ToPeer
		}
		connectedSet[peerID] = true
	}

	// Try to replace worst connections
	replaced := 0
	maxReplacements := len(scoredConns) / 4 // Replace up to 25% of connections

	for i := 0; i < len(scoredConns) && replaced < maxReplacements; i++ {
		worstConn := scoredConns[i]

		// Find better alternative
		var bestAlternative *domain.Peer
		bestScore := worstConn.Score

		for _, peer := range allPeers {
			if peer.ID == subscriber.ID {
				continue
			}
			if connectedSet[peer.ID] {
				continue
			}
			if !peer.Capabilities.IsPublisher && !peer.Capabilities.CanRelay {
				continue
			}

			score := m.calculatePeerScore(peer, subscriber)
			if score > bestScore {
				bestScore = score
				bestAlternative = peer
			}
		}

		// Replace if we found a better alternative
		if bestAlternative != nil {
			// Remove old connection
			if err := m.meshRepo.RemoveConnection(ctx, worstConn.Conn.FromPeer, worstConn.Conn.ToPeer); err != nil {
				m.logger.Warnw("failed to remove old connection",
					"from_peer", worstConn.Conn.FromPeer,
					"to_peer", worstConn.Conn.ToPeer,
					"error", err,
				)
				continue
			}

			// Add new connection
			newConn := &domain.PeerConnection{
				FromPeer:  bestAlternative.ID,
				ToPeer:    subscriber.ID,
				Direction: domain.DirectionOutbound,
				Quality:   domain.StreamQuality{Quality: "auto"},
				OpenedAt:  time.Now(),
				Bitrate:   bestAlternative.Metrics.Bandwidth,
			}

			if err := m.meshRepo.AddConnection(ctx, newConn); err != nil {
				m.logger.Warnw("failed to add new connection",
					"from_peer", bestAlternative.ID,
					"to_peer", subscriber.ID,
					"error", err,
				)
				continue
			}

			connectedSet[bestAlternative.ID] = true
			replaced++
		}
	}

	return nil
}

// rebalanceStream rebalances connections for a specific stream
func (m *meshService) rebalanceStream(ctx context.Context, streamID domain.StreamID) error {
	return m.BuildOptimalMesh(ctx, streamID)
}

// Additional methods for mesh network operations
func (m *meshService) GetPeerConnections(ctx context.Context, peerID domain.PeerID) ([]*domain.PeerConnection, error) {
	return m.meshRepo.GetConnections(ctx, peerID)
}

func (m *meshService) AddConnection(ctx context.Context, conn *domain.PeerConnection) error {
	return m.meshRepo.AddConnection(ctx, conn)
}

func (m *meshService) RemoveConnection(ctx context.Context, fromPeer, toPeer domain.PeerID) error {
	return m.meshRepo.RemoveConnection(ctx, fromPeer, toPeer)
}

// GetOptimalPath finds the optimal path between two peers using BFS
func (m *meshService) GetOptimalPath(ctx context.Context, sourcePeer, targetPeer domain.PeerID) ([]domain.PeerID, error) {
	if sourcePeer == targetPeer {
		return []domain.PeerID{sourcePeer}, nil
	}

	// Get source peer to find stream ID
	sourcePeerData, err := m.peerRepo.GetByID(ctx, sourcePeer)
	if err != nil {
		return nil, err
	}

	// Get all peers in the same stream
	streamPeers, err := m.peerRepo.FindByStream(ctx, sourcePeerData.StreamID)
	if err != nil {
		return nil, err
	}

	// Build adjacency list from connections
	graph := make(map[domain.PeerID][]domain.PeerID)
	peerSet := make(map[domain.PeerID]bool)

	for _, peer := range streamPeers {
		peerSet[peer.ID] = true
		connections, err := m.meshRepo.GetConnections(ctx, peer.ID)
		if err != nil {
			continue
		}

		for _, conn := range connections {
			// Add bidirectional edges
			if conn.FromPeer == peer.ID {
				graph[conn.FromPeer] = append(graph[conn.FromPeer], conn.ToPeer)
			}
			if conn.ToPeer == peer.ID {
				graph[conn.ToPeer] = append(graph[conn.ToPeer], conn.FromPeer)
			}
		}
	}

	// Check if both peers are in the same stream
	if !peerSet[sourcePeer] || !peerSet[targetPeer] {
		return nil, fmt.Errorf("peers are not in the same stream")
	}

	// BFS to find shortest path
	queue := []domain.PeerID{sourcePeer}
	visited := make(map[domain.PeerID]bool)
	parent := make(map[domain.PeerID]domain.PeerID)
	visited[sourcePeer] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current == targetPeer {
			// Reconstruct path
			path := []domain.PeerID{targetPeer}
			node := targetPeer
			for node != sourcePeer {
				node = parent[node]
				path = append([]domain.PeerID{node}, path...)
			}
			return path, nil
		}

		for _, neighbor := range graph[current] {
			if !visited[neighbor] && peerSet[neighbor] {
				visited[neighbor] = true
				parent[neighbor] = current
				queue = append(queue, neighbor)
			}
		}
	}

	return nil, fmt.Errorf("no path found from %s to %s", sourcePeer, targetPeer)
}

// Stop stops the rebalancing loop
func (m *meshService) Stop() {
	if m.rebalanceTicker != nil {
		m.rebalanceTicker.Stop()
	}
	close(m.rebalanceStop)
}
