package metadata

import (
	"encoding/binary"
	"fmt"
	"io"

	bolt "go.etcd.io/bbolt"
)

// WriteSnapshot writes the entire BoltDB database to w for Raft snapshots.
// Format: sequence of (bucketNameLen uint32, bucketName, numKV uint64, [(keyLen uint32, key, valLen uint32, val)]...)
func (s *Store) WriteSnapshot(w io.Writer) error {
	return s.db.View(func(tx *bolt.Tx) error {
		return tx.ForEach(func(name []byte, b *bolt.Bucket) error {
			// Write bucket name
			if err := writeBytes(w, name); err != nil {
				return fmt.Errorf("write bucket name %s: %w", name, err)
			}

			// Count keys
			var count uint64
			b.ForEach(func(k, v []byte) error {
				count++
				return nil
			})

			// Write key count
			if err := binary.Write(w, binary.BigEndian, count); err != nil {
				return fmt.Errorf("write key count: %w", err)
			}

			// Write each key-value pair
			return b.ForEach(func(k, v []byte) error {
				if err := writeBytes(w, k); err != nil {
					return err
				}
				return writeBytes(w, v)
			})
		})
	})
}

// RestoreSnapshot replaces the entire BoltDB state from a snapshot reader.
func (s *Store) RestoreSnapshot(r io.Reader) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		// Delete all existing buckets
		var existing [][]byte
		tx.ForEach(func(name []byte, _ *bolt.Bucket) error {
			existing = append(existing, append([]byte{}, name...))
			return nil
		})
		for _, name := range existing {
			if err := tx.DeleteBucket(name); err != nil {
				return fmt.Errorf("delete bucket %s: %w", name, err)
			}
		}

		// Read and restore all buckets
		for {
			name, err := readBytes(r)
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return fmt.Errorf("read bucket name: %w", err)
			}

			b, err := tx.CreateBucket(name)
			if err != nil {
				return fmt.Errorf("create bucket %s: %w", name, err)
			}

			var count uint64
			if err := binary.Read(r, binary.BigEndian, &count); err != nil {
				return fmt.Errorf("read key count: %w", err)
			}

			for i := uint64(0); i < count; i++ {
				key, err := readBytes(r)
				if err != nil {
					return fmt.Errorf("read key: %w", err)
				}
				val, err := readBytes(r)
				if err != nil {
					return fmt.Errorf("read value: %w", err)
				}
				if err := b.Put(key, val); err != nil {
					return fmt.Errorf("put key: %w", err)
				}
			}
		}
	})
}

func writeBytes(w io.Writer, data []byte) error {
	if err := binary.Write(w, binary.BigEndian, uint32(len(data))); err != nil {
		return err
	}
	_, err := w.Write(data)
	return err
}

func readBytes(r io.Reader) ([]byte, error) {
	var length uint32
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return nil, err
	}
	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, err
	}
	return data, nil
}
