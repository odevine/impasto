package effects

// Morphology on a coverage field. Choke tightens a shadow (erode) and spread
// fattens a glow (dilate) before the softening blur, matching Photoshop's model.
// Both are separable min/max box filters, run horizontally then vertically.

// dilate expands coverage by taking the maximum over a (2r+1) window
func dilate(cov []float32, w, h, r int) []float32 {
	if r <= 0 {
		return cov
	}
	tmp := minmaxH(cov, w, h, r, true)
	return minmaxV(tmp, w, h, r, true)
}

// erode contracts coverage by taking the minimum over a (2r+1) window
func erode(cov []float32, w, h, r int) []float32 {
	if r <= 0 {
		return cov
	}
	tmp := minmaxH(cov, w, h, r, false)
	return minmaxV(tmp, w, h, r, false)
}

// minmaxH runs a horizontal min or max box filter with zero padding at edges,
// so eroding shrinks toward transparent and dilating grows into transparent
func minmaxH(src []float32, w, h, r int, max bool) []float32 {
	dst := make([]float32, len(src))
	for y := 0; y < h; y++ {
		row := y * w
		for x := 0; x < w; x++ {
			v := edgeVal(max)
			for k := -r; k <= r; k++ {
				xx := x + k
				var s float32
				if xx >= 0 && xx < w {
					s = src[row+xx]
				}
				v = pick(v, s, max)
			}
			dst[row+x] = v
		}
	}
	return dst
}

// minmaxV runs the vertical pass
func minmaxV(src []float32, w, h, r int, max bool) []float32 {
	dst := make([]float32, len(src))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			v := edgeVal(max)
			for k := -r; k <= r; k++ {
				yy := y + k
				var s float32
				if yy >= 0 && yy < h {
					s = src[yy*w+x]
				}
				v = pick(v, s, max)
			}
			dst[y*w+x] = v
		}
	}
	return dst
}

func edgeVal(max bool) float32 {
	if max {
		return 0
	}
	return 1
}

func pick(a, b float32, max bool) float32 {
	if max {
		if b > a {
			return b
		}
		return a
	}
	if b < a {
		return b
	}
	return a
}
