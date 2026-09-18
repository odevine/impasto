# blend

`blend` takes pixels and a mode and returns a blended result, with no knowledge
of layers, masks, or documents. Each mode is a pure function of two pixels,
which makes it straightforward to check against the specification.

```go
import "github.com/odevine/impasto/blend"
```

## Where the formulas come from

All sixteen modes come from [ISO 32000-2] section 11.3.5, the PDF
specification, which standardizes exactly this set. The PDF Association hosts
it at no cost.

[W3C Compositing and Blending Level 1][w3c] restates the same formulas and is
easier to link into, since it carries an anchor per section:

| W3C section                                   | Covers                             |
| --------------------------------------------- | ---------------------------------- |
| [6. General formula][w3c-formula]             | How `B` combines with alpha        |
| [10. Blending][w3c]                           | All sixteen modes and their groups |
| [10.1. Separable blend modes][w3c-sep]        | The twelve per-channel modes       |
| [10.2. Non-separable blend modes][w3c-nonsep] | Hue, Saturation, Color, Luminosity |

Soft Light uses the spec's piecewise [`D(x)` helper][w3c-softlight] on its upper
branch rather than a closed-form approximation.

The tests check each mode against the spec formulas computed independently, per
channel, across a grid that includes the discontinuity points where the
piecewise branches meet.

[ISO 32000-2]: https://pdfa.org/sponsored-standards/
[w3c]: https://www.w3.org/TR/compositing-1/#blending
[w3c-sep]: https://www.w3.org/TR/compositing-1/#blendingseparable
[w3c-nonsep]: https://www.w3.org/TR/compositing-1/#blendingnonseparable
[w3c-formula]: https://www.w3.org/TR/compositing-1/#generalformula
[w3c-softlight]: https://www.w3.org/TR/compositing-1/#blendingsoftlight

## The compositing formula

Every mode goes through the [same general formula][w3c-formula] from the spec.
The blend
function `B` only touches color, and alpha is always plain source-over:

```
Co = as(1-ab)Cs + as*ab*B(Cb,Cs) + (1-as)ab*Cb
ao = as + (1-as)ab
```

Reading the color term left to right: where the source covers but the backdrop
does not, you get pure source. Where both cover, you get the blend function.
Where the backdrop covers but the source does not, you get pure backdrop.

This matters because it explains a result that otherwise looks broken: **a blend
mode has no effect over transparency.** Multiply against an empty backdrop gives
you the source unchanged, because `ab` is zero and the middle term vanishes. If
your Multiply layer looks like it is ignoring the mode, check whether there is
actually anything beneath it, and check whether the group is
[isolated](canvas.md#groups).

`B` needs straight (non-premultiplied) color, so `BlendPixel` divides alpha out,
blends, and re-premultiplies. The Normal path skips all of that, which is why it
has a dedicated fast path.

## The two calls

### BlendPixel

```go
out := blend.BlendPixel(backdrop, source [4]float32, m Mode) [4]float32
```

Both inputs are premultiplied and linear. Opacity is applied by scaling all four
components of the source before the call, not by a parameter:

```go
faded := [4]float32{src[0] * 0.5, src[1] * 0.5, src[2] * 0.5, src[3] * 0.5}
out := blend.BlendPixel(backdrop, faded, blend.Multiply)
```

A source with zero alpha returns the backdrop untouched, so there is no need to
guard empty regions.

### Composite

```go
blend.Composite(dst, src *raster.Buffer, m Mode, opacity float32)
```

Buffer-level, in place, parallel across fixed row bands. Both buffers must be
the same size; it panics otherwise, since a size mismatch is always a caller
bug rather than a runtime condition. Opacity is clamped to `[0,1]`, and zero or
below returns immediately.

`Normal` takes a separate path that skips straight-color recovery entirely.
Since most layers in most documents are Normal, this is the case worth
optimizing.

## The modes

Values are a fixed `iota` ordering that is part of the API. You can persist the
integers.

### Normal

Source replaces backdrop, weighted by alpha. No color interaction. `B(cb,cs) = cs`.

### Darkening: Multiply, Color Burn, Darken

**Multiply** multiplies channels. Always darkens or leaves alone, white is
neutral. Think stacked transparencies, or ink on paper. This is the default for
shadows for a reason: real shadows attenuate what is behind them.

**Color Burn** darkens the backdrop to reflect the source, with a much steeper
response than Multiply. Produces crushed, saturated shadows. Easy to overdo.

**Darken** keeps the per-channel minimum. Unlike Multiply it can leave a channel
completely untouched, so it tends to shift hue in ways Multiply does not.

### Lightening: Screen, Color Dodge, Lighten

**Screen** is inverse-multiply: `cb + cs - cb*cs`. Always lightens, black is
neutral. Think projected light, two projectors aimed at one wall. The default
for glows.

**Color Dodge** brightens the backdrop to reflect the source. Produces blown-out
highlights and clips readily.

**Lighten** keeps the per-channel maximum.

### Contrast: Overlay, Soft Light, Hard Light

These three all multiply the dark regions and screen the light ones. They differ
in which operand decides:

**Overlay** keys on the *backdrop*. Boosts the backdrop's existing contrast.

**Hard Light** is Overlay with the operands swapped, keying on the *source*.
Harsh, like a hard spotlight.

**Soft Light** is a gentler dodge or burn, using the spec's piecewise `D(x)`
helper on its upper branch. Like a diffuse spotlight. This is the one most
libraries approximate and impasto does not.

### Comparative: Difference, Exclusion

**Difference** is the absolute difference of the two. Identical colors cancel to
black, which makes it the standard tool for visually diffing two images.

**Exclusion** is similar with lower contrast: mid-grays stay gray.

### Non-separable: Hue, Saturation, Color, Luminosity

These four act on the whole RGB triple at once rather than channel by channel,
using the spec's [`Lum`, `SetLum`, `Sat`, and `SetSat` helpers][w3c-nonsep].
Each takes one
perceptual attribute from the source and the rest from the backdrop:

| Mode       | From source     | From backdrop         |
| ---------- | --------------- | --------------------- |
| Hue        | hue             | saturation, luminance |
| Saturation | saturation      | hue, luminance        |
| Color      | hue, saturation | luminance             |
| Luminosity | luminance       | hue, saturation       |

**Color** is the classic tinting mode: it recolors while preserving the
backdrop's light and shade, which is how you colorize a grayscale photograph.
**Luminosity** is its complement and is useful for applying a texture's
structure without its color.

`SetLum` clips out-of-gamut results back into the cube by scaling toward the
luminance rather than clamping per channel, which is what stops these modes from
producing hue shifts at the extremes.

## Mode as a value

```go
m := blend.SoftLight
fmt.Println(m)          // "SoftLight"
fmt.Println(m.Valid())  // true

fmt.Println(blend.Mode(99).Valid())  // false
fmt.Println(blend.Mode(99))          // "Mode(invalid)"
```

`Valid` is worth calling on any mode that came from configuration or a file, since
an out-of-range mode falls through to Normal rather than failing loudly.

## Gotchas

- **A blend mode does nothing over transparency.** See
  [the formula](#the-compositing-formula).
- **Opacity is not a parameter of `BlendPixel`.** Scale the source yourself.
- **Results are in linear light.** A Multiply of two mid-gray sRGB values is not
  `0.25` in sRGB terms; it is `0.25` in linear terms, which re-encodes to
  roughly sRGB 137.
- **`Composite` panics on a size mismatch.** Check `SameSize` if your sizes are
  dynamic.
