package network

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func Test_Network(t *testing.T) {
	t.Run("test network open close", func(t *testing.T) {
		nt := NewNetwork()
		err := nt.Open("127.0.0.1:7777")
		require.NoError(t, err)
		require.Equal(t, "127.0.0.1:7777", nt.Addr().String())
		err = nt.Close()
		require.NoError(t, err)
	})

	t.Run("test network dial", func(t *testing.T) {
		nt1 := NewNetwork()
		err := nt1.Open("127.0.0.1:7777")
		require.NoError(t, err)
		go func() {
			_, err := nt1.Accept()
			if err != nil {
				return
			}
		}()

		nt2 := NewNetwork()
		_, err = nt2.Dial(nt1.Addr().String(), time.Second)
		require.NoError(t, err)
		nt1.Close()
	})
}
