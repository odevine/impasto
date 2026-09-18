# gradient

`gradient` generates five gradient geometries with multi-stop color and
independent per-stop opacity. A `Gradient` satisfies `path.Paint`, so it can
fill a shape directly, and it can fill a whole buffer as a color field for
overlay effects.

```go
import "github.com/odevine/impasto/gradient"
```

## Building one

```go
g := gradient.New(kind Kind, spread Spread, p0, p1 path.Point, stops []Stop) *Gradient
```

A `Gradient` is immutable once built. `New` sorts the stops (stably, so equal
positions keep their listed order) and precomputes the geometry, which is what
makes per-pixel sampling cheap enough to call from a fill loop.

```go
g := gradient.New(gradient.Linear, gradient.Pad,
    path.Point{X: 0, Y: 0}, path.Point{X: 100, Y: 0},
    []gradient.Stop{
        {Pos: 0, Color: color.NRGBA{R: 255, A: 255}, Opacity: 1},
        {Pos: 1, Color: color.NRGBA{B: 255, A: 255}, Opacity: 1},
    })
```

## Stops

```go
type Stop struct {
    Pos     float32     // normalized position along the gradient
    Color   color.Color // sRGB, alpha ignored
    Opacity float32     // [0,1], authoritative
}
```

**The stop color's own alpha channel is ignored.** Opacity is a separate,
authoritative field, matching how Photoshop separates the color ramp from the
opacity ramp. Passing `color.NRGBA{R: 255, A: 0}` and expecting transparency
gives opaque red at whatever `Opacity` says, and a zero-value `Opacity`
means invisible, so it is worth setting explicitly every time.

Separating them is what lets a color fade and an opacity fade have different
stop positions, which is the whole point of the Photoshop model.

```go
// A fade to transparent
{Pos: 0, Color: color.White, Opacity: 1},
{Pos: 1, Color: color.White, Opacity: 0},
```

## Interpolation

Stops interpolate in **sRGB**, then convert to linear for compositing.

This is a deliberate exception to the library's linear-light rule, and it is
there to match Photoshop. Interpolating in linear light would be more accurate
physically, but it produces visibly different ramps: a black-to-white linear
interpolation looks washed out and top-heavy compared to what every design tool
shows. Since impasto exists to reproduce Photoshop output, it interpolates where
Photoshop does.

The consequence to remember is that the midpoint of a black-to-white gradient is
sRGB 128, which is linear `0.216`, not linear `0.5`.

Interpolation happens at float32 precision, so gradients carry no banding of
their own. Dithering is applied once, at
[final quantization](raster.md#why-the-8-bit-path-dithers).

## The five geometries

`p0` and `p1` mean different things per kind.

| Kind        | p0     | p1                             | Produces                                    |
| ----------- | ------ | ------------------------------ | ------------------------------------------- |
| `Linear`    | start  | end                            | Color along the axis                        |
| `Radial`    | center | a point on the radius          | Concentric rings                            |
| `Angle`     | center | sets the start angle           | A conic sweep                               |
| `Reflected` | center | end of one arm                 | A linear gradient mirrored about p0         |
| `Diamond`   | center | an edge midpoint of the square | Square contours, a Chebyshev-distance field |

`Angle` sweeps clockwise on screen (y grows downward), starting from
the direction of `p1 - p0`.

`Diamond` rotates its contours to the axis, so a diagonal `p1` gives a diamond
rotated 45 degrees.

Degenerate geometry (`p0 == p1`) samples at parameter 0 everywhere rather than
dividing by zero, so it yields a flat fill of the first stop.

## Spread

What happens outside the `[0,1]` parameter range.

| Spread    | Behavior                          |
| --------- | --------------------------------- |
| `Pad`     | Clamps to the end stops (default) |
| `Repeat`  | Tiles the gradient                |
| `Reflect` | Mirrors on each repeat            |

For `Linear`, `Pad` is the only one most callers want. `Repeat` and `Reflect`
turn a short axis into a stripe pattern, which is occasionally what you are
after and usually a bug.

`Angle` is inherently periodic and always wraps regardless of spread.

## Sampling

### As a paint

```go
path.Fill(dst, shape, path.NonZero, g)
```

`ColorAt(x, y) [4]float32` returns premultiplied linear color, satisfying
`path.Paint`. No intermediate buffer.

### As a field

```go
g.Render(dst *raster.Buffer)
```

Overwrites the whole buffer, ignoring its prior contents. This is a fill, not a
composite: masking and blending are the caller's job. It is what
`effects.GradientOverlay` uses internally.

### Pixel centers

Sampling happens at pixel centers, so pixel 0 of a 100-wide gradient is sampled
at `x = 0.5` and lands a half-pixel short of the first stop:

```go
g.ColorAt(0, 0)   // r=0.99, not 1.00
g.ColorAt(99, 0)  // b=0.99, not 1.00
```

This is correct, and it is the same convention the rasterizer uses, but it
surprises people writing exact-value tests. The end colors are approached, not
reached. If you need the exact endpoint color, extend the gradient geometry a
half-pixel past the region you are filling.

## Gotchas

- **Set `Opacity` explicitly.** The zero value is fully transparent, and the
  stop color's alpha is ignored.
- **Interpolation is sRGB**, unlike the rest of the library. See
  [Interpolation](#interpolation).
- **`Render` overwrites.** It does not composite.
- **Sampling is at pixel centers.** Endpoints are approached, not reached.
- **Stops are sorted for you**, so you may pass them in any order, but equal
  positions resolve in the order listed.
