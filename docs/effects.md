# effects

`effects` implements the eight Photoshop layer styles as declarative data. An
effect is a parameter struct rather than an imperative pixel call, which makes
it composable, serializable, and testable on its own.

```go
import "github.com/odevine/impasto/effects"
```

## The model

```go
type Effect interface {
    Render(layer *raster.Buffer) []Rendered
}

type Rendered struct {
    Pixels  *raster.Buffer
    Behind  bool        // composite under the layer, or over it
    Mode    blend.Mode
    Opacity float32
}
```

`Render` receives the layer's premultiplied content, already masked, and returns
contributions rather than drawing anything. The caller decides where they go.
This is the whole reason effects are declarative: the renderer keeps control of
ordering and compositing, and an effect cannot reach outside its own buffer.

Most effects return one contribution. `BevelEmboss` returns two, a highlight and
a shadow, because they need different blend modes.

Effects derive everything from the **layer's alpha channel**. A drop shadow does
not know what shape produced it; it reads a coverage field. That is why every
effect works on arbitrary raster content, including photographs and rasterized
text, not just shapes that came from a path.

## Stacking order

Effects composite in Photoshop's fixed order regardless of the order you list
them:

```
0  DropShadow       (behind)
1  OuterGlow        (behind)
2  ColorOverlay
3  GradientOverlay
4  Stroke
5  InnerGlow
6  InnerShadow
7  BevelEmboss
```

```go
sorted := effects.Sort(list)  // returns a new slice, input untouched
```

`canvas` calls `Sort` for you. The point is that a given set of effects always
produces the same image, so effect order never becomes an invisible piece of
state you have to get right. Shadows and outer glow render behind the layer;
everything else renders in front.

The sort is stable, and unrecognized effect types sort last, so a custom `Effect`
implementation lands on top.

## Shared conventions

**Zero opacity means opaque.** Every effect treats a zero `Opacity` as fully
opaque rather than invisible, on the grounds that an effect you added but did
not configure should be visible.

**Zero mode means the effect's default**, not `Normal`. `DropShadow` defaults to
`Multiply`, glows to `Screen`, overlays and stroke to `Normal`. A consequence
worth knowing: you cannot explicitly request `Normal` for a shadow, because the
zero value is indistinguishable from unset. This is a deliberate trade for a
sensible zero value.

**Colors are `color.Color`**, decoded from sRGB to linear internally. Pass
`color.NRGBA` or any standard library color.

**Distances and radii are pixels.** Angles are radians.

## The eight effects

### DropShadow

```go
&effects.DropShadow{
    Color:      color.Black,
    Opacity:    0.75,
    Angle:      math.Pi / 4,  // radians, clockwise from +x
    Distance:   12,
    BlurRadius: 18,
    Choke:      0,
    Mode:       blend.Multiply,  // the default
}
```

Offsets the layer's alpha, blurs it, colorizes it, and places it behind the
layer. `Choke` erodes the alpha *before* blurring, which tightens the shadow's
edge and concentrates it. Renders behind. Defaults to `Multiply`, which is
physically right: a shadow attenuates what is behind it.

`Angle` is measured clockwise from the positive x axis in screen coordinates, so
`math.Pi/4` casts down and to the right.

### InnerShadow

Same parameters as `DropShadow`. Offsets the *inverse* of the alpha, blurs, and
clips back inside the layer, producing darkness along the inside edge as if the
shape were recessed. Renders over the layer. Defaults to `Multiply`.

### OuterGlow

```go
&effects.OuterGlow{
    Color:      color.White,
    Opacity:    1,
    BlurRadius: 12,
    Spread:     2,
    Mode:       blend.Screen,  // the default
}
```

A drop shadow with no offset. `Spread` dilates the alpha before blurring, the
complement of `Choke`, pushing the glow outward before it softens. Renders
behind. Defaults to `Screen`.

### InnerGlow

Same parameters. Radiates inward from the edge, clipped to the layer. Renders
over. Defaults to `Screen`.

### ColorOverlay

```go
&effects.ColorOverlay{Color: c, Opacity: 1, Mode: blend.Normal}
```

A flat color filling the layer's shape. Recolors a
layer without touching its content.

### GradientOverlay

```go
&effects.GradientOverlay{Gradient: g, Opacity: 1, Mode: blend.Normal}
```

A [gradient](gradient.md) field clipped to the layer's shape. The gradient's
coordinates are document space, not layer-relative, which is what lets one
gradient run consistently across several layers.

### Stroke

```go
&effects.Stroke{
    Width:     4,
    Color:     color.White,
    Paint:     nil,  // optional, overrides Color
    Alignment: effects.StrokeOutside,
    Opacity:   1,
}
```

Outlines the layer's shape. The band comes from the alpha edge by morphology
(dilate, erode, subtract), so it works on any content rather than requiring a
path.

| Alignment       | Band                                                  |
| --------------- | ----------------------------------------------------- |
| `StrokeOutside` | `dilate(alpha, W) - alpha`, entirely outside the edge |
| `StrokeInside`  | `alpha - erode(alpha, W)`, entirely inside            |
| `StrokeCenter`  | `dilate(alpha, W/2) - erode(alpha, W/2)`, straddling  |

Note that `StrokeCenter` at a given `Width` reaches only `W/2` in each direction,
so it covers half as much ground on each side as the other two. A width-4 center
stroke spans 2px in and 2px out; a width-4 outside stroke spans 4px out.

Set `Paint` to fill the band with a gradient or any other `path.Paint` instead of
a flat color.

Because the band is morphological, the radius rounds to whole pixels. Sub-pixel
stroke widths are not meaningful here. For a precise sub-pixel outline, stroke a
[path](path.md#stroking) directly instead.

### BevelEmboss

```go
&effects.BevelEmboss{
    Style:            effects.BevelInner,
    Depth:            2,
    Direction:        effects.DirUp,
    Size:             6,     // blur radius on the height field
    Soften:           2,     // blur radius on the lit result
    Angle:            math.Pi / 4,  // light azimuth
    Altitude:         math.Pi / 4,  // light elevation
    HighlightColor:   color.White,
    HighlightOpacity: 0.75,
    ShadowColor:      color.Black,
    ShadowOpacity:    0.75,
}
```

Treats the alpha channel as a height field, blurs it by `Size` to round the
edges, takes surface normals from a 3x3 Sobel gradient, and applies a
directional lighting model. Positive intensity becomes the highlight, negative
becomes the shadow. Returns both contributions, clipped to the layer, defaulting
to `Screen` and `Multiply`.

`Depth` scales the normals' steepness. `Direction` flips the light so the surface
reads as raised (`DirUp`) or carved (`DirDown`). `Altitude` defaults to 45
degrees when zero.

**This is the one effect that is not spec-accurate.** The other seven derive from
published formulas; bevel has no clean public specification, so it is
close-and-configurable rather than pixel-exact against Photoshop. It is expected
to iterate against real reference renders. Treat its parameters as a lighting
model to tune, not a spec to match.

## Choke and Spread

Both are morphological operations on the coverage field, applied before the
softening blur, matching Photoshop's model:

- **Choke** (shadows) erodes, taking the minimum over a window. Shrinks coverage.
- **Spread** (glows) dilates, taking the maximum. Grows coverage.

Both are separable min/max box filters, horizontal then vertical. They pad with
**zero** at the edges, so eroding shrinks toward transparent and
dilating grows into transparent. This is the opposite of the clamping
[`blur`](blur.md#edges) uses, and it is correct for each case.

Radii round to whole pixels.

## Cost

Every effect allocates at least one document-sized buffer, and bevel allocates
several. A layer with eight effects on a 4000x4000 document is moving a lot of
memory.

The blurs themselves are cheap: `blur.Gaussian` is
[O(1) per pixel in the radius](blur.md#three-boxes-make-a-gaussian), so a large
soft shadow costs no more than a tight one. Allocation is the thing to watch, not
blur radius.

## Gotchas

- **Zero `Opacity` means opaque**, not invisible.
- **Zero `Mode` means the effect's default**, so you cannot explicitly select
  `Normal` for a shadow or glow.
- **Effect order in the slice is ignored.** See [stacking order](#stacking-order).
- **`StrokeCenter` reaches `Width/2` per side**, unlike the other alignments.
- **Morphology radii round to integers.** Sub-pixel `Choke`, `Spread`, and stroke
  `Width` values quantize.
- **Bevel is not pixel-exact** against Photoshop, by design.
