package canvas

import (
	"flag"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/odevine/impasto/effects"
	"github.com/odevine/impasto/gradient"
	"github.com/odevine/impasto/mask"
	"github.com/odevine/impasto/path"
	"github.com/odevine/impasto/raster"
)

var update = flag.Bool("update", false, "regenerate golden images")

// goldenScene builds a fixed document that exercises the whole pipeline:
// a gradient background, a masked shape, a drop shadow, and a stroke
func goldenScene() *Document {
	const w, h = 128, 128

	bgBuf := raster.MustNewBuffer(w, h)
	gradient.New(gradient.Linear, gradient.Pad,
		path.Point{X: 0, Y: 0}, path.Point{X: w, Y: h},
		[]gradient.Stop{
			{Pos: 0, Color: color.NRGBA{30, 60, 120, 255}, Opacity: 1},
			{Pos: 1, Color: color.NRGBA{200, 220, 255, 255}, Opacity: 1},
		}).Render(bgBuf)

	shape := path.New().Ellipse(64, 64, 34, 34)
	return &Document{
		Width: w, Height: h,
		Root: Group{
			PassThrough: true, Opacity: 1,
			Layers: []Node{
				&Layer{Content: bgBuf, Opacity: 1},
				&Layer{
					Content: fill(w, h, 0.95, 0.85, 0.2, 1),
					Opacity: 1,
					Mask:    mask.NewVectorMask(shape, w, h, path.NonZero),
					Effects: []effects.Effect{
						&effects.DropShadow{Color: color.Black, Opacity: 0.6, Angle: 2.36, Distance: 8, BlurRadius: 8},
						&effects.Stroke{Width: 4, Color: color.NRGBA{120, 40, 40, 255}, Alignment: effects.StrokeOutside, Opacity: 1},
					},
				},
			},
		},
	}
}

// TestGolden renders the fixed scene and compares it byte-for-byte within a tight
// per-channel tolerance against the committed reference. Run with -update to
// regenerate the reference after an intentional change
func TestGolden(t *testing.T) {
	out := MustRender(goldenScene())
	got := out.ToImage(8).(*image.NRGBA)
	goldenPath := filepath.Join("testdata", "golden.png")

	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		f, err := os.Create(goldenPath)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, got); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", goldenPath)
		return
	}

	f, err := os.Open(goldenPath)
	if err != nil {
		t.Fatalf("missing golden, run with -update: %v", err)
	}
	defer f.Close()
	ref, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}

	b := got.Bounds()
	if ref.Bounds() != b {
		t.Fatalf("size mismatch: got %v want %v", b, ref.Bounds())
	}
	const tol = 2
	maxDiff := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			gr, gg, gb, ga := got.At(x, y).RGBA()
			rr, rg, rb, ra := ref.At(x, y).RGBA()
			for _, d := range []int{delta(gr, rr), delta(gg, rg), delta(gb, rb), delta(ga, ra)} {
				if d > maxDiff {
					maxDiff = d
				}
				if d > tol {
					t.Fatalf("pixel (%d,%d) differs by %d units (tol %d)", x, y, d, tol)
				}
			}
		}
	}
	t.Logf("golden match, max per-channel delta %d", maxDiff)
}

// delta compares two 16-bit color samples in 8-bit units
func delta(a, b uint32) int {
	d := int(a>>8) - int(b>>8)
	if d < 0 {
		return -d
	}
	return d
}
