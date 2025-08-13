package main

import (
	"fmt"
	"time"

	"github.com/udit2303/p2p-client/pkg/discovery"
	"github.com/udit2303/p2p-client/pkg/netconn"
)

func main() {
	const port = 9000
	// Start TCP server in background
	go netconn.StartTCPServer(port)
	// Announce service
	go discovery.Announce("node1", "123", port)

	time.Sleep(2 * time.Second) // wait a bit before browsing
	// Find peers
	peers := discovery.FindPeers("123", 5*time.Second)
	for _, peer := range peers {
		if peer.Port != port { // avoid connecting to self
			err := netconn.ConnectTCP(peer.IP, peer.Port)
			if err != nil {
				fmt.Println("Connection failed:", err)
			}
		} else {
			fmt.Println("IP FOUND: ", peer.IP, ":", peer.Port)
		}
	}

	select {} // keep main alive
}
