// Package collection contains small generic helpers shared across page-level
// view-model builders.
package collection

// Keys returns the keys of m in unspecified order.
func Keys[K comparable, V any](m map[K]V) []K {
	out := make([]K, 0, len(m))
	for key := range m {
		out = append(out, key)
	}
	return out
}
