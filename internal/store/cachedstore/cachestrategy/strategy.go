// Package cachestrategy defines cache eviction strategy interfaces.
package cachestrategy

// Strategy defines the interface for cache eviction strategies.
type Strategy interface {
	Get(key int) ([]byte, bool)
	Add(key int, value []byte) bool
	Len() int
}
