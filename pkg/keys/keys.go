package keys

import (
	"crypto/sha256"
	"encoding/hex"
)

func HashCode(code string) string {
	hash := sha256.Sum256([]byte(code))
	return hex.EncodeToString(hash[:8]) // first 8 bytes for shorter mDNS name
}
