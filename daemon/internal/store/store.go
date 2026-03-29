// Package store provides a local key-value store for hived state.
// Uses bbolt (etcd's embedded database) for persistence.
package store

import (
	"fmt"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"
)

var (
	bucketServices          = []byte("services")
	bucketContainers        = []byte("containers")
	bucketSecrets           = []byte("secrets")
	bucketMeta              = []byte("meta")
	bucketServicePlacements = []byte("service_placements")
	bucketHealthState       = []byte("health_state")
	bucketServiceHistory    = []byte("service_history")
)

// Store is a local persistent key-value store backed by bbolt.
type Store struct {
	db *bolt.DB
}

// Open creates or opens the state store in the given directory.
func Open(dataDir string) (*Store, error) {
	dbPath := filepath.Join(dataDir, "hive.db")
	db, err := bolt.Open(dbPath, 0o600, &bolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open bolt db at %s: %w", dbPath, err)
	}

	// Create buckets
	err = db.Update(func(tx *bolt.Tx) error {
		for _, bucket := range [][]byte{bucketServices, bucketContainers, bucketSecrets, bucketMeta, bucketServicePlacements, bucketHealthState, bucketServiceHistory} {
			if _, err := tx.CreateBucketIfNotExists(bucket); err != nil {
				return fmt.Errorf("create bucket %s: %w", bucket, err)
			}
		}
		return nil
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize buckets: %w", err)
	}

	return &Store{db: db}, nil
}

// DB returns the underlying bbolt database for use by other packages (e.g., auth).
func (s *Store) DB() *bolt.DB {
	return s.db
}

// Close shuts down the store.
func (s *Store) Close() error {
	return s.db.Close()
}

// Put stores a value under the given bucket and key.
func (s *Store) Put(bucket, key string, value []byte) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return fmt.Errorf("bucket %q not found", bucket)
		}
		return b.Put([]byte(key), value)
	})
}

// Get retrieves a value from the given bucket and key.
func (s *Store) Get(bucket, key string) ([]byte, error) {
	var value []byte
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return fmt.Errorf("bucket %q not found", bucket)
		}
		v := b.Get([]byte(key))
		if v != nil {
			value = make([]byte, len(v))
			copy(value, v)
		}
		return nil
	})
	return value, err
}

// Delete removes a key from the given bucket.
func (s *Store) Delete(bucket, key string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return fmt.Errorf("bucket %q not found", bucket)
		}
		return b.Delete([]byte(key))
	})
}

// ─── Placement helpers ─────────────────────────────────────────

// SetPlacement records which node owns a service.
func (s *Store) SetPlacement(serviceName, nodeName string) error {
	return s.Put("service_placements", serviceName, []byte(nodeName))
}

// GetPlacement returns the node that owns a service, or "" if not placed.
func (s *Store) GetPlacement(serviceName string) string {
	val, err := s.Get("service_placements", serviceName)
	if err != nil || val == nil {
		return ""
	}
	return string(val)
}

// DeletePlacement removes a service placement.
func (s *Store) DeletePlacement(serviceName string) error {
	return s.Delete("service_placements", serviceName)
}

// ListPlacements returns all service→node mappings in a single read transaction
// for consistency and performance (avoids N+1 reads).
func (s *Store) ListPlacements() (map[string]string, error) {
	placements := make(map[string]string)
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketServicePlacements)
		if b == nil {
			return fmt.Errorf("bucket %q not found", "service_placements")
		}
		return b.ForEach(func(k, v []byte) error {
			placements[string(k)] = string(v)
			return nil
		})
	})
	return placements, err
}

// List returns all keys in a bucket.
func (s *Store) List(bucket string) ([]string, error) {
	var keys []string
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return fmt.Errorf("bucket %q not found", bucket)
		}
		return b.ForEach(func(k, _ []byte) error {
			keys = append(keys, string(k))
			return nil
		})
	})
	return keys, err
}
