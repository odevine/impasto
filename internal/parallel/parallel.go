// Package parallel splits row-oriented pixel work across goroutines. Every
// split partitions the output into disjoint, fixed row bands so results never
// depend on goroutine scheduling order, which is what keeps rendering
// deterministic across runs.
package parallel

import (
	"runtime"
	"sync"
)

// MinRowsPerGoroutine is the smallest band worth spawning a goroutine for.
// Below this the coordination overhead outweighs the parallelism
const MinRowsPerGoroutine = 16

// Rows runs fn over contiguous row bands covering [0,height), in parallel when
// the work is large enough. fn receives a half-open range [lo,hi) and must only
// write rows inside that range so the bands stay independent.
func Rows(height int, fn func(lo, hi int)) {
	if height <= 0 {
		return
	}
	workers := runtime.GOMAXPROCS(0)
	if workers < 1 {
		workers = 1
	}
	if workers > height/MinRowsPerGoroutine {
		workers = height / MinRowsPerGoroutine
	}
	if workers < 1 {
		workers = 1
	}
	if workers == 1 {
		fn(0, height)
		return
	}

	band := (height + workers - 1) / workers
	var wg sync.WaitGroup
	for lo := 0; lo < height; lo += band {
		hi := lo + band
		if hi > height {
			hi = height
		}
		wg.Add(1)
		go func(lo, hi int) {
			defer wg.Done()
			fn(lo, hi)
		}(lo, hi)
	}
	wg.Wait()
}
