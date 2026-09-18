# path

`path` builds bezier paths and rasterizes them with anti-aliased scanline
coverage, for both fill and stroke. Stroking is the part not covered by
`golang.org/x/image/vector`, which fills paths only.

```go
import "github.com/odevine/impasto/path"
```

## Building a path

The builders chain, each returning the path:

```go
p := path.New().
    MoveTo(10, 10).
    LineTo(90, 10).
    QuadTo(100, 50, 90, 90).
    CubicTo(60, 100, 30, 100, 10, 90).
    Close()
```

| Method                              | Adds                               |
| ----------------------------------- | ---------------------------------- |
| `MoveTo(x, y)`                      | Starts a new subpath               |
| `LineTo(x, y)`                      | A straight segment                 |
| `QuadTo(cx, cy, x, y)`              | A quadratic bezier                 |
| `CubicTo(c1x, c1y, c2x, c2y, x, y)` | A cubic bezier                     |
| `Close()`                           | Connects back to the subpath start |
| `Rect(x0, y0, x1, y1)`              | A closed rectangle                 |
| `Ellipse(cx, cy, rx, ry)`           | A closed ellipse, four cubics      |

A path holds any number of subpaths; each `MoveTo` starts another. Calling a
segment builder before any `MoveTo` implies one, so a path never starts in an
undefined state.

Curves are stored exactly and only flattened at rasterization time, so the same
path renders cleanly at any scale.

```go
minX, minY, maxX, maxY := p.Bounds()
empty := p.Empty()
```

`Bounds` covers on-curve *and* control points. Control points can sit well
outside the curve they describe, so this is a conservative outer bound, not a
tight one. It is correct for allocation and culling, and wrong for anything that
needs the true extent.

## Fill rules

```go
path.NonZero  // fill where the winding number is nonzero
path.EvenOdd  // fill where the crossing count is odd
```

They differ only where a path overlaps itself. The standard demonstration is a
five-pointed star drawn as a single self-crossing subpath: NonZero gives a solid
center, EvenOdd hollows out the middle pentagon.

```go
nonZero := star.Coverage(100, 100, path.NonZero, path.DefaultTolerance)
evenOdd := star.Coverage(100, 100, path.EvenOdd, path.DefaultTolerance)
// center: nonzero 1, evenodd 0
```

NonZero is almost always what you want, and it is what `Stroke` output requires.

## Filling

```go
path.Fill(dst *raster.Buffer, p *Path, rule FillRule, paint Paint)
path.FillTol(dst, p, rule, paint, tol float32)
path.FillColor(dst, p, rule, paint FlatColor)
```

The path is in the destination's coordinate space with the top-left at `(0,0)`.
Fill composites the paint through per-pixel coverage using source-over.

`FillTol` takes an explicit flattening tolerance. `DefaultTolerance` is `0.1`
pixels, meaning a flattened segment never deviates from the true curve by more
than a tenth of a pixel. Lower it for very large renders where curves start to
look faceted; raise it if flattening shows up in a profile. It is a distance in
pixels, so the right value scales with your output size.

## Paint

```go
type Paint interface {
    ColorAt(x, y int) [4]float32
}
```

Returns premultiplied linear color. Two implementations ship:

```go
c := path.NewFlatColor(r, g, b, a)      // straight linear components, premultiplied for you
c := path.FlatColorSRGB(color.Color)    // decodes sRGB to linear
```

`FlatColorSRGB` is what you want when the color came from a designer, a config
file, or a `color.NRGBA` literal. `NewFlatColor` is for values already in linear
light.

`gradient.Gradient` also satisfies `Paint`, so a gradient can fill a shape
directly with no intermediate buffer:

```go
path.Fill(dst, circle, path.NonZero, myGradient)
```

The interface lives here rather than in `gradient` so that `path` does not have
to depend on `gradient`.

## Coverage and anti-aliasing

```go
cov := p.Coverage(w, h int, rule FillRule, tol float32) []float32
```

Returns a `w*h` coverage field, which is also how `mask.NewVectorMask` works.

The rasterizer uses the signed-area / running-sum method: each edge deposits
partial area contributions into an accumulator, and a left-to-right prefix sum
per row turns those into exact analytic pixel coverage. This is analytic rather
than sampled, so a shape covering 30% of a pixel gets exactly 0.30, not a
supersampled approximation of it.

Coverage is `float32` throughout. Nothing downstream inherits 8-bit banding from
the rasterizer; quantization happens once, at
[egress](raster.md#getting-images-out).

Geometry outside the raster is handled rather than clipped away: x is clamped
into `[0,w]`, which is exactly correct since anything to the left of the
viewport contributes full coverage to column 0. The accumulator carries two
guard columns so an edge landing on the right border cannot bleed into the next
row.

## Stroking

```go
outline := path.Stroke(p, style)                    // returns a fillable Path
path.StrokePath(dst, p, style, paint)               // stroke and paint in one call
```

`Stroke` converts a path into an outline you then fill with **NonZero**. The
outline is built from many convex pieces, a quad per segment plus a shape per
join and per cap, all wound the same way, so nonzero winding unions them cleanly.
That is why the rule matters: EvenOdd would carve holes wherever the pieces
overlap.

```go
type StrokeStyle struct {
    Width      float32
    Cap        Cap
    Join       Join
    MiterLimit float32
    Dash       []float32
    DashOffset float32
}
```

The zero value is a hairline butt-capped miter-joined stroke, so set only what
you care about. A `Width` of zero or less produces an empty path.

### Caps

How open ends terminate. Closed subpaths have no caps.

| Cap         | Shape                              |
| ----------- | ---------------------------------- |
| `CapButt`   | Flush with the endpoint (default)  |
| `CapRound`  | A semicircle past the end          |
| `CapSquare` | A flat extension of half the width |

Round and square caps extend the outline by half the stroke width past each
endpoint, which `Bounds` reflects:

```go
line := path.New().MoveTo(10, 50).LineTo(90, 50)
outline := path.Stroke(line, path.StrokeStyle{Width: 10, Cap: path.CapRound})
// bounds: (5,45)-(95,55)
```

### Joins

How consecutive segments meet at a vertex.

| Join        | Shape                                                           |
| ----------- | --------------------------------------------------------------- |
| `JoinMiter` | Extended to a sharp point, subject to the miter limit (default) |
| `JoinRound` | An arc                                                          |
| `JoinBevel` | A flat chamfer                                                  |

`MiterLimit` caps how far a miter may extend before falling back to a bevel. As
the angle between segments gets sharper the miter point shoots off toward
infinity, so without a limit a near-doubled-back path grows a spike. The default
is `DefaultMiterLimit`, which is `4`, matching SVG and PostScript. A zero or
negative `MiterLimit` uses the default.

### Dashes

```go
Dash:       []float32{12, 6}  // 12 on, 6 off
DashOffset: 3                 // start 3 units into the pattern
```

The pattern alternates on and off, cycling. `DashOffset` shifts the start, which
is how you animate a marching-ants effect without rebuilding the path.

Dashing a closed subpath turns it into a sequence of open runs, so each dash gets
its own caps.

## Gotchas

- **Fill stroke outlines with NonZero.** EvenOdd punches holes where the
  outline's pieces overlap.
- **`Bounds` is conservative.** It includes control points, which can lie
  outside the curve.
- **Tolerance is in pixels.** A value tuned for a 500px render is too coarse at
  5000px.
- **Coordinates are document space**, origin top-left, with the destination
  buffer's `(0,0)` as the reference.
- **Paint colors are linear.** Use `FlatColorSRGB` for anything that came from an
  sRGB source.
