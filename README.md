# P2P File Transfer Client

A secure P2P file sharing client supporting both local network and internet transfers.

## Usage

### Local Network Transfer (mDNS Discovery)

**Receiver:**
```bash
go run . -name receiver-node -port 8000
```

**Sender:**
```bash
go run . -file myfile.txt -search "123"
```
The sender searches for services with ID "123" (hardcoded in announcement), finds the receiver, and connects automatically.

### Internet Transfer (WebRTC)

**Receiver:**
```bash
go run . -webrtc-recv -out downloads
```
Paste the OFFER when prompted, then copy the printed ANSWER.

**Sender:**
```bash
go run . -webrtc-send -file myfile.txt
```
Copy the printed OFFER to receiver, then paste the ANSWER back.

### Direct IP Connection

**Receiver (with port forwarding):**
```bash
go run .
```

**Sender:**
```bash
go run . -connect 203.0.113.10:8000 -file myfile.txt
```

## Features

- **mDNS discovery** for local network
- **WebRTC** for NAT traversal (internet P2P)  
- **RSA-4096 + AES-256** encryption
- **Chunked transfers** with integrity verification
- Shows local and public IP addresses on startup

## Options

- `-name node-name` - Name of this node (default: "node1")
- `-port number` - Port to listen on (default: 8000)
- `-file path` - File to send
- `-search service` - Search for peers by service ID ("123")
- `-out dir` - Output directory for received files  
- `-connect ip:port` - Connect directly to IP
- `-webrtc-send` - Send via WebRTC
- `-webrtc-recv` - Receive via WebRTC
- `-debug` - Enable debug loggingl