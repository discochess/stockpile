package builder

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Manifest contains metadata about a built stockpile database.
type Manifest struct {
	Version      int       `json:"version"`
	TotalShards  int       `json:"total_shards"`
	Strategy     string    `json:"strategy"`
	RecordCount  int64     `json:"record_count"`
	ShardCount   int       `json:"shard_count"` // Non-empty shards
	BuiltAt      time.Time `json:"built_at"`
	SourceURL    string    `json:"source_url,omitempty"`
	Compression  string    `json:"compression"`
}

const manifestFilename = "manifest.json"

// WriteManifest writes the manifest to the output directory.
func WriteManifest(dir string, m *Manifest) error {
	path := filepath.Join(dir, manifestFilename)
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling manifest: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing manifest: %w", err)
	}
	return nil
}

// ReadManifest reads the manifest from a data directory.
func ReadManifest(dir string) (*Manifest, error) {
	path := filepath.Join(dir, manifestFilename)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}
	return &m, nil
}
