package main

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

var (
	backgroundColor = color.NRGBA{R: 200, G: 200, B: 200, A: 255}
)

type Rect struct {
	Color color.NRGBA
	Size  f32.Point
	Radii float32
}

// Layout renders the Rect into the provided context
func (r Rect) Layout(gtx C) D {
	paint.FillShape(gtx.Ops, r.Color, clip.UniformRRect(image.Rectangle{Max: r.Size.Round()}, int(r.Radii)).Op(gtx.Ops))
	return layout.Dimensions{Size: image.Pt(int(r.Size.X), int(r.Size.Y))}
}
