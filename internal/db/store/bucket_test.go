package store

import (
	"bytes"
	"os"
	"testing"

	"github.com/google/btree"
	"github.com/stretchr/testify/require"
)

var bucketDirName = "-db-test-bucket--tmp-"

// Test_SingleBucketStore verifies that keys added via BucketPut
// are stored with a composite key (bucket name + key) and retrievable
func Test_SingleBucketStore(t *testing.T) {
	require.NoError(t, os.RemoveAll(bucketDirName))
	defer os.RemoveAll(bucketDirName)

	ds, err := NewDataStore(WithDirectory(bucketDirName), WithShardCollisionCount(0), WithTotalShards(1))
	require.NoError(t, err)
	defer ds.Close()

	bucketName := "users"
	bucket, err := ds.Bucket(bucketName)
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

	for _, tc := range testCases {
		if tc.execute {
			err := ds.BucketPut(bucket, tc.key, tc.val)
			require.NoError(t, err)
		} else {
			v, err := ds.Get(tc.key)
			require.NoError(t, err)
			require.True(t, bytes.Equal([]byte(tc.expected), v))
		}
	}

	var foundKeys []string
	bucket.Index.Ascend(func(item btree.Item) bool {
		foundKeys = append(foundKeys, string(item.(StringItem)))
		return true
	})

	expectedKeys := []string{bucketName + "001", bucketName + "002", bucketName + "003"}
	require.ElementsMatch(t, expectedKeys, foundKeys)
}

func Test_MultiBucketStore(t *testing.T) {
	require.NoError(t, os.RemoveAll(bucketDirName))
	defer os.RemoveAll(bucketDirName)

	ds, err := NewDataStore(WithDirectory(bucketDirName), WithShardCollisionCount(0), WithTotalShards(1))
	require.NoError(t, err)
	defer ds.Close()

	// Create the first bucket and add a key/value pair
	bucketName1 := "user_group1"
	b1, err := ds.Bucket(bucketName1)
	require.NoError(t, err)
	err = ds.BucketPut(b1, []byte("001"), []byte("elon"))
	require.NoError(t, err)

	// Create a second bucket and add a different key/value pair
	bucketName2 := "user_group2"
	b2, err := ds.Bucket(bucketName2)
	require.NoError(t, err)
	err = ds.BucketPut(b2, []byte("001"), []byte("alex"))
	require.NoError(t, err)

	// Verify that retrieving using the composite keys returns the correct values.
	v, err := ds.Get([]byte(bucketName1 + "001"))
	require.NoError(t, err)
	require.Equal(t, "elon", string(v))

	v, err = ds.Get([]byte(bucketName2 + "001"))
	require.NoError(t, err)
	require.Equal(t, "alex", string(v))

	// Add the new key
	err = ds.BucketPut(b1, []byte("002"), []byte("sam"))
	require.NoError(t, err)
	v, err = ds.Get([]byte(bucketName1 + "002"))
	require.NoError(t, err)
	require.Equal(t, "sam", string(v))
}
