package textprot

import (
	"bufio"
	"errors"
	"fmt"

	"github.com/DenzelPenzel/nyx/internal/common"
)

type ResponderText struct {
	writer *bufio.Writer
}

func NewTextResponder(writer *bufio.Writer) ResponderText {
	return ResponderText{
		writer: writer,
	}
}

func (t ResponderText) Set(_ uint32, _ bool) error {
	return t.resp("STORED")
}

func (t ResponderText) Add(_ uint32, _ bool) error {
	return t.resp("STORED")
}

func (t ResponderText) Replace(_ uint32, _ bool) error {
	return t.resp("STORED")
}

func (t ResponderText) Append(_ uint32, _ bool) error {
	return t.resp("STORED")
}

func (t ResponderText) Prepend(_ uint32, _ bool) error {
	return t.resp("STORED")
}

func (t ResponderText) Get(response common.GetResponse) error {
	if response.Miss {
		// A miss is a no-op in the textprot world
		return nil
	}

	// Write data out to client
	// [VALUE <key> <flags> <bytes>\r\n
	// <data block>\r\n]*
	// END\r\n
	_, err := fmt.Fprintf(t.writer, "VALUE %s %d %d\r\n", response.Key, response.Flags, len(response.Data))
	if err != nil {
		return err
	}

	_, err = t.writer.Write(response.Data)
	if err != nil {
		return err
	}

	_, err = t.writer.WriteString("\r\n")
	if err != nil {
		return err
	}

	t.writer.Flush()
	return nil
}

func (t ResponderText) GetEnd(_ uint32, _ bool) error {
	return t.resp("END")
}

func (t ResponderText) GetE(_ common.GetEResponse) error {
	panic("GetE command in textprot protocol")
}

func (t ResponderText) GAT(_ common.GetResponse) error {
	// There's two options here.
	// 1) panic() because this is never supposed to be called
	// 2) Respond as a normal get
	//
	// I chose to panic, since this means we are in a bad state.
	// The textprot parser will never return a GAT command because
	// it does not exist in the textprot protocol.
	panic("GAT command in textprot protocol")
}

func (t ResponderText) Delete(_ uint32) error {
	return t.resp("DELETED")
}

func (t ResponderText) Touch(_ uint32) error {
	return t.resp("TOUCHED")
}

func (t ResponderText) Noop(_ uint32) error {
	return t.resp("Yep, it works.")
}

func (t ResponderText) Quit(_ uint32, quiet bool) error {
	if !quiet {
		return t.resp("Bye")
	}
	return nil
}

func (t ResponderText) Version(_ uint32) error {
	return t.resp("VERSION " + common.VersionString)
}

func (t ResponderText) Error(_ uint32, _ common.RequestType, err error, _ bool) error {
	switch {
	case errors.Is(err, common.ErrKeyNotFound):
		return t.resp("NOT_FOUND")
	case errors.Is(err, common.ErrKeyExists):
		return t.resp("NOT_STORED")
	case errors.Is(err, common.ErrItemNotStored):
		return t.resp("NOT_STORED")
	case errors.Is(err, common.ErrValueTooBig):
		fallthrough
	case errors.Is(err, common.ErrInvalidArgs):
		return t.resp("CLIENT_ERROR bad command line")
	case errors.Is(err, common.ErrBadIncDecValue):
		return t.resp("CLIENT_ERROR invalid numeric delta argument")
	case errors.Is(err, common.ErrAuth):
		return t.resp("CLIENT_ERROR")
	case errors.Is(err, common.ErrUnknownCmd):
		fallthrough
	case errors.Is(err, common.ErrNoMem):
		fallthrough
	default:
		return t.resp(err.Error())
	}
}

func (t ResponderText) resp(s string) error {
	_, err := fmt.Fprintf(t.writer, s+"\r\n")
	if err != nil {
		return err
	}

	return t.writer.Flush()
}
