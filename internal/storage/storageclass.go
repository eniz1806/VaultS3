package storage

// StorageClass defines storage class settings.
type StorageClass struct {
	Name         string `json:"name" yaml:"name"`
	DataShards   int    `json:"data_shards" yaml:"data_shards"`
	ParityShards int    `json:"parity_shards" yaml:"parity_shards"`
}

// DefaultStorageClasses returns the built-in storage classes.
func DefaultStorageClasses() map[string]StorageClass {
	return map[string]StorageClass{
		"STANDARD": {
			Name:         "STANDARD",
			DataShards:   4,
			ParityShards: 2,
		},
		"REDUCED_REDUNDANCY": {
			Name:         "REDUCED_REDUNDANCY",
			DataShards:   4,
			ParityShards: 1,
		},
	}
}
