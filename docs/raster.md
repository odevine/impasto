# raster

`raster` defines the pixel representation every other package works in, and
holds the conversions to and from `image.Image`. Reading this guide first makes
the rest of the library easier to follow, since every buffer in it follows the
rules set out here.

```go
import "github.com/odevine/impasto/raster"
```

## The color model

Three decisions that hold everywhere in the library:

**Linear light.** Source pixels are decoded from sRGB to linear on the way in
and re-encoded on the way out. Everything in between is linear. This is what
makes blending, blurring, and gradients physically sensible rather than merely
plausible: averaging two colors in sRGB space gives a result that is too dark,
which is why blurring in sRGB space tends to look muddy.

The practical consequence is that channel values will not match what a color
picker told you. Mid-gray `#808080` is `0.2159` in linear light, not `0.5`:

```go
r, _, _, _ := buf.At(0, 0)
// sRGB 128 -> linear 0.2159
```

**Premultiplied alpha.** Color channels are stored already scaled by alpha. A
half-transparent pure red is `{0.5, 0, 0, 0.5}`, not `{1, 0, 0, 0.5}`. This
avoids fringing: with straight alpha, a fully transparent pixel still carries a
color that bleeds into its neighbors under any filter that averages pixels, and
a blurred logo picks up a halo of whatever color the transparent regions
happened to hold. Premultiplied, transparent means `{0,0,0,0}` and averaging is
well behaved.

It also makes masking trivial. Scaling all four channels by the same coverage
value attenuates color and alpha together and leaves the pixel validly
premultiplied, which is exactly what `mask.Apply` does.

**float32.** Enough headroom that intermediate results do not quantize. An
effect chain that blurs, colorizes, blends, and blurs again accumulates rounding
error at every step; at 8 bits per channel that shows up as banding. Quantization
happens once, at the end, in `ToImage`.

## Buffer

```go
type Buffer struct {
    Pix    []float32 // row-major, interleaved RGBA, len == Width*Height*4
    Width  int
    Height int
}
```

`Pix` is exported so tight pixel loops can index it directly
rather than paying for a method call per channel, and every package in the
library does exactly that. `At` and `Set` exist for the cases where clarity
matters more than speed, and for the useful property that out-of-range
coordinates are handled rather than fatal:

```go
r, g, b, a := buf.At(99999, 99999) // 0, 0, 0, 0, no panic
```

That makes sampling loops that read past an edge well defined, which several
effects rely on.

### Allocating

```go
buf, err := raster.NewBuffer(w, h)   // returns an error on bad dimensions
buf := raster.MustNewBuffer(w, h)    // panics, for trusted sizes
```

Use `NewBuffer` for any size derived from input you did not produce. Image
headers are a classic denial-of-service vector: a file can declare itself
2,000,000 by 2,000,000 and the allocation is what kills you, not the decode.
`NewBuffer` validates *before* allocating and rejects three separate failure
modes:

```go
raster.NewBuffer(0, 100)
// raster: invalid buffer dimensions: 0x100 must be positive

raster.NewBuffer(raster.MaxDimension+1, 1)
// raster: invalid buffer dimensions: 65537x1 exceeds max edge 65536
```

The third is integer overflow in the `w*h*4` element count, checked in int64
before `make` ever sees it. All three wrap `ErrDimensions`, so
`errors.Is(err, raster.ErrDimensions)` identifies the class.

`MaxDimension` is `1<<16`, or 65536 per edge. That is far larger than anything
you would composite in practice and far smaller than anything that overflows.

### Working with buffers

```go
clone := buf.Clone()        // deep copy, shares no storage
same := buf.SameSize(other) // precondition for most pixelwise ops
buf.Clear()                 // reset to transparent black
w, h := buf.Bounds()
```

`Clone` matters more than it looks. The library's immutability guarantee (a
layer can be re-composited with different settings without its content
changing) is implemented by cloning at the top of `canvas.renderLayer`. If you
build your own pipeline, clone before you mutate.

## Getting images in

```go
buf, err := raster.FromImage(img)   // any image.Image
buf, err := raster.FromFile("in.png") // PNG, JPEG, and GIF decoders registered
```

`FromImage` has fast paths for the concrete types the standard decoders
actually produce: `*image.NRGBA`, `*image.NRGBA64`, `*image.Gray`, and
`*image.Gray16`. Those read the backing slice directly and convert through a
lookup table.

Everything else, including `*image.YCbCr` from the JPEG decoder,
`*image.Paletted` from GIF, and `*image.RGBA`, falls back to a generic path that
converts through `color.NRGBA64Model` one pixel at a time. It is correct but
slower. If you are decoding JPEGs in a hot loop and the profile points here,
converting to `*image.NRGBA` once is the fix.

The sRGB decode uses precomputed tables: 256 entries for 8-bit, 65536 for
16-bit. The 16-bit table costs 256 KiB of package-level memory and buys a
branch-free, `pow`-free ingest path.

## Getting images out

```go
img := buf.ToImage(8)   // *image.NRGBA, ordered dithering
img := buf.ToImage(16)  // *image.NRGBA64, no dithering
```

Any depth other than 16 is treated as 8.

Egress reverses ingest: unpremultiply, encode linear back to sRGB, quantize.
Values are clamped to `[0,1]` at this point and not before, so an effect that
transiently pushes a channel above 1 does not get clipped mid-pipeline.

### Why the 8-bit path dithers

Quantizing a smooth gradient to 256 levels produces visible banding: wide flat
stripes where a range of float values collapse onto the same integer. Ordered
dithering breaks the bands into noise the eye integrates away, which reads as
smoother even though it is strictly less accurate per pixel.

The dither is an 8x8 Bayer matrix offsetting each value by up to half a code
before rounding. It is a pure function of `(x, y)`, which is what keeps output
deterministic. It is applied only at quantization, never to the working buffer,
so the float32 data stays clean for any further processing.

16-bit output skips it. At 65536 levels the step size is already well below the
visible threshold, and the noise would be the only thing you gained.

## Color conversion

```go
raster.SRGBToLinear(c float32) float32
raster.LinearToSRGB(c float32) float32
```

The IEC 61966-2-1 transfer functions, exported for building color constants by
hand. Both operate on a single channel in `[0,1]`.

**Alpha never goes through these.** Alpha is a coverage fraction, not a
perceptual quantity; running it through a gamma curve is a bug that shows up as
incorrectly-weighted edges. The library never does it and neither should you.

## Gotchas

- **`Pix` is premultiplied.** Writing `{1, 0, 0, 0.5}` for a half-transparent
  red produces a pixel brighter than its own alpha allows, and the result of
  compositing it is undefined-looking rather than obviously wrong. Write
  `{0.5, 0, 0, 0.5}`.
- **`Pix` is linear.** `0.5` is not mid-gray, it is around sRGB 188. Use
  `SRGBToLinear` or one of the `FlatColorSRGB`-style constructors.
- **Buffers are not goroutine-safe for writes.** Independent buffers are fully
  independent, and everything the library parallelizes partitions by disjoint
  row bands. Two goroutines writing overlapping rows of the same buffer is
  something you would need to coordinate yourself.
- **`FromImage` gives you the source image's dimensions**, not your document's.
  Use [`canvas.Place`](canvas.md#placing-content) to position it.
