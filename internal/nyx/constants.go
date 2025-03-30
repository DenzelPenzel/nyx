package nyx

type payloadType int

const (
	Get payloadType = iota
	Set
	Peer
)
