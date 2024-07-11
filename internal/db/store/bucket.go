package store

import (
	"errors"
	"slices"
	"strings"

	"github.com/DenzelPenzel/nyx/internal/common"
	"github.com/google/btree"
)

// Str implements the Item interface for strings.
type Str string

// Less returns true if a < b.
func (a Str) Less(b btree.Item) bool {
	return a < b.(Str)
}

var (
	GlobalBucketKeysStore = []byte("[all_bucket_keys]")
)

type BucketStore struct {
	Name  string
	Btree *btree.BTree
}

func (bkt *BucketStore) Put(key []byte) {
	bkt.Btree.ReplaceOrInsert(Str(key))
}

// Bucket ... Create a new bucket in the memory index
func (s *Store) Bucket(name string) (*BucketStore, error) {
	val, err := s.Get(GlobalBucketKeysStore)
	if errors.Is(err, common.ErrKeyNotFound) {
		err = nil
	}
	if err != nil {
		return nil, err
	}
	keys := strings.Split(string(val), ",")
	idx := slices.Index(keys, name)

	if idx == -1 {
		keys = append(keys, name)
		err := s.Set(GlobalBucketKeysStore, []byte(strings.Join(keys, ",")), 0)
		if err != nil {
			return nil, err
		}
	}

	return &BucketStore{Name: name, Btree: s.btree}, nil
}

func (s *Store) Put(bucket *BucketStore, k, val []byte) error {
	key := []byte(bucket.Name)
	key = append(key, k...)
	err := s.Set(key, val, 0)
	if err != nil {
		return err
	}
	// put key in index
	bucket.Put(key)
	return nil
}
