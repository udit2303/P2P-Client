package discovery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"github.com/grandcat/zeroconf"
)

// hashCode hashes a code to a short 8-byte hex string
func hashCode(code string) string {
	hash := sha256.Sum256([]byte(code))
	return hex.EncodeToString(hash[:8])
}

// Announce starts advertising the service on mDNS with hashed service name
func Announce(serviceName string, secretCode string, port int) error {
	hashedKey := hashCode(secretCode)
	network := "_p2p-" + hashedKey + "._tcp"

	log.Printf("Announcing service [%s] with hash [%s] on port %d...\n", serviceName, hashedKey, port)

	server, err := zeroconf.Register(serviceName, network, "local.", port, []string{"textv=0", "app=p2p"}, nil)
	if err != nil {
		return fmt.Errorf("failed to announce service: %w", err)
	}
	defer server.Shutdown()

	// Create a context that can be cancelled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Wait for context cancellation
	<-ctx.Done()
	return nil
}

// FindPeers looks for peers with the same hashed secret code
func FindPeers(secretCode string, timeout time.Duration) ([]Peer, error) {
	hashedKey := hashCode(secretCode)
	service := "_p2p-" + hashedKey + "._tcp"

	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize resolver: %w", err)
	}

	entries := make(chan *zeroconf.ServiceEntry)
	peers := []Peer{}

	// Use a channel to signal when processing is complete
	done := make(chan struct{})

	go func() {
		defer close(done)
		for entry := range entries {
			for _, ip := range entry.AddrIPv4 {
				peers = append(peers, Peer{
					ID:   entry.Instance,
					IP:   ip.String(),
					Port: entry.Port,
				})
				log.Printf("Found peer: %s (%s:%d)\n", entry.Instance, ip.String(), entry.Port)
			}
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	err = resolver.Browse(ctx, service, "local.", entries)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to browse: %w", err)
	}

	// Wait for context to be done or all entries to be processed
	select {
	case <-ctx.Done():
		if ctx.Err() == context.DeadlineExceeded {
			log.Println("Peer discovery timed out")
		}
	case <-done:
	}

	return peers, nil
}
