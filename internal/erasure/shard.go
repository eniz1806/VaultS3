package erasure

import (
	"encoding/json"
	"time"
)

// ShardMeta holds metadata for an erasure-coded object.
// Stored alongside the shards to enable reconstruction.
type ShardMeta struct {
	OriginalSize int64     `json:"original_size"`
	DataShards   int       `json:"data_shards"`
	ParityShards int       `json:"parity_shards"`
	BlockSize    int64     `json:"block_size"`
	ShardSizes   []int64   `json:"shard_sizes"` // actual size of each shard file
	ETag         string    `json:"etag"`
	CreatedAt    time.Time `json:"created_at"`
}

func (m *ShardMeta) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

func UnmarshalShardMeta(data []byte) (*ShardMeta, error) {
	var m ShardMeta
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// shardKey returns the storage key for a specific shard index.
// Shards are stored under: {bucket}/.ec/{key}/shard-{index}
func shardKey(key string, index int) string {
	return ecPrefix(key) + shardName(index)
}

// metaKey returns the storage key for the shard metadata.
func metaKey(key string) string {
	return ecPrefix(key) + "meta.json"
}

// ecPrefix returns the erasure coding prefix for an object key.
func ecPrefix(key string) string {
	return ".ec/" + key + "/"
}

func shardName(index int) string {
	const digits = "0123456789"
	if index < 10 {
		return "shard-0" + string(digits[index])
	}
	return "shard-" + string(digits[index/10]) + string(digits[index%10])
}
