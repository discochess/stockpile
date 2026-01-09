// Package search implements binary search within sorted shard data.
package search

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

// ErrNotFound indicates the position was not found in the shard.
var ErrNotFound = errors.New("position not found")

// EvalRecord represents a single evaluation record in the shard data.
// Matches the Lichess evaluation database format.
type EvalRecord struct {
	FEN   string `json:"fen"`
	Evals []struct {
		PVs []struct {
			CP   *int   `json:"cp,omitempty"`
			Mate *int   `json:"mate,omitempty"`
			Line string `json:"line"`
		} `json:"pvs"`
		Knodes int `json:"knodes"`
		Depth  int `json:"depth"`
	} `json:"evals"`
}

// Search searches for a FEN in sorted JSONL shard data.
// Returns the evaluation record if found, or ErrNotFound.
func Search(data []byte, targetFEN string) (*EvalRecord, error) {
	lines := splitLines(data)
	if len(lines) == 0 {
		return nil, ErrNotFound
	}

	// Binary search for the target FEN.
	idx := sort.Search(len(lines), func(i int) bool {
		fen := extractFEN(lines[i])
		return fen >= targetFEN
	})

	if idx >= len(lines) {
		return nil, ErrNotFound
	}

	// Verify exact match.
	fen := extractFEN(lines[idx])
	if fen != targetFEN {
		return nil, ErrNotFound
	}

	// Parse the full record.
	var record EvalRecord
	if err := json.Unmarshal(lines[idx], &record); err != nil {
		return nil, fmt.Errorf("parsing eval record: %w", err)
	}

	return &record, nil
}

// splitLines splits data into lines, excluding empty lines.
func splitLines(data []byte) [][]byte {
	// Pre-allocate capacity by estimating line count.
	n := bytes.Count(data, []byte{'\n'}) + 1
	lines := make([][]byte, 0, n)
	for len(data) > 0 {
		idx := bytes.IndexByte(data, '\n')
		var line []byte
		if idx < 0 {
			line = data
			data = nil
		} else {
			line = data[:idx]
			data = data[idx+1:]
		}
		if len(line) > 0 {
			lines = append(lines, line)
		}
	}
	return lines
}

// extractFEN extracts the FEN field from a JSON line without full parsing.
// This is optimized for binary search where we only need to compare FENs.
func extractFEN(line []byte) string {
	// Fast path: look for "fen":" pattern.
	const prefix = `"fen":"`
	idx := bytes.Index(line, []byte(prefix))
	if idx < 0 {
		return ""
	}

	start := idx + len(prefix)
	end := bytes.IndexByte(line[start:], '"')
	if end < 0 {
		return ""
	}

	return string(line[start : start+end])
}

