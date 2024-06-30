package record_header

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewMarshalUnmarshalRecord(t *testing.T) {
	key := []byte("myKey")
	value := []byte("myValue")
	expire := uint32(0) // no expiration

	header, recordBytes := Marshal(key, value, expire)
	require.NotNil(t, header)
	require.GreaterOrEqual(t, len(recordBytes), int(1<<header.SizeByte))

	parsedHeader, parsedKey, parsedValue := Unmarshal(recordBytes)
	require.Equal(t, header.SizeByte, parsedHeader.SizeByte)
	require.Equal(t, header.Status, parsedHeader.Status)
	require.Equal(t, header.KeyLength, parsedHeader.KeyLength)
	require.Equal(t, header.ValLength, parsedHeader.ValLength)
	require.Equal(t, header.Expire, parsedHeader.Expire)
	require.Equal(t, key, parsedKey)
	require.Equal(t, value, parsedValue)
}
