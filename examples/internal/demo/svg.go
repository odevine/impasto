package demo

import (
	"fmt"
	"strconv"

	"github.com/odevine/impasto/path"
)

// SVGPath parses an SVG path "d" attribute into a path.Path, mapping SVG user
// units into pixel space as x*scale+tx, y*scale+ty. SVG and impasto both put
// the origin at the top left with y growing downward, so no flip is needed.
//
// It covers every command a font-to-SVG export emits: move, line, horizontal,
// vertical, cubic, quadratic, their smooth forms, and close. Elliptical arcs
// are rejected rather than approximated
func SVGPath(d string, scale, tx, ty float32) (*path.Path, error) {
	l := &svgLexer{s: d}
	p := path.New()

	// Current point, subpath start, and the previous control point that the
	// smooth commands reflect
	var curX, curY, startX, startY, ctrlX, ctrlY float32
	var cmd, prev byte

	px := func(x float32) float32 { return x*scale + tx }
	py := func(y float32) float32 { return y*scale + ty }

	for {
		l.skipSep()
		if l.done() {
			break
		}

		if c, ok := l.command(); ok {
			cmd = c
		} else if cmd == 0 {
			return nil, fmt.Errorf("svg path: data does not begin with a command")
		} else if cmd == 'M' {
			// A repeated coordinate pair after a moveto is an implicit lineto
			cmd = 'L'
		} else if cmd == 'm' {
			cmd = 'l'
		}

		rel := cmd >= 'a' && cmd <= 'z'
		ox, oy := float32(0), float32(0)
		if rel {
			ox, oy = curX, curY
		}

		switch upper(cmd) {
		case 'M':
			x, y, err := l.pair()
			if err != nil {
				return nil, err
			}
			curX, curY = ox+x, oy+y
			startX, startY = curX, curY
			p.MoveTo(px(curX), py(curY))

		case 'L':
			x, y, err := l.pair()
			if err != nil {
				return nil, err
			}
			curX, curY = ox+x, oy+y
			p.LineTo(px(curX), py(curY))

		case 'H':
			x, err := l.number()
			if err != nil {
				return nil, err
			}
			curX = ox + x
			p.LineTo(px(curX), py(curY))

		case 'V':
			y, err := l.number()
			if err != nil {
				return nil, err
			}
			curY = oy + y
			p.LineTo(px(curX), py(curY))

		case 'C':
			x1, y1, x2, y2, x, y, err := l.six()
			if err != nil {
				return nil, err
			}
			c1x, c1y := ox+x1, oy+y1
			c2x, c2y := ox+x2, oy+y2
			curX, curY = ox+x, oy+y
			p.CubicTo(px(c1x), py(c1y), px(c2x), py(c2y), px(curX), py(curY))
			ctrlX, ctrlY = c2x, c2y

		case 'S':
			x2, y2, x, y, err := l.quad()
			if err != nil {
				return nil, err
			}
			c1x, c1y := curX, curY
			if u := upper(prev); u == 'C' || u == 'S' {
				c1x, c1y = 2*curX-ctrlX, 2*curY-ctrlY
			}
			c2x, c2y := ox+x2, oy+y2
			curX, curY = ox+x, oy+y
			p.CubicTo(px(c1x), py(c1y), px(c2x), py(c2y), px(curX), py(curY))
			ctrlX, ctrlY = c2x, c2y

		case 'Q':
			x1, y1, x, y, err := l.quad()
			if err != nil {
				return nil, err
			}
			cx, cy := ox+x1, oy+y1
			curX, curY = ox+x, oy+y
			p.QuadTo(px(cx), py(cy), px(curX), py(curY))
			ctrlX, ctrlY = cx, cy

		case 'T':
			x, y, err := l.pair()
			if err != nil {
				return nil, err
			}
			cx, cy := curX, curY
			if u := upper(prev); u == 'Q' || u == 'T' {
				cx, cy = 2*curX-ctrlX, 2*curY-ctrlY
			}
			curX, curY = ox+x, oy+y
			p.QuadTo(px(cx), py(cy), px(curX), py(curY))
			ctrlX, ctrlY = cx, cy

		case 'Z':
			p.Close()
			curX, curY = startX, startY

		case 'A':
			return nil, fmt.Errorf("svg path: elliptical arcs are not supported")

		default:
			return nil, fmt.Errorf("svg path: unknown command %q", string(cmd))
		}
		prev = cmd
	}
	return p, nil
}

// svgLexer walks a path data string, yielding commands and numbers. SVG lets
// separators be omitted wherever the parse stays unambiguous, so numbers are
// scanned by shape rather than split on delimiters
type svgLexer struct {
	s string
	i int
}

func (l *svgLexer) done() bool { return l.i >= len(l.s) }

func (l *svgLexer) skipSep() {
	for l.i < len(l.s) {
		switch l.s[l.i] {
		case ' ', ',', '\t', '\n', '\r':
			l.i++
		default:
			return
		}
	}
}

// command consumes and returns the next byte if it is a command letter
func (l *svgLexer) command() (byte, bool) {
	l.skipSep()
	if l.done() {
		return 0, false
	}
	c := l.s[l.i]
	if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
		l.i++
		return c, true
	}
	return 0, false
}

// number scans one float. Only the first dot belongs to the token, so the
// unseparated pair "1.5.5" reads as 1.5 then .5 the way SVG requires
func (l *svgLexer) number() (float32, error) {
	l.skipSep()
	start := l.i
	if !l.done() && (l.s[l.i] == '+' || l.s[l.i] == '-') {
		l.i++
	}
	dot := false
	for !l.done() {
		c := l.s[l.i]
		if c >= '0' && c <= '9' {
			l.i++
			continue
		}
		if c == '.' && !dot {
			dot = true
			l.i++
			continue
		}
		break
	}
	if !l.done() && (l.s[l.i] == 'e' || l.s[l.i] == 'E') {
		l.i++
		if !l.done() && (l.s[l.i] == '+' || l.s[l.i] == '-') {
			l.i++
		}
		for !l.done() && l.s[l.i] >= '0' && l.s[l.i] <= '9' {
			l.i++
		}
	}
	if start == l.i {
		return 0, fmt.Errorf("svg path: expected a number at offset %d", start)
	}
	v, err := strconv.ParseFloat(l.s[start:l.i], 32)
	if err != nil {
		return 0, fmt.Errorf("svg path: bad number %q: %w", l.s[start:l.i], err)
	}
	return float32(v), nil
}

func (l *svgLexer) pair() (a, b float32, err error) {
	if a, err = l.number(); err != nil {
		return
	}
	b, err = l.number()
	return
}

func (l *svgLexer) quad() (a, b, c, d float32, err error) {
	if a, b, err = l.pair(); err != nil {
		return
	}
	c, d, err = l.pair()
	return
}

func (l *svgLexer) six() (a, b, c, d, e, f float32, err error) {
	if a, b, c, d, err = l.quad(); err != nil {
		return
	}
	e, f, err = l.pair()
	return
}

func upper(c byte) byte {
	if c >= 'a' && c <= 'z' {
		return c - 32
	}
	return c
}
