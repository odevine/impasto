package canvas

import "github.com/odevine/impasto/raster"

// Place copies src into a new document-sized buffer with its top-left at (x,y).
// It is for callers who want a document-sized buffer, a layer whose content is
// smaller than the document can set Layer.Origin instead. Regions outside the
// document are clipped
func Place(docW, docH int, src *raster.Buffer, x, y int) *raster.Buffer {
	dst := raster.MustNewBuffer(docW, docH)
	PlaceInto(dst, src, x, y)
	return dst
}

// PlaceInto copies src into dst at (x,y), overwriting the destination region.
// Both buffers are premultiplied linear
func PlaceInto(dst, src *raster.Buffer, x, y int) {
	for sy := 0; sy < src.Height; sy++ {
		dy := y + sy
		if dy < 0 || dy >= dst.Height {
			continue
		}
		for sx := 0; sx < src.Width; sx++ {
			dx := x + sx
			if dx < 0 || dx >= dst.Width {
				continue
			}
			si := (sy*src.Width + sx) * 4
			di := (dy*dst.Width + dx) * 4
			copy(dst.Pix[di:di+4], src.Pix[si:si+4])
		}
	}
}
