// Package canvas is the orchestration layer most callers import. It assembles
// layers and groups into a document and flattens the whole stack to a single
// buffer. Layers carry content, opacity, a blend mode, an optional mask, and a
// list of non-destructive effects, groups nest layers with pass-through or
// isolated blending. Everything below this package is usable on its own for
// callers who only need, say, the blend math.
//
// A layer's content may be smaller than the document. Origin says where its
// top-left sits, and a zero Origin with document-sized content is the simplest
// case. Content that extends past the document is clipped to it.
//
// A layer can also supply its content at the moment it is composited, with Load.
// Only one loaded buffer is alive at a time, so a document of many large layers
// no longer holds all of them for the whole render. Render returns the first
// error a Load reports
package canvas

import (
	"errors"
	"fmt"
	"image"

	"github.com/odevine/impasto/blend"
	"github.com/odevine/impasto/effects"
	"github.com/odevine/impasto/mask"
	"github.com/odevine/impasto/raster"
)

// Node is a layer or a group. The interface is closed to this package's two
// implementations
type Node interface {
	isNode()
}

// Layer is a single composited element. Content is premultiplied linear and
// sits at Origin in the document, it may be smaller than the document. Masks and
// clip-to-below act in document coordinates. Opacity in (0,1] scales the layer
// and its effects together, a non-positive Opacity is treated as fully opaque.
// Effects are applied in a fixed order regardless of slice order.
//
// Load supplies the content, and where it sits, when Content is nil. Canvas calls
// it just before compositing the layer and drops the buffer once the layer is
// done. The buffer belongs to canvas from then on, so Load must return one that
// nothing else uses, and masks and clipping write to it in place. Origin is not
// used for a layer with a Load. A Load that returns a nil buffer contributes
// nothing, and one that returns an error stops the render. Content, when set,
// is used instead and Load is not called.
//
// Image and LoadImage supply the content as an 8-bit sRGB image when Content and
// Load are both nil, with Image placed at Origin and LoadImage returning its own
// origin as Load does. An *image.NRGBA with no mask, effects or clip-to-below is
// blended straight into the document without a float buffer, which is cheaper in
// time and memory and gives the same pixels. A [blend.Indexed] image is blended
// the same way and skips the transparent stretches its index records. Any other
// image, or a layer that needs its content written to, is converted first. Canvas
// never writes to an image, so one decoded image can be shared by any number of
// layers and concurrent renders. A nil or empty image contributes nothing
type Layer struct {
	Content     *raster.Buffer
	Origin      image.Point
	Load        func() (*raster.Buffer, image.Point, error)
	Image       image.Image
	LoadImage   func() (image.Image, image.Point, error)
	Opacity     float32
	Mode        blend.Mode
	Mask        mask.Mask
	Effects     []effects.Effect
	ClipToBelow bool
}

func (*Layer) isNode() {}

// Group nests nodes. A pass-through group lets its children blend with whatever
// is beneath the group, an isolated group first composites its children on a
// transparent buffer and then blends that as a unit. A group with opacity below
// one, a mask, or a non-Normal mode is treated as isolated so those apply to the
// group as a whole
type Group struct {
	Layers      []Node
	PassThrough bool
	Opacity     float32
	Mode        blend.Mode
	Mask        mask.Mask
}

func (*Group) isNode() {}

// Document is a root group with a fixed output size
type Document struct {
	Root          Group
	Width, Height int
}

// Render flattens the document to a single premultiplied linear buffer. It
// returns an error if the document dimensions are invalid, or the first error
// returned by a layer's Load, wrapped with the layer's position in the stack. A
// failed render returns no buffer
func Render(doc *Document) (*raster.Buffer, error) {
	out, err := raster.NewBuffer(doc.Width, doc.Height)
	if err != nil {
		return nil, err
	}
	if err := renderGroup(&doc.Root, out); err != nil {
		return nil, fmt.Errorf("canvas: %w", err)
	}
	return out, nil
}

// MustRender is Render for trusted documents, panicking on an invalid dimension
// or a Load error
func MustRender(doc *Document) *raster.Buffer {
	out, err := Render(doc)
	if err != nil {
		panic(err)
	}
	return out
}

// renderGroup composites a group's children onto backdrop
func renderGroup(g *Group, backdrop *raster.Buffer) error {
	isolated := !g.PassThrough || g.Opacity > 0 && g.Opacity < 1 || g.Mask != nil || g.Mode != blend.Normal
	if !isolated {
		return renderChildren(g.Layers, backdrop)
	}
	inner := raster.MustNewBuffer(backdrop.Width, backdrop.Height)
	if err := renderChildren(g.Layers, inner); err != nil {
		return err
	}
	if g.Mask != nil {
		mask.Apply(inner, g.Mask)
	}
	blend.Composite(backdrop, inner, g.Mode, opacityOr(g.Opacity))
	return nil
}

// renderChildren composites a sequence of nodes onto backdrop, tracking the clip
// base so clip-to-below layers attach to the layer beneath them. It stops at the
// first error and reports where in the stack it happened
func renderChildren(nodes []Node, backdrop *raster.Buffer) error {
	var clipBase *coverage
	for i, n := range nodes {
		switch node := n.(type) {
		case *Layer:
			var err error
			clipBase, err = renderLayer(node, backdrop, clipBase, nextClips(nodes, i))
			if err != nil {
				return fmt.Errorf("layer %d: %w", i, err)
			}
		case *Group:
			if err := renderGroup(node, backdrop); err != nil {
				return fmt.Errorf("group %d: %w", i, err)
			}
			// A nested group does not serve as a clip base
			clipBase = nil
		}
	}
	return nil
}

// nextClips reports whether the node after index i is a clip-to-below layer
func nextClips(nodes []Node, i int) bool {
	if i+1 >= len(nodes) {
		return false
	}
	next, ok := nodes[i+1].(*Layer)
	return ok && next.ClipToBelow
}

// coverage is an alpha field over a rectangle of the document, zero everywhere
// outside it. It is what a clip-to-below layer attaches to
type coverage struct {
	rect  image.Rectangle
	alpha []float32
}

// renderLayer composites one layer with its effects onto backdrop, and returns
// the coverage a following clip-to-below layer should clip against. It is only
// built when needBase is set
func renderLayer(l *Layer, backdrop *raster.Buffer, base *coverage, needBase bool) (*coverage, error) {
	content, origin := l.Content, l.Origin
	// A buffer from Load belongs to canvas, so it can be written to in place
	own := false
	if content == nil && l.Load != nil {
		var err error
		if content, origin, err = l.Load(); err != nil {
			return nil, fmt.Errorf("load: %w", err)
		}
		if content != nil && !wellFormed(content) {
			return nil, errors.New("load: returned a buffer whose pixels do not match its size")
		}
		own = true
	}
	if content == nil && l.Load == nil {
		img := l.Image
		if img == nil && l.LoadImage != nil {
			var err error
			if img, origin, err = l.LoadImage(); err != nil {
				return nil, fmt.Errorf("load: %w", err)
			}
		}
		if img != nil && !img.Bounds().Empty() {
			// A layer that is only blended needs nothing from its image beyond
			// reading it, so it skips the buffer that a mask, a clip or an
			// effect would need to write to
			blendOnly := l.Mask == nil && len(l.Effects) == 0 && !needBase && !(l.ClipToBelow && base != nil)
			if ix, ok := img.(*blend.Indexed); ok {
				if blendOnly {
					blend.CompositeIndexed(backdrop, ix, origin, l.Mode, opacityOr(l.Opacity))
					return nil, nil
				}
				img = ix.NRGBA
			}
			if n, ok := img.(*image.NRGBA); ok && blendOnly {
				blend.CompositeNRGBA(backdrop, n, origin, l.Mode, opacityOr(l.Opacity))
				return nil, nil
			}
			var err error
			if content, err = raster.FromImage(img); err != nil {
				return nil, fmt.Errorf("load: %w", err)
			}
			own = true
		}
	}
	if content == nil {
		return base, nil
	}
	doc := image.Rect(0, 0, backdrop.Width, backdrop.Height)
	if rect := bufferRect(content, origin); !rect.In(doc) {
		// Only the part inside the document takes part, as if it were placed there
		rect = rect.Intersect(doc)
		if rect.Empty() {
			return layerBase(nil, origin, base, l.ClipToBelow && base != nil, needBase), nil
		}
		content, origin, own = window(content, origin, rect), rect.Min, true
	}

	clipped := l.ClipToBelow && base != nil
	// Only masks and clipping write to the content, so the caller's buffer is copied just for those
	if (l.Mask != nil || clipped) && !own {
		content, own = content.Clone(), true
	}
	if l.Mask != nil {
		mask.ApplyAt(content, l.Mask, origin)
	}
	if clipped {
		multiplyCoverage(content, origin, base)
	}
	next := layerBase(content, origin, base, clipped, needBase)

	op := opacityOr(l.Opacity)
	sorted := effects.Sort(l.Effects)
	if len(sorted) == 0 {
		blend.CompositeRect(backdrop, content, origin, l.Mode, op)
		return next, nil
	}

	// Effects run on a buffer that covers the content plus whatever they can
	// reach. One that is not Bounded may write anywhere, so it gets the document
	bleed, bounded := effects.MaxBleed(sorted)
	scratchRect := bufferRect(content, origin)
	if bounded {
		scratchRect = scratchRect.Inset(-bleed).Intersect(doc)
	} else {
		scratchRect = doc
	}
	scratch := content
	if scratchRect != bufferRect(content, origin) {
		scratch = window(content, origin, scratchRect)
	}
	var all []effects.Rendered
	for _, e := range sorted {
		all = append(all, e.Render(scratch)...)
	}

	// Behind effects, then the layer, then front effects
	at := scratchRect.Min
	for _, r := range all {
		if r.Behind {
			blend.CompositeRect(backdrop, r.Pixels, at, r.Mode, r.Opacity*op)
		}
	}
	blend.CompositeRect(backdrop, scratch, at, l.Mode, op)
	for _, r := range all {
		if !r.Behind {
			blend.CompositeRect(backdrop, r.Pixels, at, r.Mode, r.Opacity*op)
		}
	}
	return next, nil
}

// wellFormed reports whether a buffer has positive dimensions and the pixel
// count they imply, which a buffer from raster always does
func wellFormed(b *raster.Buffer) bool {
	return b.Width > 0 && b.Height > 0 && len(b.Pix) == b.Width*b.Height*4
}

// layerBase returns the coverage a clip-to-below layer after this one attaches
// to. A clipped layer keeps the base it was clipped to. Any other layer becomes
// the base itself, and a layer with no visible content is an empty one
func layerBase(content *raster.Buffer, origin image.Point, base *coverage, clipped, needBase bool) *coverage {
	if clipped {
		return base
	}
	if !needBase {
		return nil
	}
	if content == nil {
		return &coverage{}
	}
	return extractCoverage(content, origin)
}

// bufferRect is the document rectangle a buffer covers when placed at origin
func bufferRect(b *raster.Buffer, origin image.Point) image.Rectangle {
	return image.Rect(origin.X, origin.Y, origin.X+b.Width, origin.Y+b.Height)
}

// window returns a new buffer covering rect, holding the pixels of src (placed
// at origin) that fall inside it and transparent elsewhere
func window(src *raster.Buffer, origin image.Point, rect image.Rectangle) *raster.Buffer {
	dst := raster.MustNewBuffer(rect.Dx(), rect.Dy())
	r := rect.Intersect(bufferRect(src, origin))
	for y := r.Min.Y; y < r.Max.Y; y++ {
		si := ((y-origin.Y)*src.Width + r.Min.X - origin.X) * 4
		di := ((y-rect.Min.Y)*dst.Width + r.Min.X - rect.Min.X) * 4
		copy(dst.Pix[di:di+r.Dx()*4], src.Pix[si:si+r.Dx()*4])
	}
	return dst
}

// multiplyCoverage scales a premultiplied buffer placed at origin by a coverage
// field, the clip-to-below operation. Pixels outside the field become zero
func multiplyCoverage(b *raster.Buffer, origin image.Point, cov *coverage) {
	cw := cov.rect.Dx()
	for y := 0; y < b.Height; y++ {
		for x := 0; x < b.Width; x++ {
			p := image.Pt(origin.X+x, origin.Y+y)
			var c float32
			if p.In(cov.rect) {
				c = cov.alpha[(p.Y-cov.rect.Min.Y)*cw+p.X-cov.rect.Min.X]
			}
			j := (y*b.Width + x) * 4
			b.Pix[j] *= c
			b.Pix[j+1] *= c
			b.Pix[j+2] *= c
			b.Pix[j+3] *= c
		}
	}
}

// extractCoverage copies a buffer's alpha channel along with where it sits
func extractCoverage(b *raster.Buffer, origin image.Point) *coverage {
	out := make([]float32, b.Width*b.Height)
	for i := range out {
		out[i] = b.Pix[i*4+3]
	}
	return &coverage{rect: bufferRect(b, origin), alpha: out}
}

// opacityOr defaults a non-positive opacity to fully opaque and clamps above one
func opacityOr(o float32) float32 {
	if o <= 0 {
		return 1
	}
	if o > 1 {
		return 1
	}
	return o
}
