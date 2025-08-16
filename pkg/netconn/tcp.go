package netconn

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/udit2303/p2p-client/pkg/transfer"
	"github.com/udit2303/p2p-client/pkg/util"
	"golang.org/x/crypto/bcrypt"
)

var (
	log = util.DefaultLogger()
)

var (
	connectionLocked bool
	lock             sync.Mutex
)

const passcode = "hello123"

func generateNonce(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
func ConnectTCP(ip string, port int, filePath string) error {
	// Check if we can establish a new connection
	lock.Lock()
	if connectionLocked {
		lock.Unlock()
		log.Warn("Connection attempt rejected: connection is locked")
		return fmt.Errorf("connection locked")
	}
	connectionLocked = true
	lock.Unlock()

	log.Info("Attempting to establish connection", "remote", fmt.Sprintf("%s:%d", ip, port))

	// Ensure we unlock when done
	defer func() {
		lock.Lock()
		connectionLocked = false
		lock.Unlock()
		log.Debug("Connection lock released")
	}()

	// Use net.JoinHostPort to properly handle both IPv4 and IPv6 addresses
	addr := net.JoinHostPort(ip, fmt.Sprintf("%d", port))
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		log.Error("Failed to establish connection", "error", err)
		return fmt.Errorf("connection failed: %w", err)
	}
	defer conn.Close()

	log.Debug("Connection established, waiting for nonce")

	nonce, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		log.Error("Failed to read nonce", "error", err)
		return fmt.Errorf("failed to read nonce: %w", err)
	}
	nonce = strings.TrimSpace(nonce)
	log.Debug("Received nonce", "nonce", nonce)

	// Step 2: Prompt user for passcode
	log.Info("Authentication required")
	fmt.Print("Enter passcode: ")
	inputPass, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		log.Error("Failed to read passcode", "error", err)
		return fmt.Errorf("failed to read passcode: %w", err)
	}
	inputPass = strings.TrimSpace(inputPass)

	// Step 3: Hash(passcode + nonce) using bcrypt
	hash, err := bcrypt.GenerateFromPassword([]byte(inputPass+nonce), bcrypt.DefaultCost)
	if err != nil {
		log.Error("Failed to hash passcode", "error", err)
		return fmt.Errorf("authentication failed: %w", err)
	}

	_, err = conn.Write([]byte(string(hash) + "\n"))
	if err != nil {
		log.Error("Failed to send authentication hash", "error", err)
		return fmt.Errorf("failed to send authentication: %w", err)
	}

	// Step 4: Get result
	result, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		log.Error("Failed to read authentication response", "error", err)
		return fmt.Errorf("failed to read server response: %w", err)
	}
	result = strings.TrimSpace(result)
	log.Debug("Authentication response received", "status", result)

	if result != "SUCCESS" {
		log.Warn("Authentication failed", "response", result)
		return fmt.Errorf("authentication failed: server responded with '%s'", result)
	}

	log.Info("Authentication successful")
	if filePath != "" {
		log.Info("Starting file transfer", "file", filePath)
		err = transfer.SendFile(conn, filePath)
		if err != nil {
			log.Error("File transfer failed", "error", err, "file", filePath)
			return fmt.Errorf("file transfer failed: %w", err)
		}
		log.Info("File transfer completed successfully", "file", filePath)
	}
	return nil
}

func StartTCPServer(port int) error {
	addr := fmt.Sprintf(":%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to start TCP server: %w", err)
	}
	defer ln.Close()

	log.Info("TCP server started", "address", addr)

	for {
		lock.Lock()
		if connectionLocked {
			log.Info("Connection locked, not accepting new connections")
			lock.Unlock()
			return nil
		}
		lock.Unlock()

		conn, err := ln.Accept()
		if err != nil {
			log.Error("Error accepting connection", "error", err)
			continue
		}

		go func(c net.Conn) {
			remoteAddr := c.RemoteAddr().String()
			log.Info("New connection accepted", "remote", remoteAddr)
			handleConnection(c)
			log.Info("Connection closed", "remote", remoteAddr)
		}(conn)
	}
}

func handleConnection(conn net.Conn) {
	remoteAddr := conn.RemoteAddr().String()
	log := log.With("remote", remoteAddr)

	defer func() {
		if err := conn.Close(); err != nil {
			log.Error("Error closing connection", "error", err)
		}
	}()

	// Generate and send nonce
	nonce, err := generateNonce(15)
	if err != nil {
		log.Error("Failed to generate nonce", "error", err)
		return
	}

	log.Debug("Sending nonce to client")
	if _, err := conn.Write([]byte(nonce + "\n")); err != nil {
		log.Error("Failed to send nonce", "error", err)
		return
	}

	// Receive and verify client hash
	clientHash, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		log.Error("Failed to read client hash", "error", err)
		return
	}
	clientHash = strings.TrimSpace(clientHash)

	log.Debug("Verifying client authentication")
	err = bcrypt.CompareHashAndPassword([]byte(clientHash), []byte(passcode+nonce))
	if err != nil {
		log.Warn("Authentication failed", "error", err)
		if _, err := conn.Write([]byte("FAIL\n")); err != nil {
			log.Error("Failed to send auth failure response", "error", err)
		}
		return
	}

	log.Info("Authentication successful")
	if _, err := conn.Write([]byte("SUCCESS\n")); err != nil {
		log.Error("Failed to send auth success response", "error", err)
		return
	}

	// Lock connection for file transfer
	lock.Lock()
	if connectionLocked {
		log.Warn("Connection already locked, rejecting transfer")
		lock.Unlock()
		return
	}
	connectionLocked = true
	lock.Unlock()

	// Ensure we unlock when done
	defer func() {
		lock.Lock()
		connectionLocked = false
		lock.Unlock()
		log.Debug("Connection lock released")
	}()

	// Handle file transfer
	log.Info("Starting file transfer")
	if err := transfer.ReceiveFile(conn, "public"); err != nil {
		log.Error("File received failed", "error", err)
	} else {
		log.Info("File received successfully")
	}
}
