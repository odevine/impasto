# impasto documentation

These guides cover things beyond just the API reference. Things like how the
pieces all fit together, why the model works the way it does, and when the
obvious thing might be the wrong thing.

For the API itself, use [pkg.go.dev](https://pkg.go.dev/github.com/odevine/impasto).
Every exported symbol is documented there, with runnable examples attached to
the ones that need them.

## Start here

If you are new to the library, read [canvas](canvas.md) first. Most callers
never import anything else. Then read [raster](raster.md), because the color
model it defines is the one assumption every other package inherits.

## The guides

Ordered bottom to top. Lower layers never import higher ones, so any of them can
be used on its own.

| Guide                   | Covers                                                                       |
| ----------------------- | ---------------------------------------------------------------------------- |
| [raster](raster.md)     | `Buffer`, the linear/premultiplied color model, ingest and egress, dithering |
| [blend](blend.md)       | The 16 blend modes, what each is for, the compositing formula                |
| [mask](mask.md)         | Raster, vector, procedural, and combined coverage                            |
| [path](path.md)         | Bezier construction, fill rules, anti-aliasing, stroking, dashes             |
| [gradient](gradient.md) | The five geometries, stops, opacity, spread modes                            |
| [blur](blur.md)         | The three-box Gaussian approximation, sigma vs radius                        |
| [effects](effects.md)   | The eight layer styles, their parameters, stacking order                     |
| [canvas](canvas.md)     | Documents, layers, groups, clipping, rendering                               |
| [mpcfill](mpcfill.md)   | Writing cards as an MPC Autofill project, and the rules that shape it        |

## Cross-cutting topics

A few things span packages and are documented where they bite hardest:

- **The color model** (linear light, premultiplied, float32) is in
  [raster](raster.md#the-color-model). Read it before anything else, because
  every buffer in the library follows those rules.
- **Determinism** and what preserves it is in [canvas](canvas.md#determinism).
- **Effect stacking order** is in [effects](effects.md#stacking-order).
- **Text** is in [canvas](canvas.md#text), because impasto does not
  shape it.

## Seeing it work

The [examples directory](../examples/) has six runnable programs that render
PNGs, with a [walk-through](../examples/README.md) covering every blend mode,
gradient type, effect, and stroke feature with the image next to it. If you
learn better from pictures than prose, start there instead.
