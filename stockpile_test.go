package stockpile

import (
	"context"
	"errors"
	"testing"

	"github.com/discochess/stockpile/internal/store"
	"github.com/discochess/stockpile/internal/store/memstore"
)

func TestNew_RequiresStore(t *testing.T) {
	_, err := New()
	if !errors.Is(err, ErrNoStore) {
		t.Errorf("New() error = %v, want ErrNoStore", err)
	}
}

func TestNew_WithStore(t *testing.T) {
	mem := memstore.New()
	client, err := New(WithStore(mem))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	if client.Store() != mem {
		t.Error("Store() returned unexpected store")
	}
}

func TestClient_Lookup_NotFound(t *testing.T) {
	mem := memstore.New()
	client, err := New(WithStore(mem))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	_, err = client.Lookup(context.Background(), "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	if !errors.Is(err, store.ErrNotFound) && !errors.Is(err, ErrNotFound) {
		t.Errorf("Lookup() error = %v, want ErrNotFound", err)
	}
}

func TestClient_Lookup_Found(t *testing.T) {
	mem := memstore.New()
	// Set up a shard with test data in JSONL format.
	// The shard ID for this FEN would need to be computed, but for testing
	// we can use a simple approach: put data in shard 0 and use a custom strategy.
	testFEN := "8/8/8/8/8/8/8/8 w - - 0 1"
	testData := []byte(`{"fen":"8/8/8/8/8/8/8/8 w - - 0 1","evals":[{"pvs":[{"cp":0,"line":""}],"knodes":1,"depth":1}]}` + "\n")

	// The material shard strategy will put empty board in shard 0.
	mem.SetShard(0, testData)

	client, err := New(
		WithStore(mem),
		WithTotalShards(1), // Single shard for testing.
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	eval, err := client.Lookup(context.Background(), testFEN)
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}

	if eval.FEN != testFEN {
		t.Errorf("Eval.FEN = %q, want %q", eval.FEN, testFEN)
	}
}

func TestClient_Close(t *testing.T) {
	mem := memstore.New()
	client, err := New(WithStore(mem))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// First close should succeed.
	if err := client.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Second close should return ErrClosed.
	if err := client.Close(); !errors.Is(err, ErrClosed) {
		t.Errorf("Close() second call error = %v, want ErrClosed", err)
	}
}

func TestClient_Lookup_AfterClose(t *testing.T) {
	mem := memstore.New()
	client, err := New(WithStore(mem))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	client.Close()

	_, err = client.Lookup(context.Background(), "test")
	if !errors.Is(err, ErrClosed) {
		t.Errorf("Lookup() after close error = %v, want ErrClosed", err)
	}
}

func TestClient_ShardStrategy(t *testing.T) {
	mem := memstore.New()
	client, err := New(WithStore(mem))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	strategy := client.ShardStrategy()
	if strategy == nil {
		t.Error("ShardStrategy() returned nil")
	}
	if strategy.Name() == "" {
		t.Error("ShardStrategy().Name() returned empty string")
	}
}

func TestWithTotalShards(t *testing.T) {
	mem := memstore.New()
	client, err := New(
		WithStore(mem),
		WithTotalShards(100),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()
	// Client created successfully with custom shard count.
}
