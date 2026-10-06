# canvas

`canvas` assembles layers and groups into a document and flattens the stack to a
single buffer. It is the orchestration layer, and the only package most callers
import.

```go
import "github.com/odevine/impasto/canvas"
```

## The model

```go
type Document struct {
    Root          Group
    Width, Height int
}

type Group struct {
    Layers      []Node
    PassThrough bool
    Opacity     float32
    Mode        blend.Mode
    Mask        mask.Mask
}

type Layer struct {
    Content     *raster.Buffer
    Origin      image.Point
    Opacity     float32
    Mode        blend.Mode
    Mask        mask.Mask
    Effects     []effects.Effect
    ClipToBelow bool
}
```

`Node` is a closed interface: a node is a `*Layer` or a `*Group`, and nothing
else. Layers composite bottom to top in slice order, exactly as they read.

## Rendering

```go
out, err := canvas.Render(doc)   // error only on invalid dimensions
out := canvas.MustRender(doc)    // panics instead
img := out.ToImage(8)
```

`Render` fails for exactly one reason, invalid document dimensions:

```go
_, err := canvas.Render(&canvas.Document{Width: 0, Height: 10})
// raster: invalid buffer dimensions: 0x10 must be positive
```

Everything else about a document is renderable by construction. A nil layer
`Content` is skipped, an out-of-range blend mode falls back to Normal, an empty
group contributes nothing.

## Layer content and origin

A layer's `Content` can be any size. `Origin` is the document position of its
top-left pixel, and it defaults to zero, so a document-sized buffer with no
`Origin` is the simplest layer.

```go
logo, _ := raster.FromFile("logo.png")     // whatever size it happens to be
layer := &canvas.Layer{Content: logo, Origin: image.Pt(340, 120)}
```

Content that extends past the document is clipped to it, and a layer entirely
outside the document draws nothing. A layer only blends over the part of the
document its content covers, so a small layer costs memory and time in
proportion to its own size.

Masks and clip-to-below act in document coordinates. A mask is asked for
`Coverage(x+Origin.X, y+Origin.Y)` at the content pixel `(x, y)`, so the same
mask gives the same result wherever the layer sits. A clipped layer is confined
to the alpha of its base where the two overlap, and is empty everywhere else.

Effects see only the layer's content, grown by the largest [`Bleed`](effects.md#bounded-effects)
among them. When every effect on a layer is `Bounded` the effects run on that
smaller buffer. When any is not, the layer is expanded to the whole document
first, so such effects behave as they do for a document-sized layer.

A bounded layer renders the same as the same pixels placed into a document-sized
buffer. Layers with a drop shadow can differ by about 1e-6 in a float, because
the shadow is computed on a differently sized buffer and its offset and blur round
differently. In 8-bit output that shows as a byte that is one level apart, on a
very small share of pixels.

### Placing content

`Place` predates `Origin` and still works. It is useful when a buffer of the
document's size is wanted, for example as the base for `PlaceInto`.

```go
content := canvas.Place(docW, docH int, src *raster.Buffer, x, y int) *raster.Buffer
canvas.PlaceInto(dst, src *raster.Buffer, x, y int)
```

`Place` allocates a new document-sized buffer; `PlaceInto` writes into one you
already have. Both **overwrite** the destination region rather than compositing
into it, and both clip anything falling outside the document.

### Memory

Isolated groups still use a document-sized buffer, and a layer with an effect
that is not `Bounded` is expanded to the document for the render.

Document-sized layers cost memory. Fifty of them on a 4000x4000 document is fifty
256 MB buffers. A layer with a small `Content` and an `Origin` costs only its own
size, and for the rest, composite in stages and feed the flattened result back in
as a single layer.

## Opacity

`Opacity` is in `(0,1]`, and **a non-positive value is treated as fully opaque**,
not invisible. The zero value of a `Layer` is a visible layer, which makes
`&canvas.Layer{Content: buf}` do the obvious thing. Values above 1 clamp.

To hide a layer, omit it from the slice rather than setting opacity to zero.

A layer's opacity scales the layer **and its effects together**, so fading a
layer fades its shadow with it rather than leaving a disembodied shadow behind.

## Groups

A group nests nodes and can blend them as a unit.

**Pass-through** groups let their children blend with whatever is beneath the
group. They are a pure organizational construct: the children composite directly
onto the backdrop, as if the group were not there.

**Isolated** groups composite their children onto a transparent buffer first,
then blend that result as a unit. The children never see the backdrop.

A group is isolated whenever any of these hold:

- `PassThrough` is false
- `Opacity` is below 1
- `Mask` is non-nil
- `Mode` is not `Normal`

The last three force isolation because there is no other way to apply them to the
group as a whole.

The difference is invisible until a child uses a non-Normal mode:

```go
// backdrop: mid-gray. group child: mid-gray, Multiply.
// pass-through: 0.25   the child multiplies against the backdrop
// isolated:     0.50   the child multiplies against transparency, so it keeps
//                      its own value, and the group lands on the backdrop
```

This is the same rule as [a blend mode over transparency](blend.md#the-compositing-formula),
and it is the most common source of "why does my blend mode do nothing" in any
layered renderer.

The root group is usually pass-through:

```go
Root: canvas.Group{PassThrough: true, Opacity: 1, Layers: []canvas.Node{...}}
```

## Clipping

`ClipToBelow` confines a layer to the alpha of the layer beneath it, Photoshop's
clipping mask.

```go
Layers: []canvas.Node{
    &canvas.Layer{Content: badge},                        // the base
    &canvas.Layer{Content: texture, ClipToBelow: true},   // clipped to the badge
    &canvas.Layer{Content: shine, ClipToBelow: true},     // also clipped to the badge
}
```

The base is the most recent **non-clipped** layer, so a run of clipped layers all
attach to the same base rather than chaining. A nested group is never a clip
base, and clears the current one.

Clipping uses the base layer's alpha **after** its mask is applied but
**before** its effects render, so a clipped layer is confined to the base's
shape, not the base's shadow.

This lives in `canvas` rather than [`mask`](mask.md#what-is-not-here) because it
is a compositing-order rule: its coverage depends on the layer stack, which a
`Mask` has no access to.

## How a layer renders

Worth knowing in order, because it explains most ordering questions:

1. **Crop** the content to the document, if it extends past it.
2. **Copy** the content, only if it is about to be masked or clipped, so the
   caller's buffer is never mutated.
3. **Apply the mask**, if any, in document coordinates.
4. **Clip to below**, if set, or become the clip base for the layers above.
5. **Sort the effects** into [canonical order](effects.md#stacking-order).
6. **Render each effect** against the masked content, grown by the effects'
   [bleed](effects.md#bounded-effects) or expanded to the document.
7. **Composite behind-effects** (shadows, outer glow) onto the backdrop.
8. **Composite the layer** with its own mode and opacity.
9. **Composite front-effects** (everything else) on top.

Two consequences fall out of this. Effects see the layer's content *after*
masking, so a mask reshapes the shadow too. And effect opacity is multiplied by
layer opacity in step 7 and 9, which is what keeps a layer and its effects fading
together.

## Immutability

`Render` never mutates a caller's buffer. Masking and clipping are the only steps
that write to a layer's content, so the copy in step 2 is what guarantees it. A
layer can be re-composited with different settings, or shared between two
documents, without surprises:

```go
doc.Root.Layers[0].(*canvas.Layer).Opacity = 0.5
out2 := canvas.MustRender(doc)  // content is still pristine
```

A layer that is not masked or clipped is composited straight from the buffer you
gave it, so rendering it allocates nothing for the content.

## Determinism

The same input produces byte-identical output across runs, across machines, and
across `GOMAXPROCS` values. This is a guarantee, not a coincidence, and it is
what makes golden-image regression testing possible:

```
go test ./canvas            # compare against the golden image
go test ./canvas -update    # regenerate it
```

It holds because every parallel operation partitions work into **fixed row
bands** computed from the height and worker count, never by scheduling order or
work stealing. Each band writes a disjoint range of rows. Floating-point addition
is not associative, so a work-stealing scheduler that summed bands in a different
order each run would produce different low bits; partitioning by index sidesteps
that entirely.

The [dither pattern](raster.md#why-the-8-bit-path-dithers) is likewise a pure
function of `(x, y)` rather than anything stateful.

What breaks it: a `mask.FuncMask` whose output depends on time, randomness, or
map iteration order. Everything in the library itself is deterministic.

## Text

impasto does not shape text. Shaping is a deep problem in its own right (bidi,
ligatures, cluster boundaries, font fallback), and a package like
`go-text/typesetting` already does it very well.

So the recommendation is to rasterize glyph outlines yourself, feed them through
the [`path`](path.md) package, and hand the resulting buffer to `canvas` as an
ordinary layer.

Nothing below `canvas` needs to know a layer's pixels came from text, so
rasterized text gets shadows, strokes, bevels, and gradient overlays like any
other content.

## Gotchas

- **`Origin` is where the content's top-left sits.** A buffer that was already `Place`d at a position and also given that `Origin` is offset twice.
- **Opacity 0 means opaque**, not hidden. Omit the layer instead.
- **A blend mode inside an isolated group cannot see the backdrop.** See
  [groups](#groups).
- **`Place` overwrites**, it does not composite.
- **Effect slice order is ignored.** See [stacking order](effects.md#stacking-order).
- **The clip base is the last non-clipped layer**, and a group resets it.
