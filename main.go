package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/udit2303/p2p-client/pkg/discovery"
	"github.com/udit2303/p2p-client/pkg/netconn"
)

func main() {
	// Define command-line flags
	port := flag.Int("port", 8000, "Port to listen on")
	nodeName := flag.String("name", "node1", "Name of this node")
	filePath := flag.String("file", "", "Path to the file to send")
	search := flag.String("search", "", "Search for a peer")
	flag.Parse()

	// Check if file path is provided if this node is a sender
	if *filePath != "" {
		if _, err := os.Stat(*filePath); os.IsNotExist(err) {
			fmt.Printf("Error: File '%s' does not exist\n", *filePath)
			os.Exit(1)
		}
		fmt.Printf("Will send file: %s\n", *filePath)
	}

	fmt.Printf("Starting node '%s' on port %d\n", *nodeName, *port)

	// Start TCP server in background
	go netconn.StartTCPServer(*port)
	// Announce service
	go discovery.Announce(*nodeName, "123", *port)

	time.Sleep(2 * time.Second) // wait a bit before browsing
	// Find peers
	if *search != "" {
		peers := discovery.FindPeers(*search, 5*time.Second)
		for _, peer := range peers {
			if peer.Port != *port { // avoid connecting to self
				fmt.Printf("Found peer: %s:%d\n", peer.IP, peer.Port)
				err := netconn.ConnectTCP(peer.IP, peer.Port, *filePath)
				if err != nil {
					fmt.Println("Connection failed:", err)
				}
			} else {
				fmt.Println("This node:", peer.IP, ":", peer.Port)
			}
		}
	}

	select {} // keep main alive
}
