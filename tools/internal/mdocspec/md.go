package mdocspec

import (
	"io"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer"
)

const (
	// OCSpecHeading is the MarkDown heading that MUST precede the yaml
	// section containing the OC path and RPC listing.
	OCSpecHeading = "OpenConfig Path and RPC Coverage"
)

type mdOCSpecs struct{}

// MDOCSpecs is an extension that only renders the first yaml block from a
// functional test README that comes after the pre-established OC Spec heading
// `OCSpecHeading`.
var MDOCSpecs = &mdOCSpecs{}

func (e *mdOCSpecs) Extend(m goldmark.Markdown) {
	extension.GFM.Extend(m)
	m.SetRenderer(&yamlRenderer{})
}

type yamlRenderer struct{}

func (r *yamlRenderer) Render(w io.Writer, source []byte, n ast.Node) error {
	return ast.Walk(n, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		return renderYAML(w, source, n, entering)
	})
}

func (r *yamlRenderer) AddOptions(...renderer.Option) {}

func ocSpecHeading(source []byte, n ast.Node) (heading *ast.Heading, ok bool) {
	if h, ok := n.(*ast.Heading); ok {
		headingSegment := h.Lines().At(0)
		if string(headingSegment.Value(source)) == OCSpecHeading {
			return h, true
		}
	}
	return nil, false
}

func yamlCodeBlock(source []byte, n ast.Node) (block *ast.FencedCodeBlock, ok bool) {
	if c, ok := n.(*ast.FencedCodeBlock); ok && c.Info != nil {
		if lang := c.Info.Text(source); len(lang) > 0 && string(lang) == "yaml" {
			return c, true
		}
	}
	return nil, false
}

func renderYAML(w io.Writer, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	heading, ok := ocSpecHeading(source, n)
	if !ok {
		return ast.WalkContinue, nil
	}
	// Check if prior to the next heading of the same level,
	// a yaml code block can be found.
	for next := heading.NextSibling(); next != nil; next = next.NextSibling() {
		if h, ok := next.(*ast.Heading); ok && h.Level <= heading.Level {
			// End of heading reached.
			return ast.WalkContinue, nil
		}
		if c, ok := yamlCodeBlock(source, next); ok {
			l := c.Lines().Len()
			for i := 0; i != l; i++ {
				line := c.Lines().At(i)
				if _, err := w.Write(line.Value(source)); err != nil {
					return ast.WalkStop, err
				}
			}
			// Stop after finding the first such YAML block.
			return ast.WalkStop, nil
		}
	}
	return ast.WalkContinue, nil
}
