package parallel

import (
	"runtime"
	"sync"
	"testing"
)

func TestRowsAlignedCoversEveryRowOnceInAlignedBands(t *testing.T) {
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(8))
	for _, height := range []int{0, 1, 2, 3, 31, 32, 33, 1000, 1001} {
		for _, align := range []int{1, 2, 4} {
			var mu sync.Mutex
			seen := make([]int, height)
			RowsAligned(height, align, func(lo, hi int) {
				if lo%align != 0 {
					t.Errorf("height %d align %d: band starts at %d", height, align, lo)
				}
				if hi != height && hi%align != 0 {
					t.Errorf("height %d align %d: band ends at %d", height, align, hi)
				}
				mu.Lock()
				for y := lo; y < hi; y++ {
					seen[y]++
				}
				mu.Unlock()
			})
			for y, n := range seen {
				if n != 1 {
					t.Fatalf("height %d align %d: row %d visited %d times", height, align, y, n)
				}
			}
		}
	}
}
