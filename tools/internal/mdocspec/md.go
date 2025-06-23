// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	// CanonicalOCHeading is the MarkDown heading that MUST precede the json
	// section containing the OC path and RPC listing.
	CanonicalOCHeading = "Canonical OC"
	// yamlLang is the language identifier for yaml code blocks.
	yamlLang = "yaml"
	// jsonLang is the language identifier for json code blocks.
	jsonLang = "json"
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

type mdJSONSpecs struct {
	CanonicalOCs []string
}

// MDJSONSpecs is an extension that only renders the first json block from a
// functional test README that comes after the pre-established OC Spec heading
// `CanonicalOCHeading`.
var MDJSONSpecs = &mdJSONSpecs{}

func (e *mdJSONSpecs) Extend(m goldmark.Markdown) {
	// Clear the CanonicalOCs in the shared MDJSONSpecs when a new Markdown is created.
	MDJSONSpecs.CanonicalOCs = MDJSONSpecs.CanonicalOCs[:0]
	// NOMUTANTS -- This function call is required to extend the markdown and is actually tested.
	extension.GFM.Extend(m)
	m.SetRenderer(&jsonRenderer{})
}

type yamlRenderer struct{}

type jsonRenderer struct{}

type codeBlockHandler func(c *ast.FencedCodeBlock, writer io.Writer, source []byte) (ast.WalkStatus, error)

type renderCodeBlockArgs struct {
	writer           io.Writer
	source           []byte
	node             ast.Node
	entering         bool
	specHeading      string
	lang             string
	processCodeBlock codeBlockHandler
}

func processYAMLBlock(c *ast.FencedCodeBlock, writer io.Writer, source []byte) (ast.WalkStatus, error) {
	l := c.Lines().Len()
	for i := 0; i != l; i++ {
		line := c.Lines().At(i)
		if _, err := writer.Write(line.Value(source)); err != nil {
			return ast.WalkStop, err
		}
	}
	return ast.WalkStop, nil
}

func (r *yamlRenderer) Render(writer io.Writer, source []byte, node ast.Node) error {
	return ast.Walk(node, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		return renderCodeBlock(renderCodeBlockArgs{writer, source, node, entering, OCSpecHeading, yamlLang, processYAMLBlock})
	})
}

func processJSONBlock(c *ast.FencedCodeBlock, _ io.Writer, source []byte) (ast.WalkStatus, error) {
	var curBytes []byte
	l := c.Lines().Len()
	for i := 0; i != l; i++ {
		line := c.Lines().At(i)
		curBytes = append(curBytes, line.Value(source)...)
	}
	if len(curBytes) != 0 {
		MDJSONSpecs.CanonicalOCs = append(MDJSONSpecs.CanonicalOCs, string(curBytes))
	}
	return ast.WalkContinue, nil
}

func (r *jsonRenderer) Render(writer io.Writer, source []byte, node ast.Node) error {
	return ast.Walk(node, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		return renderCodeBlock(renderCodeBlockArgs{writer, source, node, entering, CanonicalOCHeading, jsonLang, processJSONBlock})
	})
}

func (r *yamlRenderer) AddOptions(...renderer.Option) {}

func (r *jsonRenderer) AddOptions(...renderer.Option) {}

func fetchHeading(source []byte, node ast.Node, specHeading string) (heading *ast.Heading, ok bool) {
	if h, ok := node.(*ast.Heading); ok {
		if h.Lines().Len() == 0 {
			return nil, false
		}
		headingSegment := h.Lines().At(0)
		if string(headingSegment.Value(source)) == specHeading {
			return h, true
		}
	}
	return nil, false
}

func codeBlock(source []byte, node ast.Node, lang string) (block *ast.FencedCodeBlock, ok bool) {
	if c, ok := node.(*ast.FencedCodeBlock); ok && c.Info != nil {
		if l := c.Info.Text(source); len(l) > 0 && string(l) == lang {
			return c, true
		}
	}
	return nil, false
}

func renderCodeBlock(args renderCodeBlockArgs) (ast.WalkStatus, error) {
	entering := args.entering
	if !entering {
		return ast.WalkContinue, nil
	}
	heading, ok := fetchHeading(args.source, args.node, args.specHeading)
	if !ok {
		return ast.WalkContinue, nil
	}
	// Check if prior to the next heading of the same level,
	// a code block with the specified language can be found.
	for next := heading.NextSibling(); next != nil; next = next.NextSibling() {
		if h, ok := next.(*ast.Heading); ok && h.Level <= heading.Level {
			// End of heading reached.
			return ast.WalkContinue, nil
		}
		if c, ok := codeBlock(args.source, next, args.lang); ok {
			return args.processCodeBlock(c, args.writer, args.source)
		}
	}
	return ast.WalkContinue, nil
}
