package dyport

import (
	"crypto/rand"
	"math/big"
	"net"
	"sync"
)

const (
	minPort    = 10000
	endPort    = 65535
	countPorts = 1024
	maxBlocks  = 16
	attempts   = 3
)

var (
	port     int
	initPort int
	once     sync.Once
	mu       sync.Mutex
)

func AllocatePorts(count int) ([]int, error) {
	if count > countPorts-1 {
		count = countPorts - 1
	}

	mu.Lock()
	defer mu.Unlock()

	ports := make([]int, 0)

	once.Do(func() {
		for i := 0; i < attempts; i++ {
			rndBlocks, err := rand.Int(rand.Reader, big.NewInt(maxBlocks))
			if err != nil {
				continue
			}
			initPort = minPort + int(rndBlocks.Int64())*countPorts
			lockLn, err := listener(initPort)
			if err != nil {
				continue
			}
			_ = lockLn.Close()
			return
		}
		panic("failed to allocate port block")
	})

	for len(ports) < count {
		port++
		if port < initPort+1 || port >= initPort+countPorts {
			port = initPort + 1
		}
		ln, err := listener(port)
		if err != nil {
			continue
		}
		_ = ln.Close()
		ports = append(ports, port)
	}

	return ports, nil
}

func listener(port int) (*net.TCPListener, error) {
	return net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: port})
}
