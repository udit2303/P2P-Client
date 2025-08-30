package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/udit2303/p2p-client/pkg/discovery"
	"github.com/udit2303/p2p-client/pkg/netconn"
	"github.com/udit2303/p2p-client/pkg/util"
)

var (
	log = util.DefaultLogger()
)

// GetLocalIP returns the non-loopback local IP of the machine
func GetLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}
	return "", fmt.Errorf("no non-loopback address found")
}

func main() {
	// Set up context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle OS signals for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Info("Received signal, shutting down...", "signal", sig)
		cancel()
	}()

	// Define command-line flags
	port := flag.Int("port", 8000, "Port to listen on")
	nodeName := flag.String("name", "node1", "Name of this node")
	filePath := flag.String("file", "", "Path to the file to send")
	search := flag.String("search", "", "Search for a peer")
	connect := flag.String("connect", "", "Directly connect to peer at ip:port (over internet)")
	debug := flag.Bool("debug", false, "Enable debug logging")
	flag.Parse()

	// Configure logger based on debug flag
	if *debug {
		log = util.NewLogger(os.Stdout, util.DebugLevel)
	}

	// Add node name to all log messages
	log = log.With("node", *nodeName, "port", *port)

	// Check if file path is provided if this node is a sender
	if *filePath != "" {
		if _, err := os.Stat(*filePath); os.IsNotExist(err) {
			log.Error("File does not exist", "path", *filePath)
			os.Exit(1)
		}
		log.Info("Will send file", "path", *filePath)
	}

	log.Info("Starting P2P node")

	// Show local and public IPs to the user
	if localIPs, err := util.GetLocalIPs(); err == nil {
		log.Info("Local IPv4 addresses", "ips", localIPs)
	} else {
		log.Warn("Unable to get local IPs", "error", err)
	}
	if pubIP, pubPort, err := util.GetPublicIP(3 * time.Second); err == nil {
		log.Info("Public internet address (via STUN)", "ip", pubIP, "port", pubPort)
	} else {
		log.Warn("Unable to determine public IP (STUN)", "error", err)
	}

	// Start TCP server in background
	errCh := make(chan error, 1)
	go func() {
		if err := netconn.StartTCPServer(*port); err != nil {
			errCh <- fmt.Errorf("TCP server error: %w", err)
		}
	}()

	// Announce service
	go func() {
		if err := discovery.Announce(*nodeName, "123", *port); err != nil {
			errCh <- fmt.Errorf("service announcement error: %w", err)
		}
	}()

	// Wait a bit for services to start
	select {
	case <-time.After(3 * time.Second):
		log.Debug("Services started successfully")
	case err := <-errCh:
		log.Error("Failed to start services", "error", err)
		os.Exit(1)
	}

	// Direct connection if connect flag is provided (ip:port)
	if *connect != "" {
		host, cport, err := net.SplitHostPort(*connect)
		if err != nil {
			log.Error("Invalid -connect address, expected ip:port", "value", *connect, "error", err)
		} else {
			// Parse port
			var p int
			if _, err := fmt.Sscanf(cport, "%d", &p); err != nil {
				log.Error("Invalid port in -connect", "port", cport, "error", err)
			} else {
				log.Info("Connecting to peer (direct)", "address", *connect)
				if err := netconn.ConnectTCP(host, p, *filePath); err != nil {
					log.Error("Direct connect failed", "address", *connect, "error", err)
				}
			}
		}
	}

	// Find peers if search flag is provided
	if *search != "" {
		log.Info("Searching for peers", "service", *search)
		peers, err := discovery.FindPeers(*search, 5*time.Second)
		if err != nil {
			log.Error("Error finding peers", "error", err)
		} else {
			log.Info("Discovered peers", "count", len(peers), "peers", peers)
		}

		for _, peer := range peers {
			// Skip if this is our own node
			if peer.ID == *nodeName {
				log.Debug("Skipping self", "peer", peer.ID)
				continue
			}

			log.Info("Attempting to connect to peer", "peer", peer.ID, "address", fmt.Sprintf("%s:%d", peer.IP, peer.Port))

			// Use retry with backoff for connection attempts
			err := util.RetryWithBackoff(ctx, 3, time.Second, func() error {
				return netconn.ConnectTCP(peer.IP, peer.Port, *filePath)
			})

			if err != nil {
				log.Error("Failed to connect to peer",
					"peer", peer.ID,
					"address", fmt.Sprintf("%s:%d", peer.IP, peer.Port),
					"error", err)
			} else {
				log.Info("Successfully connected to peer", "peer", peer.ID)
			}
		}
	}

	// Wait for context cancellation (from signal or error)
	<-ctx.Done()
	log.Info("Shutting down...")
}
