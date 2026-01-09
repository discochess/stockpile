# Stockpile

Fast lookups for hundreds of millions of chess position evaluations from the Lichess database.

```go
eval, err := client.Lookup(ctx, "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq -")
fmt.Println(eval.Score()) // +0.20
```

## Why?

Running Stockfish analysis is expensive. Analyzing a single game at depth 20+ takes several seconds of CPU time. [Disco Chess](https://www.discochess.com) is a chess training platform built around the [Woodpecker Method](https://www.discochess.com/about/woodpecker-method) — solving the same puzzles repeatedly until pattern recognition becomes automatic. Its review queue resurfaces mistakes from both training cycles and users' actual games, which means analyzing thousands of imported games to find missed tactics. That's a lot of Stockfish:

- **Scaling headaches**: Spin up worker pools, manage job queues, handle bursty workloads ([sound familiar?](https://gist.github.com/discochess/9ca536f709aed0cad071a81c25a8332e))
- **Slow feedback**: Users wait minutes for game analysis to complete
- **High costs**: CPU-intensive workloads don't come cheap

Stockpile sidesteps most of this. Lichess has already [analyzed hundreds of millions of positions](https://database.lichess.org/#evals) with Stockfish at depth 30+. Why redo work they've already done? Look up what exists, run Stockfish only for the gaps.

## How?

1. **Shard by material** — Positions are distributed across 32K shards based on piece counts. Positions with similar material land in the same shard.

2. **Sort by FEN** — Within each shard, positions are sorted lexicographically. Lookups use binary search.

3. **Cache hot shards** — An LRU cache keeps frequently accessed shards in memory. Game analysis hits the same few shards repeatedly.

The key insight: consecutive positions in a chess game almost always have the same material (captures are rare). Material-based sharding keeps them together, maximizing cache hits.

## Features

- **Fast lookups** with LRU caching
- **Hundreds of millions of positions** from Lichess Stockfish evaluations (depth 30+)
- **Pluggable storage**: Local filesystem, GCS, S3
- **Material-based sharding** for cache locality during game analysis
- **Zero external dependencies** at runtime (all data self-contained)

## Quick Start

### Installation

```bash
go get github.com/discochess/stockpile
```

### Build the Database

Download and process the Lichess evaluation database:

```bash
# Install CLI
go install github.com/discochess/stockpile/cmd/stockpile@latest

# Build from Lichess source (downloads ~17GB)
stockpile build --output ./data

# Or from a local file
stockpile build --source ./lichess_db_eval.jsonl.zst --output ./data
```

**Build options:**

| Flag | Default | Description |
|------|---------|-------------|
| `--output` | `./data` | Output directory for shards (local) |
| `--output-gcs` | | GCS path for output (`gs://bucket/prefix`) |
| `--shards` | `32768` | Number of shards to create |
| `--strategy` | `material` | Sharding strategy: `material`, `fnv32` |
| `--workers` | `4` | Parallel workers for compression |
| `--max-memory` | `1024` | Max memory (MB) before spilling to disk |

**Memory note:** The build process can be memory-intensive. If you experience OOM kills, lower `--max-memory` (e.g., `--max-memory 512`). For long builds, use `caffeinate` on macOS:

```bash
caffeinate -dims stockpile build --source ./lichess_db_eval.jsonl.zst --output ./data --workers 10
```

**GCS output:** For cloud deployments, build directly to GCS:

```bash
stockpile build --output-gcs gs://my-bucket/stockpile
```

This builds locally to a temp directory, then uploads to GCS. Suitable for monthly cronjobs to pick up new positions from Lichess.

### Use the Library

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/discochess/stockpile"
)

func main() {
    // Create client with default settings (LRU cache, zstd decompression).
    opt, _ := stockpile.WithDataDir("./data")
    client, _ := stockpile.New(opt)
    defer client.Close()

    // Look up a position.
    ctx := context.Background()
    eval, err := client.Lookup(ctx, "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq -")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Score: %s\n", eval.Score())  // +0.20
    fmt.Printf("Depth: %d\n", eval.Depth)    // 36
    if pv := eval.BestPV(); pv != nil {
        fmt.Printf("PV: %s\n", pv.Line)      // e2e4 e7e5 g1f3
    }
}
```

For advanced configuration (custom cache size, cloud storage), see [examples/](examples/).

### CLI Usage

```bash
# Look up a position
stockpile lookup "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq -"

# Show database stats
stockpile stats --data-dir ./data

# Verify database integrity
stockpile verify --data-dir ./data
```

## Architecture

### Design Principles

Inspired by [SSTables](https://research.google/pubs/bigtable-a-distributed-storage-system-for-structured-data/): sorted immutable files with binary search.

1. **Simplicity over cleverness** — Standard formats (JSONL, zstd) over custom binary formats
2. **Pluggable components** — Interfaces for storage, cache, and sharding strategy
3. **Zero runtime dependencies** — All data self-contained, no external services required

### Data Flow

```
Build Phase:
  Lichess DB (.zst) → Decompress → Shard by Material → Sort by FEN → Compress

Lookup Phase:
  FEN → Compute Shard ID → Check Cache → Decompress (if miss) → Binary Search
```

### Shard File Format

Each shard is a zstd-compressed JSONL file with lines sorted by FEN:

```
shards/
├── 00000.zst
├── 00001.zst
├── ...
└── 32767.zst
```

Each line matches the Lichess format:

```json
{"fen":"rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq -","evals":[{"pvs":[{"cp":20,"line":"e7e5"}],"knodes":3000,"depth":36}]}
```

### Material-Based Sharding

The default strategy encodes piece counts into a shard ID:

| Piece Type | Bits | Range |
|------------|------|-------|
| White/Black Queens | 3 each | 0-7 |
| White/Black Rooks | 3 each | 0-7 |
| White/Black Minors (B+N) | 3 each | 0-7 |
| Side to move | 1 | 0-1 |

Total: 19 bits → modulo 32,768 shards.

This clusters positions with similar material together. Games progress through predictable material phases, so consecutive positions land in the same shard.

### Binary Search

Lookup within a shard:

1. Decompress (zstd)
2. Binary search by FEN
3. Parse matching JSON line

FEN extraction during search avoids full JSON parsing—just a string search for `"fen":"`.

### Thread Safety

The `Client` is safe for concurrent use. Stats use atomic operations. Cache and store implementations handle concurrent access.

### Sentinel Errors

```go
var (
    ErrNotFound = errors.New("stockpile: position not found")
    ErrClosed   = errors.New("stockpile: client closed")
)
```

## Benchmarking

Compare sharding strategies with real game data:

```bash
# Install benchmark CLI
go install github.com/discochess/stockpile/cmd/stockpile-bench@latest

# Run simulation with PGN games
stockpile-bench run --games games.pgn --strategies material,fnv32

# Generate markdown report
stockpile-bench run --games games.pgn --format markdown --output report.md --verbose
```

## Storage Backends

### Local Filesystem (default)

```go
store, _ := diskstore.New("./data", zstdcodec.New())
```

### Google Cloud Storage

```go
store, _ := gcsstore.New(ctx, "my-bucket", gcsstore.WithPrefix("stockpile/"))
```

### AWS S3

```go
store, _ := s3store.New(ctx, "my-bucket", s3store.WithPrefix("stockpile/"))
```

## Performance

Performance depends on storage backend, cache size, and access patterns. Warm cache lookups (shard already in memory) are fast. Cold lookups require decompression. Cloud storage adds network latency.

Run benchmarks on your hardware with `stockpile-bench` to measure actual performance.

## Data Source

Evaluations come from the [Lichess evaluation database](https://database.lichess.org/#evals):
- Hundreds of millions of unique positions (and growing)
- Stockfish 16+ at depth 30+
- Updated monthly

## Project Structure

```
stockpile/
├── cmd/
│   ├── stockpile/              # Main CLI (build, lookup, stats, verify)
│   └── stockpile-bench/        # Benchmark CLI
├── internal/
│   ├── builder/                # Database build pipeline
│   ├── codec/                  # Compression codecs (zstd, gzip, noop)
│   ├── search/                 # Binary search on sorted JSONL
│   ├── shard/                  # Sharding strategies
│   │   ├── materialshard/      # Material-based (default)
│   │   └── fnvshard/           # FNV32 hash
│   ├── stats/                  # Metrics collection
│   └── store/                  # Storage backends
│       ├── diskstore/          # Local filesystem
│       ├── gcsstore/           # Google Cloud Storage
│       ├── s3store/            # AWS S3
│       └── cachedstore/        # LRU caching wrapper
├── benchmark/                  # Benchmarking infrastructure
├── examples/                   # Example applications
└── fx/                         # Uber fx modules for DI
```

## Fx Modules

For applications using [Uber Fx](https://github.com/uber-go/fx):

```go
import "github.com/discochess/stockpile/fx/diskstockpilefx"

fx.New(
    diskstockpilefx.Module,
    // ... your modules
)
```

## License

MIT License - see [LICENSE](LICENSE) for details.

## Contributing

Contributions welcome! Please read the Architecture section above first.
