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
	"golang.org/x/crypto/bcrypt"
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
func ConnectTCP(ip string, port int) error {
	lock.Lock()
	if connectionLocked {
		lock.Unlock()
		fmt.Println("Connection is locked. Cannot connect to server.")
		return fmt.Errorf("connection locked")
	}
	lock.Unlock()

	addr := fmt.Sprintf("%s:%d", ip, port)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	nonce, _ := bufio.NewReader(conn).ReadString('\n')
	nonce = strings.TrimSpace(nonce)
	fmt.Println("Received nonce:", nonce)

	// Step 2: Prompt user for passcode
	fmt.Print("Enter passcode: ")
	inputPass, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	inputPass = strings.TrimSpace(inputPass)

	// Step 3: Hash(passcode + nonce) using bcrypt
	hash, _ := bcrypt.GenerateFromPassword([]byte(inputPass+nonce), bcrypt.DefaultCost)
	conn.Write([]byte(string(hash) + "\n"))

	// Step 4: Get result
	result, _ := bufio.NewReader(conn).ReadString('\n')
	fmt.Println("Server says:", strings.TrimSpace(result))
	e := transfer.SendFile(conn, "file.txt")
	if e != nil {
		// handle error
		fmt.Println(e)
	}
	return nil
}

func StartTCPServer(port int) {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		panic(err)
	}
	defer ln.Close()

	fmt.Printf("TCP server listening on port %d\n", port)

	for {
		lock.Lock()
		if connectionLocked {
			lock.Unlock()
			fmt.Println("Connection locked. No longer accepting new connections.")
			break // Stop accepting new connections
		}
		lock.Unlock()
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println("Connection error:", err)
			continue
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	nonce, _ := generateNonce(15)
	conn.Write([]byte(nonce + "\n"))

	// Step 2: Receive bcrypt hash from client
	clientHash, _ := bufio.NewReader(conn).ReadString('\n')
	clientHash = strings.TrimSpace(clientHash)
	// Step 4: Compare hashes
	err := bcrypt.CompareHashAndPassword([]byte(clientHash), []byte(passcode+nonce))
	if err == nil {
		conn.Write([]byte("SUCCESS\n"))
		fmt.Println("Authentication successful for a peer")
		lock.Lock()
		connectionLocked = true
		err := transfer.ReceiveFile(conn, "public")
		if err != nil {
			fmt.Println(err)
		}
		lock.Unlock()
	} else {
		conn.Write([]byte("FAIL\n"))
		fmt.Println("Authentication failed for a peer")
	}
}
