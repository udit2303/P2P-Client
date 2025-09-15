package netconn

import (
	"bufio"
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pion/webrtc/v3"
	"github.com/udit2303/p2p-client/pkg/keys"
	"github.com/udit2303/p2p-client/pkg/transfer"
	"github.com/udit2303/p2p-client/pkg/util"
)

// sdpBlob is a simplified container for manual signaling
type sdpBlob struct {
	Type webrtc.SDPType `json:"type"`
	SDP  string         `json:"sdp"`
}

func encodeSDP(sd webrtc.SessionDescription) (string, error) {
	blob := sdpBlob{Type: sd.Type, SDP: sd.SDP}
	b, err := json.Marshal(blob)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

func decodeSDP(s string) (webrtc.SessionDescription, error) {
	var blob sdpBlob
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	if err != nil {
		return webrtc.SessionDescription{}, err
	}
	if err := json.Unmarshal(data, &blob); err != nil {
		return webrtc.SessionDescription{}, err
	}
	return webrtc.SessionDescription{Type: blob.Type, SDP: blob.SDP}, nil
}

// StartWebRTCSender starts a WebRTC sender that sends a file to a receiver over a reliable data channel.
// Manual copy-paste signaling is used. The receiver must paste the OFFER and return an ANSWER.
func StartWebRTCSender(filePath string) error {
	// Enable Detach to get io.ReadWriteCloser
	se := webrtc.SettingEngine{}
	se.DetachDataChannels()
	api := webrtc.NewAPI(webrtc.WithSettingEngine(se))

	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	}
	pc, err := api.NewPeerConnection(config)
	if err != nil {
		return err
	}
	defer pc.Close()

	// Data channel for file transfer
	dc, err := pc.CreateDataChannel("file", nil)
	if err != nil {
		return err
	}

	done := make(chan error, 1)

	dc.OnOpen(func() {
		log.Info("WebRTC data channel open; waiting for receiver public key")
		rw, err := dc.Detach()
		if err != nil {
			done <- fmt.Errorf("detach failed: %w", err)
			return
		}
		go func() {
			// Read receiver's public key (length-prefixed)
			rpubBytes, rerr := util.ReadWithLength(rw)
			if rerr != nil {
				done <- fmt.Errorf("failed to read receiver pub key: %w", rerr)
				return
			}
			rpub, perr := x509.ParsePKCS1PublicKey(rpubBytes)
			if perr != nil {
				done <- fmt.Errorf("failed to parse receiver pub key: %w", perr)
				return
			}
			// Send the file using our existing pipeline
			if err := transfer.SendFile(rw, filePath, rpub); err != nil {
				done <- err
				return
			}
			log.Info("WebRTC file transfer finished")
			done <- nil
		}()
	})

	// Create offer and gather ICE
	offer, err := pc.CreateOffer(nil)
	if err != nil {
		return err
	}
	if err := pc.SetLocalDescription(offer); err != nil {
		return err
	}
	<-webrtc.GatheringCompletePromise(pc)

	enc, err := encodeSDP(*pc.LocalDescription())
	if err != nil {
		return err
	}
	fmt.Println("--- BEGIN WEBRTC OFFER ---")
	fmt.Println(enc)
	fmt.Println("--- END WEBRTC OFFER ---")
	fmt.Print("Paste remote ANSWER and press Enter: ")
	ansLine, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	ans, err := decodeSDP(ansLine)
	if err != nil {
		return fmt.Errorf("failed to decode answer: %w", err)
	}
	if err := pc.SetRemoteDescription(ans); err != nil {
		return fmt.Errorf("set remote failed: %w", err)
	}

	// Add a local ICE candidate manually
	pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate != nil {
			fmt.Printf("ICE Candidate: %s\n", candidate.ToJSON().Candidate)
		}
	})

	// Wait for completion
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// StartWebRTCReceiver starts a WebRTC receiver that accepts a file over a reliable data channel.
// It prints an ANSWER to paste back to the sender.
func StartWebRTCReceiver(outputDir string) error {
	se := webrtc.SettingEngine{}
	se.DetachDataChannels()
	api := webrtc.NewAPI(webrtc.WithSettingEngine(se))

	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
			{URLs: []string{"stun:stun1.l.google.com:19302"}},
			{URLs: []string{"stun:stun.stunprotocol.org:3478"}},
			{URLs: []string{"stun:stun.cloudflare.com:3478"}},
		},
	}
	pc, err := api.NewPeerConnection(config)
	if err != nil {
		return err
	}
	defer pc.Close()

	done := make(chan error, 1)

	pc.OnDataChannel(func(dc *webrtc.DataChannel) {
		dc.OnOpen(func() {
			log.Info("WebRTC data channel open; sending receiver public key and awaiting file")
			rw, err := dc.Detach()
			if err != nil {
				done <- fmt.Errorf("detach failed: %w", err)
				return
			}
			go func() {
				// Load and send our public key so sender can encrypt a session key
				pub, kerr := keys.LoadPublicKey()
				if kerr != nil {
					done <- fmt.Errorf("failed to load public key: %w", kerr)
					return
				}
				pubBytes := x509.MarshalPKCS1PublicKey(pub)
				if err := util.SendWithLength(rw, pubBytes); err != nil {
					done <- fmt.Errorf("failed to send public key: %w", err)
					return
				}
				if err := transfer.ReceiveFile(rw, outputDir); err != nil {
					done <- err
					return
				}
				log.Info("WebRTC file received successfully")
				done <- nil
			}()
		})
	})

	fmt.Print("Paste remote OFFER and press Enter: ")
	offerLine, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	offer, err := decodeSDP(offerLine)
	if err != nil {
		return fmt.Errorf("failed to decode offer: %w", err)
	}
	if err := pc.SetRemoteDescription(offer); err != nil {
		return fmt.Errorf("set remote failed: %w", err)
	}
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		return err
	}
	if err := pc.SetLocalDescription(answer); err != nil {
		return err
	}
	<-webrtc.GatheringCompletePromise(pc)

	enc, err := encodeSDP(*pc.LocalDescription())
	if err != nil {
		return err
	}
	fmt.Println("--- BEGIN WEBRTC ANSWER ---")
	fmt.Println(enc)
	fmt.Println("--- END WEBRTC ANSWER ---")

	// Wait for completion
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
