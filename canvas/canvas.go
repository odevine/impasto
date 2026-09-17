// Package canvas is the orchestration layer most callers import. It assembles
// layers and groups into a document and flattens the whole stack to a single
// buffer. Layers carry content, opacity, a blend mode, an optional mask, and a
// list of non-destructive effects, groups nest layers with pass-through or
// isolated blending. Everything below this package is usable on its own for
// callers who only need, say, the blend math.
//
// Layer content buffers are document-sized. A caller with a smaller image places
// it into a document-sized buffer first with Place, so the model needs no
// per-layer offset and effects can reach anywhere on the canvas.
package canvas

import (
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
// document-sized. Opacity in (0,1] scales the layer and its effects together, a
// non-positive Opacity is treated as fully opaque. Effects are applied in a fixed
// order regardless of slice order
type Layer struct {
	Content     *raster.Buffer
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
// returns an error only if the document dimensions are invalid
func Render(doc *Document) (*raster.Buffer, error) {
	out, err := raster.NewBuffer(doc.Width, doc.Height)
	if err != nil {
		return nil, err
	}
	renderGroup(&doc.Root, out)
	return out, nil
}

// MustRender is Render for trusted sizes, panicking on an invalid dimension
func MustRender(doc *Document) *raster.Buffer {
	out, err := Render(doc)
	if err != nil {
		panic(err)
	}
	return out
}

// renderGroup composites a group's children onto backdrop
func renderGroup(g *Group, backdrop *raster.Buffer) {
	isolated := !g.PassThrough || g.Opacity > 0 && g.Opacity < 1 || g.Mask != nil || g.Mode != blend.Normal
	if !isolated {
		renderChildren(g.Layers, backdrop)
		return
	}
	inner := raster.MustNewBuffer(backdrop.Width, backdrop.Height)
	renderChildren(g.Layers, inner)
	if g.Mask != nil {
		mask.Apply(inner, g.Mask)
	}
	blend.Composite(backdrop, inner, g.Mode, opacityOr(g.Opacity))
}

// renderChildren composites a sequence of nodes onto backdrop, tracking the clip
// base so clip-to-below layers attach to the layer beneath them
func renderChildren(nodes []Node, backdrop *raster.Buffer) {
	var clipBase []float32
	for _, n := range nodes {
		switch node := n.(type) {
		case *Layer:
			clipBase = renderLayer(node, backdrop, clipBase)
		case *Group:
			renderGroup(node, backdrop)
			// A nested group does not serve as a clip base
			clipBase = nil
		}
	}
}

// renderLayer composites one layer with its effects onto backdrop, and returns
// the alpha field a following clip-to-below layer should clip against
func renderLayer(l *Layer, backdrop *raster.Buffer, clipBase []float32) []float32 {
	if l.Content == nil {
		return clipBase
	}
	// Never mutate the caller's content, effects and masks work on a copy
	content := l.Content.Clone()
	if l.Mask != nil {
		mask.Apply(content, l.Mask)
	}

	nextBase := clipBase
	if l.ClipToBelow && clipBase != nil {
		multiplyAlpha(content, clipBase)
	} else {
		// This layer becomes the base that subsequent clip layers attach to
		nextBase = extractAlpha(content)
	}

	op := opacityOr(l.Opacity)
	sorted := effects.Sort(l.Effects)
	var all []effects.Rendered
	for _, e := range sorted {
		all = append(all, e.Render(content)...)
	}

	// Behind effects, then the layer, then front effects
	for _, r := range all {
		if r.Behind {
			blend.Composite(backdrop, r.Pixels, r.Mode, r.Opacity*op)
		}
	}
	blend.Composite(backdrop, content, l.Mode, op)
	for _, r := range all {
		if !r.Behind {
			blend.Composite(backdrop, r.Pixels, r.Mode, r.Opacity*op)
		}
	}
	return nextBase
}

// multiplyAlpha scales a premultiplied buffer by a coverage field, the clip-to
// -below operation
func multiplyAlpha(b *raster.Buffer, cov []float32) {
	for i := 0; i < len(cov); i++ {
		c := cov[i]
		j := i * 4
		b.Pix[j] *= c
		b.Pix[j+1] *= c
		b.Pix[j+2] *= c
		b.Pix[j+3] *= c
	}
}

// extractAlpha copies a buffer's alpha channel
func extractAlpha(b *raster.Buffer) []float32 {
	out := make([]float32, b.Width*b.Height)
	for i := range out {
		out[i] = b.Pix[i*4+3]
	}
	return out
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
