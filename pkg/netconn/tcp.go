package netconn

import (
	"fmt"
	"net"
)

func ConnectTCP(ip string, port int) error {
	addr := fmt.Sprintf("%s:%d", ip, port)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	message := "Hello from client!\n"
	_, err = conn.Write([]byte(message))
	if err != nil {
		return err
	}

	fmt.Printf("Sent message to %s\n", addr)
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
	buf := make([]byte, 1024)
	n, _ := conn.Read(buf)
	fmt.Printf("Received: %s", string(buf[:n]))
}
