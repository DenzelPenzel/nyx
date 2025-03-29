package store

import (
	"errors"
	"slices"
	"strings"

	"github.com/DenzelPenzel/nyx/internal/common"
	"github.com/google/btree"
)

type StringItem string

func (a StringItem) Less(b btree.Item) bool {
	return a < b.(StringItem)
}

var GlobalBucketsKey = []byte("[all_bucket_keys]")

type Bucket struct {
	Name  string
	Index *btree.BTree
}

// AddKey adds a key to the bucket's index
func (bkt *Bucket) AddKey(key []byte) {
	bkt.Index.ReplaceOrInsert(StringItem(key))
}

// Bucket returns (or creates) a bucket with the given name.
func (ds *DataStore) Bucket(name string) (*Bucket, error) {
	val, err := ds.Get(GlobalBucketsKey)
	if errors.Is(err, common.ErrKeyNotFound) {
		err = nil
	}
	if err != nil {
		return nil, err
	}
	keys := strings.Split(string(val), ",")

	if slices.Index(keys, name) == -1 {
		keys = append(keys, name)
		if err := ds.Set(GlobalBucketsKey, []byte(strings.Join(keys, ",")), 0); err != nil {
			return nil, err
		}
	}

	return &Bucket{Name: name, Index: ds.btree}, nil
}

// BucketPut stores a key/value pair under the given bucket.
func (ds *DataStore) BucketPut(bucket *Bucket, key, value []byte) error {
	compositeKey := append([]byte(bucket.Name), key...)
	if err := ds.Set(compositeKey, value, 0); err != nil {
		return err
	}
	// Update the bucket index
	bucket.AddKey(compositeKey)
	return nil
}
