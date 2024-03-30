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
	// OCSpecHeadingSearchN is the number of previous siblings in the
	// MarkDown AST to search for OCSpecHeading in order to establish
	// whether the yaml block might be the OC path and RPC listing.
	OCSpecHeadingSearchN = 5
)

type mdOCSpecs struct{}

// MDOCSpecs is an extension that only renders the first yaml block from a
// functional test README that comes after the pre-established OC Spec heading
// `OCSpecHeading`.
var MDOCSpecs = &mdOCSpecs{}

func (e *mdOCSpecs) Extend(m goldmark.Markdown) {
	extension.GFM.Extend(m)
	m.SetRenderer(&YAMLRenderer{})
}

type YAMLRenderer struct{}

func (r *YAMLRenderer) Render(w io.Writer, source []byte, n ast.Node) error {
	return ast.Walk(n, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		return yamlRenderer(w, source, n, entering)
	})
}

func (r *YAMLRenderer) AddOptions(...renderer.Option) {}

func yamlCodeBlock(source []byte, n ast.Node) *ast.FencedCodeBlock {
	if s, ok := n.(*ast.FencedCodeBlock); ok && s.Info != nil {
		if lang := s.Info.Text(source); len(lang) > 0 && string(lang) == "yaml" {
			return s
		}
	}
	return nil
}

func yamlRenderer(w io.Writer, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	if s := yamlCodeBlock(source, n); s != nil {
		// Check if within the last `OCSpecHeadingSearchN` siblings,
		// the "OC Path and RPC Coverage" heading can be found, and
		// that there is no intervening yaml block.
		yamlBlockIsOCSpec := false
		prev := s.PreviousSibling()
		for i := 0; i != OCSpecHeadingSearchN && prev != nil; i++ {
			if yamlCodeBlock(source, prev) != nil {
				break
			}
			if h, ok := prev.(*ast.Heading); ok {
				if h.Lines().Len() > 0 {
					headingSegment := h.Lines().At(0)
					if string(headingSegment.Value(source)) == OCSpecHeading {
						yamlBlockIsOCSpec = true
						break
					}
				}
			}
			prev = prev.PreviousSibling()
		}
		if !yamlBlockIsOCSpec {
			return ast.WalkContinue, nil
		}

		l := s.Lines().Len()
		for i := 0; i != l; i++ {
			line := s.Lines().At(i)
			if _, err := w.Write(line.Value(source)); err != nil {
				return ast.WalkStop, err
			}
		}
		// Stop after finding the first.
		return ast.WalkStop, nil
	}
	return ast.WalkContinue, nil
}
