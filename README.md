# impasto

![impasto, a pure-Go library for Photoshop-style layered image compositing](examples/out/banner.png)

[![CI](https://github.com/odevine/impasto/actions/workflows/ci.yml/badge.svg)](https://github.com/odevine/impasto/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/odevine/impasto.svg)](https://pkg.go.dev/github.com/odevine/impasto)

A pure-Go library for Photoshop-style layered image compositing: blend modes,
masks, vector paths, gradients, blur, and non-destructive layer effects.
Intentionally built with `CGO_ENABLED=0` and no dependencies outside the
standard library.

This is just a library and not a full application, so there is no CLI, and no
opinion about file formats beyond decoding into and encoding out of `image.Image`.
The one exception is the optional `mpcfill` package, which nothing else imports.
This way, a poster renderer, a game's UI layer, or a template service should all
find it equally usable.

## Why

Go has pieces of this already. `image/draw` does Porter-Duff compositing,
`x/image/vector` fills paths. Neither does Photoshop's blend math, layer
effects, or dithered gradients, and the alternatives all mean CGO bindings to
Skia or Cairo.

There are definitely better alternatives outside Go, but I'm stubborn and
wanted to try filling out that missing middle piece that Go didn't seem to have.

## Design

**Compositing happens in linear light, premultiplied, at float32.** Source pixels
decode from sRGB on ingest and re-encode on output, so compositing always
happens in linear space.

**Rendering is deterministic.** The same input gives byte-identical output every
run. Parallel work is split into fixed row bands rather than scheduled
dynamically.

**Expensive operations are concurrent.** Splits across `GOMAXPROCS` goroutines.

**Inputs are immutable.** Effects and masks work on copies, so a layer can be
re-composited with different settings without its content changing.

**Effects are declarative.** A layer effect is a parameter struct rather than an
imperative call, and the renderer decides how and when to rasterize it.

## Packages

Lower layers never import higher ones, so each package works on its own.

| Package    | Responsibility                                                                                     |
| ---------- | -------------------------------------------------------------------------------------------------- |
| `raster`   | The `Buffer` type, sRGB/linear conversion, 8/16-bit image ingest and egress with ordered dithering |
| `blend`    | The blend-mode formulas from ISO 32000-2, per-pixel and buffer-level                               |
| `mask`     | Raster and vector masks, coverage composition                                                      |
| `path`     | Bezier construction, anti-aliased fill (nonzero/even-odd), and stroking (joins, caps, dashes)      |
| `gradient` | Linear, radial, angle, reflected, and diamond gradients with per-stop opacity                      |
| `blur`     | 3-pass box blur as a separable Gaussian approximation                                              |
| `effects`  | The declarative layer styles                                                                       |
| `canvas`   | Layer stack, groups, and `Render`                                                                  |
| `mpcfill`  | Writes rendered cards as an [MPC Autofill](https://github.com/chilli-axe/mpc-autofill) project     |

Most callers import only `canvas`.

## Install

```
go get github.com/odevine/impasto
```

Requires Go 1.24+.

## Quickstart

```go
doc := &canvas.Document{
    Width: 200, Height: 120,
    Root: canvas.Group{
        PassThrough: true, Opacity: 1,
        Layers: []canvas.Node{
            &canvas.Layer{Content: bg, Opacity: 1, Mode: blend.Normal},
            &canvas.Layer{
                Content: logo,
                Opacity: 1,
                Mask:    mask.NewVectorMask(shape, 200, 120, path.NonZero),
                Effects: []effects.Effect{
                    &effects.DropShadow{Color: color.Black, Opacity: 0.75,
                        Angle: 2.36, Distance: 12, BlurRadius: 18, Mode: blend.Multiply},
                    &effects.Stroke{Width: 4, Color: color.White,
                        Alignment: effects.StrokeOutside},
                },
            },
        },
    },
}

out := canvas.MustRender(doc)
png.Encode(w, out.ToImage(8))
```

Layer content is document-sized. Place a smaller image with `canvas.Place`.

## Documentation

The API reference lives on
[pkg.go.dev](https://pkg.go.dev/github.com/odevine/impasto), with runnable
examples attached to the symbols that need them.

The [`docs/`](docs/) directory covers things beyond just references. Things like
how the pieces all fit together, why the model works the way it does, and when
the obvious thing might be the wrong thing.

| Guide                        | Covers                                                   |
| ---------------------------- | -------------------------------------------------------- |
| [raster](docs/raster.md)     | The color model, `Buffer`, ingest and egress, dithering  |
| [blend](docs/blend.md)       | The 16 modes, what each is for, the compositing formula  |
| [mask](docs/mask.md)         | Raster, vector, procedural, and combined coverage        |
| [path](docs/path.md)         | Bezier construction, fill rules, anti-aliasing, stroking |
| [gradient](docs/gradient.md) | The five geometries, stops, opacity, spread modes        |
| [blur](docs/blur.md)         | The three-box Gaussian approximation, sigma vs radius    |
| [effects](docs/effects.md)   | The eight layer styles, their parameters, stacking order |
| [canvas](docs/canvas.md)     | Documents, layers, groups, clipping, rendering           |
| [mpcfill](docs/mpcfill.md)   | MPC Autofill projects, and why names get rewritten       |

New to the library? Read [canvas](docs/canvas.md), then
[raster](docs/raster.md).

## Examples

The [`examples/`](examples/) directory has runnable programs that render PNGs
showing off each part of the library. See
[examples/README.md](examples/README.md) for a walk-through of every mode,
effect, and gradient with the image beside it.

[![Showcase poster](examples/out/showcase.png)](examples/README.md#showcase)

| Program                             | Renders                                                       |
| ----------------------------------- | ------------------------------------------------------------- |
| [`blendmodes`](examples/blendmodes) | All 16 blend modes as a chart                                 |
| [`gradients`](examples/gradients)   | The five gradient types plus a transparency fade              |
| [`effects`](examples/effects)       | The eight layer styles on a badge                             |
| [`paths`](examples/paths)           | Stroke caps, joins, dashes, fill rules, and beziers           |
| [`showcase`](examples/showcase)     | A poster combining gradients, masks, groups, and every effect |
| [`banner`](examples/banner)         | The README banner at the top of this page                     |

Run one with `go run ./examples/<name>`, or regenerate all of them with
`make examples`.

## Text

This library does not shape text. That's something that a package like `go-text/typesetting`
does very well already, so the recommendation is to rasterize glyph outlines yourself,
pass them into the `path` package, and hand the resulting buffer to `canvas` as an
ordinary layer.

Nothing below `canvas` needs to know a layer's pixels came from text, so rasterized
text gets shadows and strokes like anything else.

## Testing

Blend modes are checked against the ISO formulas computed independently, per
channel, across a grid that includes the discontinuity points. Path coverage is
validated by area conservation and exact partial-pixel coverage. Golden-image
regression runs over a full document, regenerated with `go test ./canvas -update`.

Fuzzing covers the path rasterizer, stroker, and gradient sampling, which is
where untrusted or degenerate input first reaches numeric code:

```
go test ./path -fuzz=FuzzRasterize
```

Benchmarks track blend throughput, blur throughput, and full-document render:

```
go test ./... -bench=.
```

## Development

A `Makefile` wraps the common tasks, and `make ci` runs exactly what CI runs:

```
make ci       # gofmt check, vet, build, race tests
make test     # plain test suite
make cover    # coverage summary
make fuzz     # short fuzz smoke run
make examples # regenerate the example images
make hooks    # enable the pre-commit hook (fmt + vet + test)
```

CI runs on every push and pull request: a test matrix on the minimum and current
Go versions, `govulncheck`, and a fuzz smoke run. Releases are automated with
[release-please](https://github.com/googleapis/release-please). Commits follow
[Conventional Commits](https://www.conventionalcommits.org), and merging the
release PR tags a semver version and publishes a GitHub Release.

## Status

Seven of the eight effects are built to spec-level accuracy. Bevel and emboss is
close-and-configurable rather than pixel-exact, by design: it is the one effect
with no clean public spec, so it derives a normal map from the layer's alpha and
applies a directional lighting model. Expect it to keep iterating against real
reference renders.

## Non-goals (v1)

No font shaping, no CMYK or print color management, no GPU backend, no
animation.

## License

[MIT](LICENSE)
