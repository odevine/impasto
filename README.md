# impasto

[![CI](https://github.com/odevine/impasto/actions/workflows/ci.yml/badge.svg)](https://github.com/odevine/impasto/actions/workflows/ci.yml)

A pure-Go (`CGO=0`) library for Photoshop-style layered image compositing: blend
modes, masks, vector paths, gradients, blur, and non-destructive layer effects
(drop shadow, inner shadow, outer/inner glow, gradient/color overlay, stroke,
bevel & emboss).

It is a **library, not an application**. There is no CLI and no file-format
opinion beyond decoding into and encoding out of the standard library's
`image.Image`. It is meant to be as useful to a poster tool or a game's UI
renderer as to a template renderer.

## Why

Go has scattered pieces (`image/draw` for basic Porter-Duff, `x/image/vector`
for path filling) but nothing that reproduces Photoshop's blend-mode math,
layer-style effects, or dithered gradients. impasto fills that gap without
dropping to CGO bindings for Skia or Cairo.

## Design

- **Linear light, premultiplied, float32.** Source pixels are decoded sRGB to
  linear on ingest, composited premultiplied to avoid fringing, and re-encoded
  on output. Compositing in gamma space is not a reachable mistake.
- **Deterministic.** The same input produces byte-identical output across runs.
  Parallelism partitions work into fixed row bands, never by scheduling order,
  which is what makes golden-image testing possible.
- **Concurrent by default.** Every expensive operation splits across
  `GOMAXPROCS` goroutines.
- **Immutable inputs.** Effects and masks work on copies, so a layer can be
  re-composited with different settings without surprises.
- **Effects are declarative.** A layer effect is a parameter struct, not an
  imperative call. The renderer decides how and when to rasterize it.

## Packages

Lower layers never import higher ones, so each is usable on its own.

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

Most callers import only `canvas`.

## Install

```
go get github.com/odevine/impasto
```

Requires Go 1.24+. No non-stdlib dependencies, none pulling in CGO.

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

## Text

impasto never shapes text. Rasterize glyph outlines yourself (for example with
`go-text/typesetting` feeding the `path` package) and hand the resulting buffer
to `canvas` as an ordinary layer. Nothing below `canvas` needs to know a layer's
pixels came from text.

## Testing

- **Blend modes** are checked against the ISO formulas computed independently,
  per channel, across a grid that includes the discontinuity points.
- **Path coverage** is validated by area conservation (triangles, circles) and
  exact partial-pixel coverage.
- **Golden image** regression via `go test ./canvas -update` to regenerate.
- **Fuzzing** covers the path rasterizer, stroker, and gradient sampling, the
  places untrusted or degenerate input first reaches numeric code:
  `go test ./path -fuzz=FuzzRasterize`.
- **Benchmarks** track blend throughput, blur throughput, and full-document
  render: `go test ./... -bench=.`.

## Development

A `Makefile` wraps the common tasks, and `make ci` runs exactly what the CI
workflow runs:

```
make ci       # gofmt check, vet, build, race tests
make test     # plain test suite
make cover    # coverage summary
make fuzz     # short fuzz smoke run
make examples # regenerate the example images
make hooks    # enable the pre-commit hook (fmt + vet + test)
```

Continuous integration runs on every push and pull request (test matrix on the
minimum and current Go versions, `govulncheck`, and a fuzz smoke run). Releases
are automated with [release-please](https://github.com/googleapis/release-please):
commits follow the [Conventional Commits](https://www.conventionalcommits.org)
format, and merging the release PR tags a semver version and publishes a GitHub
Release.

## Status

Seven of the eight effects are built to spec-level accuracy. Bevel & emboss is
close-and-configurable rather than pixel-exact against Photoshop, by design: it
is the one effect with no clean public spec, so it derives a normal map from the
layer's alpha and applies a directional lighting model, and it is expected to
iterate against real reference renders.

## Non-goals (v1)

No font shaping, no CMYK or print color management, no GPU backend,
and no animation.
