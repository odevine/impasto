# mask

`mask` answers one question per pixel: how much of this layer shows through? The
answer is a `float32` in `[0,1]`. Everything else in the package is a different
way of producing that number.

```go
import "github.com/odevine/impasto/mask"
```

## The interface

```go
type Mask interface {
    Coverage(x, y int) float32
}
```

Coordinates outside a mask's own extent return `0`. A mask only ever reveals,
never invents: a layer masked by something smaller than the document is hidden
everywhere the mask does not reach.

## Applying a mask

```go
mask.Apply(dst *raster.Buffer, m Mask)
```

In place, parallel, and safe to call with a nil mask (it returns immediately).
The buffer's top-left is the mask's origin.

The implementation is one line of arithmetic, and it shows what the
[premultiplied representation](raster.md#the-color-model) buys:

```go
dst.Pix[i]   *= c
dst.Pix[i+1] *= c
dst.Pix[i+2] *= c
dst.Pix[i+3] *= c
```

Scaling all four channels by the same coverage attenuates color and alpha
together and leaves the pixel validly premultiplied. With straight alpha you
would scale alpha only, and then have to reason about whether the color channels
still mean anything.

## The four kinds

### RasterMask

A stored coverage buffer.

```go
m := mask.NewRasterMask(cov []float32, w, h int) // len(cov) must be w*h
m := mask.NewRasterMaskFromImage(img image.Image)
```

`NewRasterMask` panics on a length mismatch, which is always a caller bug.

`NewRasterMaskFromImage` converts a grayscale image to coverage using Rec. 601
luma weights (`0.299R + 0.587G + 0.114B`), then scales by the pixel's own alpha
so transparent regions of the mask image reveal nothing.

Note that coverage is stored as an alpha-like quantity and is **never gamma
decoded**. A mask is not a color. If you paint a mask in an image editor and the
midtones come out differently than you expect, this is why: an sRGB 50% gray
gives 0.5 coverage here, not 0.216.

### VectorMask

A path rasterized to coverage, once, up front.

```go
circle := path.New().Ellipse(100, 100, 80, 80)
m := mask.NewVectorMask(circle, docW, docH, path.NonZero)
```

`VectorMask` embeds `*RasterMask`, so after construction it *is* a raster mask
and costs the same to sample. The rasterization is eager: a layer
that gets re-composited does not re-rasterize its geometry every time.

Coverage is anti-aliased at float32 precision from
[`path`](path.md#coverage-and-anti-aliasing), so mask edges are as clean as
filled edges.

### FuncMask

Coverage computed rather than stored.

```go
fade := mask.FuncMask(func(x, y int) float32 {
    return float32(x) / float32(docW-1)
})
```

Useful for anything cheaper to compute than to store: linear fades, procedural
patterns, simple geometry. It is called once per pixel per apply, so keep the
body cheap; for anything expensive, render it to a `RasterMask` once instead.

### Multi

Several masks combined by multiplying their coverage.

```go
m := mask.Multi{vectorMask, rasterMask, fadeMask}
```

This is the defined behavior when a layer carries more than one mask. It
short-circuits on the first zero. An empty `Multi` is fully opaque, so it is
safe as a default.

## What is not here

**Clip-to-below** lives in [`canvas`](canvas.md#clipping) rather than in this
package. It looks like a mask, but it is a compositing-order rule: it takes its
coverage from whatever layer happens to be beneath it in the stack, which is
information a mask has no access to. Modeling it as a `Mask` would mean threading
the layer stack into this package, so instead `canvas.Layer` carries a
`ClipToBelow` flag.

## Gotchas

- **Masks are document-sized in practice.** `Apply` maps the buffer's origin to
  the mask's, so a mask smaller than the buffer hides everything past its edge.
- **Coverage is not color.** No gamma conversion happens, in either direction.
- **`NewRasterMask` panics** if `len(cov) != w*h`.
- **`FuncMask` runs per pixel per apply.** Cache expensive coverage in a
  `RasterMask`.
