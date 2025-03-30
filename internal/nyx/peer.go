package nyx

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	jsonPeerPath = "peers.json"
)

// NumPeers returns the number of peers indicated by the config files within raftDir
// This code makes assumptions about how the Raft module works
func NumPeers(raftDir string) (int, error) {
	buf, err := os.ReadFile(filepath.Join(raftDir, jsonPeerPath)) // #nosec G304
	if err != nil && !os.IsNotExist(err) {
		return 0, err
	}

	if len(buf) == 0 {
		return 0, nil
	}

	var peerSet []string
	dec := json.NewDecoder(bytes.NewReader(buf))
	if err := dec.Decode(&peerSet); err != nil {
		return 0, err
	}
	return len(peerSet), nil
}

func (n *Nyx) JoinAllowed(raftDir string) (bool, error) {
	count, err := NumPeers(raftDir)
	if err != nil {
		return false, err
	}
	return count <= 1, nil
}
