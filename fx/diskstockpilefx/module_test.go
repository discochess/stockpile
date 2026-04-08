package diskstockpilefx

import (
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"

	"github.com/discochess/stockpile"
	"github.com/discochess/stockpile/internal/store/cachedstore"
)

func TestCacheSize_Zero_NoLRU(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "shards"), 0755)

	var client *stockpile.Client
	app := fxtest.New(t,
		Module,
		fx.Provide(func() *zap.Logger { return zap.NewNop() }),
		fx.Supply(Config{DataDir: dir, CacheSize: 0}),
		fx.Populate(&client),
	)
	app.RequireStart()
	t.Cleanup(func() { app.RequireStop() })

	if _, ok := client.Store().(*cachedstore.Store); ok {
		t.Error("CacheSize 0: expected no in-memory cache, but got cachedstore.Store")
	}
}

func TestCacheSize_Positive_HasLRU(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "shards"), 0755)

	var client *stockpile.Client
	app := fxtest.New(t,
		Module,
		fx.Provide(func() *zap.Logger { return zap.NewNop() }),
		fx.Supply(Config{DataDir: dir, CacheSize: 10}),
		fx.Populate(&client),
	)
	app.RequireStart()
	t.Cleanup(func() { app.RequireStop() })

	if _, ok := client.Store().(*cachedstore.Store); !ok {
		t.Error("CacheSize 10: expected cachedstore.Store, but got something else")
	}
}
