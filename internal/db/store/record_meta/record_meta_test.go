package record_meta

import (
	"io"
	"os"
	"testing"

	"github.com/DenzelPenzel/nyx/internal/db/store/record_header"
	"github.com/stretchr/testify/require"
)

// TestEncodeDecodeRecordMeta verifies that the Encode and Decode functions work as expected.
// Note that the expiration encoding is lossy: the decoded expiration is computed as:
//
//	((expire >> 9) << 9) + 511   (if expire != 0)
func TestEncodeDecodeRecordMeta(t *testing.T) {
	addr := uint32(123456)
	size := byte(12)

	// Test with zero expiration
	expireZero := uint32(0)
	meta := Encode(addr, size, expireZero)
	decAddr, decSize, decExpire := Decode(meta)
	require.Equal(t, addr, decAddr)
	require.Equal(t, size, decSize)
	require.Equal(t, expireZero, decExpire)

	// Test with a non-zero expiration
	expireVal := uint32(3600) // 3600 seconds
	meta = Encode(addr, size, expireVal)
	decAddr, decSize, decExpire = Decode(meta)
	require.Equal(t, addr, decAddr)
	require.Equal(t, size, decSize)

	// Expected expiration is computed as: (expireVal >> 9)<<9 + 511
	expectedExpire := ((expireVal >> 9) << 9) + 511
	require.Equal(t, expectedExpire, decExpire)
}

func createTempFileWithContent(t *testing.T, content []byte) (*os.File, func()) {
	tmpFile, err := os.CreateTemp("", "version_test_*.db")
	require.NoError(t, err)
	_, err = tmpFile.Write(content)
	require.NoError(t, err)

	// Reset offset to start for reading.
	_, err = tmpFile.Seek(0, io.SeekStart)
	require.NoError(t, err)
	cleanup := func() {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
	}
	return tmpFile, cleanup
}

func TestReadFileVersion(t *testing.T) {
	// File starts with [RecordVersionMarker, 0] => version 0
	content := []byte{record_header.RecordVersionMarker, 0}
	file, cleanup := createTempFileWithContent(t, content)
	defer cleanup()

	ver, err := ReadFileVersion(file)
	require.NoError(t, err)
	require.Equal(t, 0, ver)

	// File starts with [RecordVersionMarker, someVersion] => that version
	versionByte := byte(2)
	content = []byte{record_header.RecordVersionMarker, versionByte}
	file, cleanup = createTempFileWithContent(t, content)
	defer cleanup()
	ver, err = ReadFileVersion(file)
	require.NoError(t, err)
	require.Equal(t, int(versionByte), ver)

	// File with insufficient bytes
	file, cleanup = createTempFileWithContent(t, []byte{record_header.RecordVersionMarker})
	defer cleanup()
	_, err = ReadFileVersion(file)
	require.Error(t, err)

	// File does not start with version marker but second byte is 0
	content = []byte{0x00, 0x00}
	file, cleanup = createTempFileWithContent(t, content)
	defer cleanup()
	ver, err = ReadFileVersion(file)
	require.NoError(t, err)
	require.Equal(t, 0, ver)
}
