// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"strings"
	"text/template"
)

var badgeTpl = template.Must(template.New("badgeTpl").Parse(`<svg xmlns='http://www.w3.org/2000/svg' width='{{.Width}}' height='20' role="img">
<linearGradient id='a' x2='0' y2='100%'>
  <stop offset='0' stop-color='#bbb' stop-opacity='.1'/>
  <stop offset='1' stop-opacity='.1'/>
</linearGradient>
<clipPath id="r">
  <rect width="{{.Width}}" height="20" rx="3" fill="#fff"></rect>
</clipPath>
<g clip-path="url(#r)">
  <rect width="{{.LabelWidth}}" height="20" fill="#555"></rect>
  <rect x="{{.LabelWidth}}" width="{{.MessageWidth}}" height="20" fill="{{.Color}}"></rect>
  <rect width="{{.Width}}" height="20" fill="url(#a)"></rect>
</g>
<g fill='#fff' text-anchor='middle' font-family='DejaVu Sans,Verdana,Geneva,sans-serif' font-size='11'>
  <text x='{{.LabelAnchor}}' y='15' fill='#010101' fill-opacity='.3'>
	{{.Label}}
  </text>
  <text x='{{.LabelAnchor}}' y='14'>
	{{.Label}}
  </text>
  <text x='{{.MessageAnchor}}' y='15' fill='#010101' fill-opacity='.3'>
	{{.Message}}
  </text>
  <text x='{{.MessageAnchor}}' y='14'>
	{{.Message}}
  </text>
</g>
</svg>`))

// svgBadge returns an SVG image with the label identifying the purpose of the
// badge and the message providing any details on the badge status.  The
// messages "success" and "failure" are given unique color codes, while other
// messages are displayed in a neutral color.
func svgBadge(label, message string) (*bytes.Buffer, error) {
	var badge struct {
		Color         string
		Label         string
		LabelAnchor   int
		LabelWidth    int
		Message       string
		MessageAnchor int
		MessageWidth  int
		Width         int
	}

	badge.Label = strings.ToLower(label)
	badge.Message = strings.ToLower(message)

	switch badge.Message {
	case "success":
		badge.Color = "#4C1"
	case "failure":
		badge.Color = "#E05D44"
	default:
		badge.Color = "#9F9F9F"
	}

	badge.LabelWidth = estimateStringWidth(badge.Label)
	if badge.LabelWidth < 80 {
		badge.LabelWidth = 80
	}
	badge.LabelAnchor = badge.LabelWidth / 2
	badge.MessageWidth = estimateStringWidth(badge.Message)
	if badge.MessageWidth < 200 {
		badge.MessageWidth = 200
	}
	badge.MessageAnchor = badge.LabelWidth + (badge.MessageWidth / 2)
	badge.Width = badge.LabelWidth + badge.MessageWidth

	var buf bytes.Buffer
	err := badgeTpl.Execute(&buf, badge)
	return &buf, err
}

// estimateStringWidth determines the length of s. The character size is always
// 7px; this is not accurate for variable width fonts.
func estimateStringWidth(s string) int {
	const (
		padding     = 10
		avgCharSize = 7
	)
	return padding + (avgCharSize * len(s))
}
