package proto

import (
	"bufio"

	"github.com/DenzelPenzel/nyx/internal/common"
)

type RequestParser interface {
	Parse() (common.Request, common.RequestType, int64, error)
}

type Responder interface {
	Set(opaque uint32, quiet bool) error
	Add(opaque uint32, quiet bool) error
	Replace(opaque uint32, quiet bool) error
	Append(opaque uint32, quiet bool) error
	Prepend(opaque uint32, quiet bool) error
	Get(response common.GetResponse) error
	GetEnd(opaque uint32, noopEnd bool) error
	GetE(response common.GetEResponse) error
	GAT(response common.GetResponse) error
	Delete(opaque uint32) error
	Touch(opaque uint32) error
	Noop(opaque uint32) error
	Quit(opaque uint32, quiet bool) error
	Version(opaque uint32) error
	Error(opaque uint32, reqType common.RequestType, err error, quiet bool) error
}

type Peeker interface {
	Peek(n int) ([]byte, error)
}

type Disambiguator interface {
	CanParse() (bool, error)
}

type Components interface {
	NewDisambiguator(p Peeker) Disambiguator
	NewRequestParser(r *bufio.Reader) RequestParser
	NewResponder(w *bufio.Writer) Responder
}
