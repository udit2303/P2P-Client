package util

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/pion/stun"
)

// GetLocalIPs returns all non-loopback IPv4 addresses on active interfaces.
func GetLocalIPs() ([]string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var ips []string
	for _, iface := range ifaces {
		if (iface.Flags&net.FlagUp) == 0 || (iface.Flags&net.FlagLoopback) != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil { // Skip IPv6 for now
				continue
			}
			ips = append(ips, ip.String())
		}
	}
	if len(ips) == 0 {
		return nil, errors.New("no active IPv4 addresses found")
	}
	return ips, nil
}

// GetPublicIP discovers the public IPv4 address using a STUN Binding Request.
// It returns the observed public IP and port (as seen by the STUN server).
func GetPublicIP(timeout time.Duration) (string, int, error) {
	server := "stun.l.google.com:19302"
	d := &net.Dialer{Timeout: timeout}
	conn, err := d.Dial("udp4", server)
	if err != nil {
		return "", 0, fmt.Errorf("stun dial failed: %w", err)
	}
	defer conn.Close()

	c, err := stun.NewClient(conn)
	if err != nil {
		return "", 0, fmt.Errorf("stun client create failed: %w", err)
	}
	defer c.Close()

	var pubIP string
	var pubPort int
	var reqErr error

	// Set overall deadline
	_ = conn.SetDeadline(time.Now().Add(timeout))

	err = c.Do(stun.MustBuild(stun.TransactionID, stun.BindingRequest), func(res stun.Event) {
		if res.Error != nil {
			reqErr = res.Error
			return
		}
		var xorAddr stun.XORMappedAddress
		if getErr := xorAddr.GetFrom(res.Message); getErr != nil {
			reqErr = getErr
			return
		}
		pubIP = xorAddr.IP.String()
		pubPort = xorAddr.Port
	})
	if err != nil {
		return "", 0, fmt.Errorf("stun transaction failed: %w", err)
	}
	if reqErr != nil {
		return "", 0, reqErr
	}
	if pubIP == "" {
		return "", 0, errors.New("stun returned empty IP")
	}
	return pubIP, pubPort, nil
}
