package utils

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net"
	"os"
	"runtime"
	"strings"
)

const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz1234567890-_"

// NextPowerOf2 ... Return next power of 2 for v
func NextPowerOf2(v uint32) (byte, uint32) {
	if v == 0 {
		return 0, 1
	}
	v--
	v |= v >> 1
	v |= v >> 2
	v |= v >> 4
	v |= v >> 8
	v |= v >> 16
	v++
	power := byte(0)
	for tmp := v; tmp > 1; tmp >>= 1 {
		power++
	}
	return power, v
}

func GetTCPAddr(addr string) (*net.TCPAddr, error) {
	res, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func TempDir(name string) string {
	path, err := os.MkdirTemp("", name)
	if err != nil {
		panic("failed to create temp dir")
	}
	return path
}

func RandData(length int64) []byte {
	res := make([]byte, length)
	alphalen := big.NewInt(int64(len(alphabet)))
	for i := 0; i < int(length); i++ {
		c, _ := rand.Int(rand.Reader, alphalen)
		res[i] = alphabet[int(c.Int64())]
	}
	return res
}

func GenKeys(totKeys int) [][]byte {
	keySize := 10
	tmp := make([]string, 0)
	for len(tmp) < totKeys {
		r := RandData(int64(keySize))
		tmp = append(tmp, string(r))
	}
	keys := make([][]byte, 0, totKeys)
	for _, v := range tmp {
		keys = append(keys, []byte(v))
	}
	return keys
}

func IdentifyPanic() string {
	var (
		name, file string
		line       int
		pc         [16]uintptr
	)
	// Capture the program counters for up to 16 stack frames, skipping 3 frames to get to the caller
	n := runtime.Callers(3, pc[:])
	for _, pc := range pc[:n] {
		fn := runtime.FuncForPC(pc)
		if fn == nil {
			continue
		}
		file, line = fn.FileLine(pc)
		name = fn.Name()
		if !strings.HasPrefix(name, "runtime.") {
			break
		}
	}

	return fmt.Sprintf("Panic occurred at: %v:%v (line %v)", file, name, line)
}

func Connect(h *net.TCPAddr) (net.Conn, error) {
	conn, err := net.Dial("tcp", h.String())
	if err != nil {
		return nil, err
	}
	return conn, nil
}
