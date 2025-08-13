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
func Announce(serviceName string, secretCode string, port int) {
	hashedKey := hashCode(secretCode)
	network := "_p2p-" + hashedKey + "._tcp"

	server, err := zeroconf.Register(serviceName, network, "local.", port, []string{"textv=0", "app=p2p"}, nil)
	if err != nil {
		log.Fatalf("Failed to announce service: %v", err)
	}
	defer server.Shutdown()

	fmt.Printf("Announcing service [%s] with hash [%s] on port %d...\n", serviceName, hashedKey, port)
	time.Sleep(10 * time.Minute) // Keep service alive for testing
}

// FindPeers looks for peers with the same hashed secret code
func FindPeers(secretCode string, timeout time.Duration) []Peer {
	hashedKey := hashCode(secretCode)
	service := "_p2p-" + hashedKey + "._tcp"

	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		log.Fatalf("Failed to initialize resolver: %v", err)
	}

	entries := make(chan *zeroconf.ServiceEntry)
	peers := []Peer{}

	go func(results <-chan *zeroconf.ServiceEntry) {
		for entry := range results {
			for _, ip := range entry.AddrIPv4 {
				peers = append(peers, Peer{
					ID:   entry.Instance,
					IP:   ip.String(),
					Port: entry.Port,
				})
				fmt.Printf("Found peer: %s (%s:%d)\n", entry.Instance, ip.String(), entry.Port)
			}
		}
	}(entries)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	err = resolver.Browse(ctx, service, "local.", entries)
	if err != nil {
		log.Fatalf("Failed to browse: %v", err)
	}

	<-ctx.Done()
	return peers
}
