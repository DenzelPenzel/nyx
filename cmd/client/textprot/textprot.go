package textprot

import (
	"bufio"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
)

const (
	maxTTL = 3600
)

type TextProt struct{}

func (t TextProt) Set(rw *bufio.ReadWriter, key []byte, value []byte) error {
	strKey := string(key)

	if _, err := fmt.Fprintf(rw, "set %s 0 0 %v\r\n%s\r\n", strKey, len(value), string(value)); err != nil {
		return err
	}

	rw.Flush()

	_, err := rw.ReadString('\n')
	if err != nil {
		return err
	}

	return nil
}

func (t TextProt) Add(rw *bufio.ReadWriter, key []byte, value []byte) error {
	strKey := string(key)

	if _, err := fmt.Fprintf(rw, "add %s 0 0 %v\r\n%s\r\n", strKey, len(value), string(value)); err != nil {
		return err
	}

	rw.Flush()

	_, err := rw.ReadString('\n')
	if err != nil {
		return err
	}

	return nil
}

func (t TextProt) Replace(rw *bufio.ReadWriter, key []byte, value []byte) error {
	strKey := string(key)

	if _, err := fmt.Fprintf(rw, "replace %s 0 0 %v\r\n%s\r\n", strKey, len(value), string(value)); err != nil {
		return err
	}

	rw.Flush()

	_, err := rw.ReadString('\n')
	if err != nil {
		return err
	}

	return nil
}

func (t TextProt) Append(rw *bufio.ReadWriter, key []byte, value []byte) error {
	if _, err := fmt.Fprintf(rw, "append %s 0 0 %v\r\n%s\r\n", string(key), len(value), string(value)); err != nil {
		return err
	}

	rw.Flush()

	_, err := rw.ReadString('\n')
	return err
}

func (t TextProt) Prepend(rw *bufio.ReadWriter, key []byte, value []byte) error {
	if _, err := fmt.Fprintf(rw, "prepend %s 0 0 %v\r\n%s\r\n", string(key), len(value), string(value)); err != nil {
		return err
	}

	rw.Flush()

	_, err := rw.ReadString('\n')
	return err
}

func (t TextProt) GetWithOpaque(rw *bufio.ReadWriter, key []byte, _ int) ([]byte, error) {
	return t.Get(rw, key)
}

func (t TextProt) Get(rw *bufio.ReadWriter, key []byte) ([]byte, error) {
	strKey := string(key)

	if _, err := fmt.Fprintf(rw, "get %s\r\n", strKey); err != nil {
		return nil, err
	}

	rw.Flush()

	// read the header line
	header, err := rw.ReadString('\n')
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(header) == "END" {
		return []byte{}, nil
	}

	// then read the value
	value, err := rw.ReadString('\n')
	if err != nil {
		return nil, err
	}

	value = strings.TrimSpace(value)

	// then read the END
	if _, err := rw.ReadString('\n'); err != nil {
		return nil, err
	}
	return []byte(value), nil
}

func (t TextProt) BatchGet(rw *bufio.ReadWriter, keys [][]byte) ([][]byte, error) {
	cmd := []byte("get")
	space := byte(' ')
	end := []byte("\r\n")

	for _, key := range keys {
		cmd = append(cmd, space)
		cmd = append(cmd, key...)
	}

	cmd = append(cmd, end...)

	if _, err := fmt.Fprint(rw, string(cmd)); err != nil {
		return nil, err
	}

	rw.Flush()

	var ret [][]byte

	for {
		// read the header line
		response, err := rw.ReadString('\n')
		if err != nil {
			return nil, err
		}

		if strings.TrimSpace(response) == "END" {
			return ret, nil
		}

		// then read the value
		response, err = rw.ReadString('\n')
		if err != nil {
			return nil, err
		}

		ret = append(ret, []byte(response))
	}
}

func (t TextProt) GAT(_ *bufio.ReadWriter, _ []byte) ([]byte, error) {
	panic("implement it")
}

func (t TextProt) Delete(rw *bufio.ReadWriter, key []byte) error {
	strKey := string(key)

	if _, err := fmt.Fprintf(rw, "delete %s\r\n", strKey); err != nil {
		return err
	}

	rw.Flush()

	_, err := rw.ReadString('\n')
	if err != nil {
		return err
	}

	return nil
}

func (t TextProt) Touch(rw *bufio.ReadWriter, key []byte) error {
	strKey := string(key)

	ttl, err := rand.Int(rand.Reader, big.NewInt(int64(maxTTL)))
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintf(rw, "touch %s %v\r\n", strKey, ttl); err != nil {
		return err
	}

	rw.Flush()

	_, err = rw.ReadString('\n')
	if err != nil {
		return err
	}

	return nil
}
