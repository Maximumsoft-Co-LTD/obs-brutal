//go:build race

package benchmarks

// raceEnabled reports whether the race detector is compiled in. The
// time budgets are sized for uninstrumented builds; race instrumentation
// slows the hot path ~10x, so the ns/op gates skip themselves under -race.
const raceEnabled = true
