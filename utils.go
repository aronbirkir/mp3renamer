package main

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

func rgb(c uint32) color.NRGBA {
	return color.NRGBA{R: uint8(c >> 16), G: uint8(c >> 8), B: uint8(c), A: 0xff}
}

// rounded returns a widget that fills its minimum constraints with a rounded rectangle.
// Use it as the background of a layout.Background.
func rounded(c color.NRGBA, radius unit.Dp) layout.Widget {
	return func(gtx C) D {
		size := gtx.Constraints.Min
		defer clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(radius)).Push(gtx.Ops).Pop()
		paint.Fill(gtx.Ops, c)
		return D{Size: size}
	}
}

// fill paints the whole available area.
func fill(gtx C, c color.NRGBA) D {
	size := gtx.Constraints.Max
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	paint.Fill(gtx.Ops, c)
	return D{Size: size}
}

// divider draws a 1dp horizontal line across the available width.
func divider(gtx C) D {
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(1))
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	paint.Fill(gtx.Ops, pal.Border)
	return D{Size: size}
}

func vspace(dp unit.Dp) layout.FlexChild {
	return layout.Rigid(layout.Spacer{Height: dp}.Layout)
}

func hspace(dp unit.Dp) layout.FlexChild {
	return layout.Rigid(layout.Spacer{Width: dp}.Layout)
}
