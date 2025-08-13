package discovery

// Peer represents a node in the P2P network.
type Peer struct {
	ID        string
	IP        string
	Port      int
	PublicKey []byte
}
type Discovery interface {
	Announce(serviceName string) error
	FindPeers(serviceName string) ([]Peer, error)
}
