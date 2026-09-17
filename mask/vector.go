package mask

import "github.com/odevine/impasto/path"

// VectorMask is a path rasterized to coverage once and then used exactly like a
// raster mask. Rasterizing eagerly means a reused layer never re-rasterizes the
// same path on every composite.
type VectorMask struct {
	*RasterMask
}

// NewVectorMask rasterizes p into a coverage mask of the given size using the
// fill rule and path's default flattening tolerance
func NewVectorMask(p *path.Path, w, h int, rule path.FillRule) *VectorMask {
	cov := p.Coverage(w, h, rule, path.DefaultTolerance)
	return &VectorMask{RasterMask: NewRasterMask(cov, w, h)}
}
