package textprot

import (
	"bufio"
	"github.com/denzelpenzel/nyx/internal/proto"
)

// Components ... Holder for all the different protocol components in the textprot package
var Components proto.Components = comps{}

type comps struct{}

func (c comps) NewRequestParser(r *bufio.Reader) proto.RequestParser {
	return NewTextParser(r)
}

func (c comps) NewResponder(w *bufio.Writer) proto.Responder {
	return NewTextResponder(w)
}

func (c comps) NewDisambiguator(p proto.Peeker) proto.Disambiguator {
	return disam{p}
}

type disam struct {
	p proto.Peeker
}

func (d disam) CanParse() (bool, error) {
	headerByte, err := d.p.Peek(1)
	if err != nil {
		return false, err
	}
	return headerByte[0] >= 'a' && headerByte[0] <= 'z', nil
}
