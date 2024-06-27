package main

import "bufio"

type Proto interface {
	Set(rw *bufio.ReadWriter, key []byte, value []byte) error
	Add(rw *bufio.ReadWriter, key []byte, value []byte) error
	Replace(rw *bufio.ReadWriter, key []byte, value []byte) error
	Append(rw *bufio.ReadWriter, key []byte, value []byte) error
	Prepend(rw *bufio.ReadWriter, key []byte, value []byte) error
	Get(rw *bufio.ReadWriter, key []byte) ([]byte, error)
	GetWithOpaque(rw *bufio.ReadWriter, key []byte, opaque int) ([]byte, error)
	GAT(rw *bufio.ReadWriter, key []byte) ([]byte, error)
	BatchGet(rw *bufio.ReadWriter, keys [][]byte) ([][]byte, error)
	Delete(rw *bufio.ReadWriter, key []byte) error
	Touch(rw *bufio.ReadWriter, key []byte) error
}

type Op int

func (o Op) String() string {
	switch o {
	case Set:
		return "Set"
	case Add:
		return "Add"
	case Replace:
		return "Replace"
	case Append:
		return "Append"
	case Prepend:
		return "Prepend"
	case Get:
		return "Get"
	case Gat:
		return "Get and Touch"
	case Bget:
		return "Batch Get"
	case Delete:
		return "Delete"
	case Touch:
		return "Touch"
	default:
		return ""
	}
}

const (
	Get Op = iota
	Bget
	Gat
	Set
	Add
	Replace
	Append
	Prepend
	Touch
	Delete
)

var allOps = []Op{Get, Set, Add, Delete, Replace, Append, Prepend}

type metric struct {
	duration int64
	op       Op
	miss     bool
}
