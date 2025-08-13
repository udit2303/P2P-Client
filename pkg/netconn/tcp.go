package netconn

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"
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
	} else {
		conn.Write([]byte("FAIL\n"))
		fmt.Println("Authentication failed for a peer")
	}
}
