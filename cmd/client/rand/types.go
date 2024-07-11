package main

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
