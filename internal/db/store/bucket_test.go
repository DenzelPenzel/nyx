package store

import (
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

var bucketDirName = "-db-test-bucket--tmp-"

func Test_SingleBucketStore(t *testing.T) {
	err := os.RemoveAll(bucketDirName)
	defer os.RemoveAll(bucketDirName)
	require.NoError(t, err)

	s, err := Open(Dir(bucketDirName), ShardsCollision(0), ShardsTotal(1))
	require.NoError(t, err)

	bucketName := "users"
	b, err := s.Bucket(bucketName)
	require.NoError(t, err)

	testCases := []struct {
		key      []byte
		val      []byte
		expected string
		execute  bool
	}{
		{
			key:     []byte("001"),
			val:     []byte("elon"),
			execute: true,
		},
		{
			key:     []byte("002"),
			val:     []byte("xi"),
			execute: true,
		},
		{
			key:     []byte("003"),
			val:     []byte("frank"),
			execute: true,
		},
		{
			key:      []byte(bucketName + "001"),
			expected: `elon`,
			execute:  false,
		},
		{
			key:      []byte(bucketName + "002"),
			expected: `xi`,
			execute:  false,
		},
		{
			key:      []byte(bucketName + "003"),
			expected: `frank`,
			execute:  false,
		},
	}

	for _, ts := range testCases {
		if ts.execute {
			err := s.Put(b, ts.key, ts.val)
			require.NoError(t, err)
		} else {
			v, err := s.Get(ts.key)
			require.NoError(t, err)
			require.Equal(t, ts.expected, string(v))
		}
	}
}

func Test_MultiBucketStore(t *testing.T) {
	err := os.RemoveAll(bucketDirName)
	defer os.RemoveAll(bucketDirName)
	require.NoError(t, err)

	s, err := Open(Dir(bucketDirName), ShardsCollision(0), ShardsTotal(1))
	require.NoError(t, err)

	bucketName1 := "user_group1"
	b1, err := s.Bucket(bucketName1)
	require.NoError(t, err)

	err = s.Put(b1, []byte("001"), []byte("elon"))
	require.NoError(t, err)

	bucketName2 := "user_group2"
	b2, err := s.Bucket(bucketName2)
	require.NoError(t, err)

	err = s.Put(b2, []byte("001"), []byte("alex"))
	require.NoError(t, err)

	v, err := s.Get([]byte(bucketName1 + "001"))
	require.NoError(t, err)
	require.Equal(t, "elon", string(v))

	v, err = s.Get([]byte(bucketName2 + "001"))
	require.NoError(t, err)
	require.Equal(t, "alex", string(v))
}
