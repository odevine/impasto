package mask

import (
	"testing"

	"github.com/odevine/impasto/path"
)

func TestVectorMaskCoverage(t *testing.T) {
	p := path.New().Rect(0, 0, 5, 10)
	m := NewVectorMask(p, 10, 10, path.NonZero)
	if m.Coverage(2, 5) < 0.99 {
		t.Errorf("inside path coverage = %v want ~1", m.Coverage(2, 5))
	}
	if m.Coverage(7, 5) > 0.01 {
		t.Errorf("outside path coverage = %v want ~0", m.Coverage(7, 5))
	}
}
