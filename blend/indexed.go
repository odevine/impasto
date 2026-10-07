package blend

import (
	"image"

	"github.com/odevine/impasto/internal/parallel"
	"github.com/odevine/impasto/raster"
)

// minGap is how many transparent pixels in a row end one run of visible pixels
// and start the next. A shorter gap costs less to blend through than to skip,
// and merging across it keeps the run list short for an image with fine detail
const minGap = 32

// Indexed is an *image.NRGBA with a record of where its visible pixels are, so
// compositing it skips whole stretches of transparent pixels without reading
// them. It embeds the image and is one, so it can be given anywhere an
// *image.NRGBA is, and a program that decodes an image once and composites it
// many times builds the index once and keeps it with the image.
//
// The record belongs to the image as it was when Index read it. Writing to the
// pixels afterward leaves the record wrong
type Indexed struct {
	*image.NRGBA
	// rows[y] is where row y's runs start in runs, and rows[y+1] where they end.
	// A run is a half-open range of columns, counted from the image's own left edge
	rows []int32
	runs []int32
}

// Index reads img once and returns it with the runs of columns in each row that
// have any alpha. Runs that sit within a short distance of one another are
// merged. The cost is one pass over the alpha bytes
func Index(img *image.NRGBA) *Indexed {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	ix := &Indexed{NRGBA: img, rows: make([]int32, h+1), runs: make([]int32, 0, 2*h)}
	for y := 0; y < h; y++ {
		ix.rows[y] = int32(len(ix.runs) / 2)
		row := img.Pix[img.PixOffset(b.Min.X, b.Min.Y+y):]
		row = row[:w*4]
		start, last := -1, -1
		for x := 0; x < w; x++ {
			if row[x*4+3] == 0 {
				continue
			}
			if start >= 0 && x-last-1 >= minGap {
				ix.runs = append(ix.runs, int32(start), int32(last+1))
				start = -1
			}
			if start < 0 {
				start = x
			}
			last = x
		}
		if start >= 0 {
			ix.runs = append(ix.runs, int32(start), int32(last+1))
		}
	}
	ix.rows[h] = int32(len(ix.runs) / 2)
	return ix
}

// CompositeIndexed is CompositeNRGBA for an image whose visible pixels have been
// indexed. It blends only the runs the index lists, which gives the same result
// as blending every pixel, since a pixel with no alpha leaves the backdrop alone
// in every mode, and costs time in proportion to the visible area
func CompositeIndexed(dst *raster.Buffer, src *Indexed, origin image.Point, m Mode, opacity float32) {
	if opacity <= 0 {
		return
	}
	if opacity > 1 {
		opacity = 1
	}
	sb := src.Bounds()
	r := image.Rect(origin.X, origin.Y, origin.X+sb.Dx(), origin.Y+sb.Dy()).
		Intersect(image.Rect(0, 0, dst.Width, dst.Height))
	if r.Empty() {
		return
	}
	// The part of each run that lands inside dst, counted from the image's left edge
	from, to := int32(r.Min.X-origin.X), int32(r.Max.X-origin.X)
	parallel.Rows(r.Dy(), func(lo, hi int) {
		for y := r.Min.Y + lo; y < r.Min.Y+hi; y++ {
			sy := y - origin.Y
			rowStart := src.PixOffset(sb.Min.X, sb.Min.Y+sy)
			for k := src.rows[sy]; k < src.rows[sy+1]; k++ {
				x0, x1 := max(src.runs[2*k], from), min(src.runs[2*k+1], to)
				if x0 >= x1 {
					continue
				}
				di := (y*dst.Width + origin.X + int(x0)) * 4
				si := rowStart + int(x0)*4
				n := int(x1-x0) * 4
				if m == Normal {
					normalRow(dst.Pix[di:di+n], src.Pix[si:si+n], opacity)
				} else {
					modeRow(dst.Pix[di:di+n], src.Pix[si:si+n], m, opacity)
				}
			}
		}
	})
}
