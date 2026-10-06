# Bounded and lazy layers

This document specifies changes to `canvas` that let a layer cover only the part of the document it draws on, and let a layer's pixels be produced at the moment they are composited instead of held for the whole render. Both exist to cut peak memory. Output must stay byte-identical for every document that renders today.

## Why

A `raster.Buffer` is `[]float32` with four channels per pixel, so a 3264x4440 document costs 221 MiB per buffer. `canvas.Layer.Content` must be document-sized, so every layer costs that much however small its content is, and a caller builds the whole node tree before `Render` runs, so every layer is resident at once.

A heap profile of one card render in the mimic engine (a frame of 8 layers plus 10 text boxes, a divider and the art, with the profile taken at peak, `GOGC=10`) showed 4.76 GB in use, 99.9% of it raster buffers:

| Source                                        | Buffers | Share |
| --------------------------------------------- | ------- | ----- |
| Text boxes, one document-sized buffer each    | 10      | 48%   |
| Frame layers, loaded and placed up front      | 8       | 38%   |
| `canvas.Render` output and an isolated group  | 2       | 9%    |
| Divider layer                                 | 1       | 5%    |
| A cloned frame buffer                         | 1       | 5%    |

Memory, not CPU, bounds how many cards can render in parallel. One render peaks near 6 GB of RSS at `GOGC=25`, and throughput scaled almost linearly from 1 to 4 workers (0.17 to 0.61 cards per second on a 16 core machine) while using 6 GB to 18 GB. At that cost a 16 GB machine cannot run more than one or two renders at a time.

The caller already works around the document-sized rule where it can. The mimic engine's set symbol is drawn into a small buffer and then `Place`d, which still allocates a full document buffer for the layer.

## Goal and non-goals

The goal is a peak of about 4 to 5 document-sized buffers for the card scene above, roughly 1 GB, down from about 21.

Not in scope: changing the working representation (float32, premultiplied, linear), changing any blend math, or shrinking isolated group buffers, which stay document-sized.

## Phase 0: stop copying

Two changes in `canvas.renderLayer`, with no API change.

`renderLayer` clones `Content` on every call. The clone is only needed when something mutates the content, which is a non-nil `Mask` or a `ClipToBelow` layer with a clip base. Clone in those cases only. Before relying on this, confirm that no effect in `effects` writes to the layer buffer it is given. The `Effect` contract says the content is already masked and passed for reading, and the effects read it today, so this should hold, but a test should pin it.

`renderLayer` also builds an alpha slice for every layer so a following clip layer could use it as a base. Build it only when the next sibling is a `ClipToBelow` layer.

Together these remove about 280 MB of allocation per composited layer at 3264x4440. The existing golden tests in `canvas/golden_test.go` must pass unchanged.

## Phase 1: bounded layers

A layer states where its content sits in the document, and content may be smaller than the document.

### API

```go
type Layer struct {
    Content     *raster.Buffer
    Origin      image.Point // document position of Content's top-left, zero by default
    // ... existing fields
}
```

A zero `Origin` with document-sized content is exactly today's layer. Content that extends past the document is clipped, as `Place` clips today.

`blend` gains a rect-limited composite:

```go
func CompositeRect(dst, src *raster.Buffer, origin image.Point, m Mode, opacity float32)
```

It blends `src` over the part of `dst` that `src` covers at `origin`, clipped to `dst`, and touches nothing else. `Composite` becomes a call to it with a zero origin and keeps its same-size panic. The row-band split in `internal/parallel` applies to the rect's rows, so output stays independent of scheduling.

Effects declare how far they can reach beyond the content:

```go
type Bounded interface {
    Bleed() int // pixels past the content's edges the effect can write
}
```

`DropShadow.Bleed` returns `ceil(Distance + 3*BlurRadius + Choke) + 2`. The extra 2 covers bilinear sampling in `offsetAlpha`. Other effects implement it where their reach is local and are left alone otherwise.

### Semantics

**Masks** are sampled in document coordinates. At a pixel of the content buffer `(x, y)` the coverage is `Coverage(x+Origin.X, y+Origin.Y)`. For a zero origin this is today's behavior. `mask.Apply` documents the buffer's top-left as the mask's origin, so it needs an origin parameter or a wrapper that offsets the mask.

**Effects** run on a scratch buffer. When every effect on a layer implements `Bounded`, canvas pads the content by the largest `Bleed()`, clipped to the document, runs the effects there, and composites each result at the padded origin. When any effect does not implement it, canvas expands the layer to the whole document first, which is today's behavior. Position-dependent effects such as gradient overlays and bevels therefore keep their meaning.

**Clip-to-below** keeps its coverage base as a rect with its own origin, so a clipped layer multiplies only where the two overlap and is zero elsewhere.

**Isolated groups** keep a document-sized buffer. The group's children composite into it with `CompositeRect`.

## Phase 2: lazy layers

A layer can supply its pixels at composite time.

### API

```go
type Layer struct {
    Content *raster.Buffer
    Load    func() (*raster.Buffer, image.Point, error) // used when Content is nil
    // ... existing fields
}
```

Canvas calls `Load` immediately before compositing the layer and drops its reference once the layer is done, so one loaded buffer is live at a time. The returned origin plays the role of `Origin`.

`Render` and the internal `renderGroup`, `renderChildren` and `renderLayer` gain an error return. `Render` returns the first `Load` error, wrapped with the layer's position in the stack, and stops. The package doc line that says `Render` fails for exactly one reason needs to change.

A buffer returned by `Load` belongs to canvas, so masks and clip multiplication mutate it in place with no clone. A buffer in `Content` still belongs to the caller and is cloned when something would mutate it, as in Phase 0.

### Prefetch

Streaming makes PNG decode sequential with blending. An optional `RenderOptions{Prefetch int}` could run `Load` for the next layer while the current one composites, at the cost of one more live buffer per step ahead. It should default to zero and stay off unless a benchmark shows the decode stall matters. Decide this after Phase 2 is measured, not before.

## Mimic engine follow-up

These changes are outside this repository and are listed so the library API can be checked against its first user. They land in the mimic engine once a release with Phases 1 and 2 exists.

- `template.LoadLayerAt` returns a `Layer` with `Load`, so decode, scale and wrap all happen inside the callback.
- `RenderTextBox` allocates an image for the box rectangle plus a margin instead of `image.NewRGBA(image.Rect(0, 0, docW, docH))`, and returns it with its document position in `Bounds().Min`. Go images support a non-zero `Min` natively, so the drawing code may already work. This needs checking. The margin comes from the box shadow's `Bleed()`, which for the engine's hard shadows (`BlurRadius` zero) is `ceil(Distance) + 2`.
- `raster.FromImage` keeps `Bounds().Min` available so the caller can set `Origin`. Today it ignores it.
- The art slot and set symbol use `Origin` in place of `canvas.Place`.
- A layer with `Mirror` flips about the canvas centre and a blended-color layer spans the card, so both stay document-sized.

## Expected memory

The figures below are estimates from the buffer counts above and need to be confirmed with a benchmark after each phase.

| After   | Live buffers at peak                         | Approx. working set |
| ------- | -------------------------------------------- | ------------------- |
| Today   | about 21                                     | 4.7 GB              |
| Phase 0 | about 21, with lower transient allocation    | 4.5 GB              |
| Phase 1 | about 11, text boxes and art shrink          | 2.4 GB              |
| Phase 2 | about 4 to 5, output, current layer, ToImage | 1.0 GB              |

Blending each small layer only over its own rect should also reduce compositing time, since a text box today blends 14.5 million pixels to touch a small strip.

## Verification

Equivalence is the main requirement, so most tests compare two renders of the same scene.

- **Bounded against placed.** Render a scene once with document-sized `Place`d layers and once with bounded layers and `Origin`. Assert identical bytes. Cover a layer at each document edge and corner, one partly outside the document, and a layer with a `Mask`.
- **Effect bleed.** Drop shadows at several distances and blur radii on a bounded layer, with the shadow reaching exactly the `Bleed()` limit. Compare against the expanded-to-document result. Include a layer with an effect that does not implement `Bounded` and check it falls back.
- **Clip-to-below.** A clipped layer over a bounded base, with the two rects overlapping partially, fully and not at all.
- **Lazy against eager.** The same scene with `Content` and with `Load` must match. A `Load` error stops the render and returns an error that names the layer. A `Load` that returns a buffer canvas then mutates must not disturb a second render that reuses the same source.
- **Existing tests.** `canvas/golden_test.go` and the other package tests pass unchanged at every phase.
- **Memory benchmark.** Add a benchmark in `canvas` that builds a scene shaped like the one profiled above at 3264x4440: eight document-sized layers, ten bounded text-sized layers, one with a hard offset shadow, and a small icon. Report peak heap with `b.ReportMetric` from `runtime.MemStats`, and run it at each phase. This is the number to quote when a phase lands.

## Risks

A `Bleed()` that is too small clips a shadow without any error. The boundary tests above are what catch it, so they should be written before the effect implementations.

The mask coordinate change affects only callers who pass a non-zero `Origin`, which cannot exist before this lands.

Streaming moves PNG decode onto the critical path. If single-render latency regresses noticeably, prefetch is the remedy.

This is additive, so it is a minor release for release-please. Existing callers that build document-sized layers are unaffected.
