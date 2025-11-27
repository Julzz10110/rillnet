package main

/*
import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"rillnet/internal/core/domain"
	"rillnet/internal/core/ports"
	"rillnet/internal/core/services"
	"rillnet/internal/infrastructure/repositories/memory"
)

type StressTest struct {
	streamService  ports.StreamService
	meshService    ports.MeshService
	metricsService services.MetricsService
}

func NewStressTest() *StressTest {
	streamRepo := memory.NewMemoryStreamRepository()
	peerRepo := memory.NewMemoryPeerRepository()
	meshRepo := memory.NewMemoryMeshRepository()
	meshService := services.NewMeshService(peerRepo, meshRepo)
	metricsService := services.NewMetricsService()
	streamService := services.NewStreamService(streamRepo, peerRepo, meshRepo, meshService, metricsService)

	return &StressTest{
		streamService: streamService,
		meshService:   meshService,
	}
}

func (st *StressTest) RunConcurrentJoinLeave(numPeers int, duration time.Duration) {
	ctx := context.Background()
	streamID := domain.StreamID("stress-test-stream")

	// Create test stream
	_, err := st.streamService.CreateStream(ctx, "stress-test", "stress-owner", numPeers*2)
	if err != nil {
		log.Fatalf("Failed to create stream: %v", err)
	}

	var wg sync.WaitGroup
	stop := make(chan bool)

	// Start peers
	for i := 0; i < numPeers; i++ {
		wg.Add(1)
		go func(peerNum int) {
			defer wg.Done()

			peerID := domain.PeerID(fmt.Sprintf("stress-peer-%d", peerNum))
			peer := &domain.Peer{
				ID:       peerID,
				StreamID: streamID,
				Capabilities: domain.PeerCapabilities{
					IsPublisher: peerNum%10 == 0, // 10% are publishers
					MaxBitrate:  1000 + rand.Intn(2000),
				},
			}

			ticker := time.NewTicker(time.Duration(rand.Intn(5000)+1000) * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case <-stop:
					return
				case <-ticker.C:
					// Join stream
					err := st.streamService.JoinStream(ctx, streamID, peer)
					if err != nil {
						log.Printf("Peer %s failed to join: %v", peerID, err)
					} else {
						log.Printf("Peer %s joined stream", peerID)
					}

					// Wait a bit
					time.Sleep(time.Duration(rand.Intn(3000)) * time.Millisecond)

					// Leave stream
					err = st.streamService.LeaveStream(ctx, streamID, peerID)
					if err != nil {
						log.Printf("Peer %s failed to leave: %v", peerID, err)
					} else {
						log.Printf("Peer %s left stream", peerID)
					}
				}
			}
		}(i)
	}

	// Run for specified duration
	log.Printf("Running stress test for %v with %d peers", duration, numPeers)
	time.Sleep(duration)

	// Stop all goroutines
	close(stop)
	wg.Wait()

	log.Printf("Stress test completed")
}

func (st *StressTest) MeasurePerformance(numOperations int) {
	ctx := context.Background()
	streamID := domain.StreamID("perf-test-stream")

	// Create test stream
	_, err := st.streamService.CreateStream(ctx, "perf-test", "perf-owner", numOperations)
	if err != nil {
		log.Fatalf("Failed to create stream: %v", err)
	}

	start := time.Now()

	// Perform operations
	for i := 0; i < numOperations; i++ {
		peerID := domain.PeerID(fmt.Sprintf("perf-peer-%d", i))
		peer := &domain.Peer{
			ID:       peerID,
			StreamID: streamID,
			Capabilities: domain.PeerCapabilities{
				IsPublisher: i%5 == 0,
				MaxBitrate:  1000,
			},
		}

		// Join
		err := st.streamService.JoinStream(ctx, streamID, peer)
		if err != nil {
			log.Printf("Operation %d failed: %v", i, err)
		}

		// Leave
		err = st.streamService.LeaveStream(ctx, streamID, peerID)
		if err != nil {
			log.Printf("Operation %d failed: %v", i, err)
		}
	}

	duration := time.Since(start)
	opsPerSecond := float64(numOperations*2) / duration.Seconds()

	log.Printf("Performance test completed:")
	log.Printf("  Operations: %d", numOperations*2)
	log.Printf("  Duration: %v", duration)
	log.Printf("  Ops/sec: %.2f", opsPerSecond)
}

func main() {
	stressTest := NewStressTest()

	fmt.Println("=== RillNet Stress Test ===")

	// Test 1: Concurrent join/leave with 100 peers for 30 seconds
	fmt.Println("\n1. Running concurrent join/leave test...")
	stressTest.RunConcurrentJoinLeave(100, 30*time.Second)

	// Test 2: Performance measurement with 1000 operations
	fmt.Println("\n2. Running performance test...")
	stressTest.MeasurePerformance(1000)

	fmt.Println("\n=== All tests completed ===")
}
*/
