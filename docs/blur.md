# blur

`blur` blurs a buffer in place, quickly and deterministically. `effects` uses it
for every shadow, glow, and bevel, and it is exported for callers who want the
primitive on its own.

```go
import "github.com/odevine/impasto/blur"
```

## The API

```go
blur.Gaussian(b *raster.Buffer, sigma float32)  // approximate Gaussian
blur.BoxBlur(b *raster.Buffer, radius int)      // one box pass
```

Both operate in place on premultiplied channels. Both are no-ops for
non-positive input, so a configured radius can be passed straight through
without a guard:

```go
blur.Gaussian(buf, cfg.BlurRadius)  // fine when BlurRadius is 0
```

## Three boxes make a Gaussian

`Gaussian` does not evaluate a Gaussian kernel. It runs three successive box
blurs, which is a well-known approximation with two properties that matter here.

**It converges fast.** By the central limit theorem, repeated convolution with
any kernel tends toward a Gaussian. Three box passes get within a visually
indistinguishable error, and the fourth pass is not worth its cost.

**It is O(1) per pixel in the radius.** A box blur is a sliding window: advancing
one pixel adds one sample and drops one. The work per pixel does not depend on
the radius at all. A true Gaussian kernel costs O(radius) per pixel per axis,
which is where the cost of a large blur normally comes from.

Combined with separability (a horizontal pass then a vertical one, rather than a
2D kernel), a sigma-50 blur costs the same per pixel as a sigma-2 blur. Large
soft shadows are effectively free.

The box widths come from Ivan Kutskir's derivation, which picks three odd widths
whose combined variance matches the requested sigma.

## Sigma is not radius

`Gaussian` takes a **standard deviation** in pixels. `BoxBlur` takes a **radius**
in pixels. They are not interchangeable, and sigma is the smaller number for a
blur of the same apparent size: a Gaussian's visible extent runs to roughly
three sigma.

The effects package calls its parameters `BlurRadius` and passes them to
`Gaussian` as sigma, so an `effects.DropShadow{BlurRadius: 10}` spreads roughly
30 pixels.

## Premultiplied input matters

The blur runs on all four channels, including alpha, with no special casing.
That is only correct because the buffer is
[premultiplied](raster.md#the-color-model).

With straight alpha, fully transparent pixels still carry a color value, and
averaging pulls that color into the visible region: a white logo blurred on a
transparent background picks up a dark halo from the black-with-zero-alpha
pixels around it. Premultiplied, transparent is `{0,0,0,0}`, and averaging it in
correctly contributes nothing.

## Edges

Both passes clamp (replicate) at the edges rather than wrapping or treating
outside as zero.

For a transparent margin this leaves the margin transparent, which is what you
want. For an image that runs to its own border, it avoids the dark vignette a
zero-padded blur produces.

Note that [`effects`](effects.md) morphology (`Choke`, `Spread`) pads with zero
instead, so that eroding shrinks toward transparent and dilating
grows into it.

## Determinism

Both passes parallelize across fixed row (or column) bands. The partitioning is
by index, never by scheduling order, so output is byte-identical across runs and
across `GOMAXPROCS` values. This is what makes the library's golden-image tests
possible.

## Cost

Each `Gaussian` call allocates one temporary the size of the buffer and reuses it
across all three passes. `BoxBlur` allocates one per call. For a pipeline running
many blurs over the same dimensions, that allocation is the thing to watch in a
profile.

## Gotchas

- **Sigma, not radius**, for `Gaussian`. Visible extent is roughly three sigma.
- **Non-positive is a no-op**, not an error.
- **Input must be premultiplied**, or you get halos.
- **`BoxBlur` radius is an int.** Fractional radii round.
