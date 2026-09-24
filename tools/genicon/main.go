// Command genicon renders the app icon to icon.png: a document with a music
// note being edited by a pencil (in the spirit of Font Awesome's "file-pen"),
// drawn in white on a violet macOS-style rounded square.
package main

import (
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
)

const size = 1024

func main() {
	out := "icon.png"
	if len(os.Args) > 1 {
		out = os.Args[1]
	}
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	const ss = 4 // supersampling per axis
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var r, g, b, a float64
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					px := float64(x) + (float64(sx)+0.5)/ss
					py := float64(y) + (float64(sy)+0.5)/ss
					if c, ok := sample(px, py); ok {
						r, g, b, a = r+c[0], g+c[1], b+c[2], a+1
					}
				}
			}
			if a == 0 {
				continue
			}
			img.SetNRGBA(x, y, color.NRGBA{
				R: uint8(r / a), G: uint8(g / a), B: uint8(b / a),
				A: uint8(a / (ss * ss) * 255),
			})
		}
	}
	f, err := os.Create(out)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		log.Fatal(err)
	}
}

// sample returns the colour at p, or false if p is outside the icon.
func sample(x, y float64) ([3]float64, bool) {
	if !inRoundRect(x, y, 100, 100, 924, 924, 185) {
		return [3]float64{}, false
	}
	// Diagonal violet gradient.
	t := (x + y) / (2 * size)
	bg := [3]float64{lerp(0x9a, 0x55, t), lerp(0x7c, 0x33, t), lerp(0xff, 0xe0, t)}
	white := [3]float64{255, 255, 255}

	// Shift the artwork so the page and pencil together are centred.
	x += 25

	pu, pv := pencilCoords(x, y)
	switch {
	case inPencil(pu, pv):
		return white, true
	case inPencilHalo(pu, pv):
		return bg, true
	case inFold(x, y):
		return white, true
	case inDoc(x, y) && !inDocCutout(x, y):
		return white, true
	}
	return bg, true
}

func lerp(a, b, t float64) float64 { return a + (b-a)*t }

func mix(a, b [3]float64, t float64) [3]float64 {
	return [3]float64{lerp(a[0], b[0], t), lerp(a[1], b[1], t), lerp(a[2], b[2], t)}
}

func inRoundRect(x, y, x0, y0, x1, y1, rad float64) bool {
	if x < x0 || x > x1 || y < y0 || y > y1 {
		return false
	}
	cx := math.Max(math.Max(x0+rad-x, x-(x1-rad)), 0)
	cy := math.Max(math.Max(y0+rad-y, y-(y1-rad)), 0)
	return cx*cx+cy*cy <= rad*rad
}

// ---- Document ----

const (
	docX0, docY0, docX1, docY1 float64 = 215, 190, 635, 830
	docRad                     float64 = 44
	fold                       float64 = 140 // size of the folded top-right corner
	foldGap                    float64 = 22  // gap between the page and the fold flap
)

func inDoc(x, y float64) bool {
	if !inRoundRect(x, y, docX0, docY0, docX1, docY1, docRad) {
		return false
	}
	// Leave out the top-right corner, where the fold flap sits.
	return x < docX1-fold-foldGap || y > docY0+fold+foldGap
}

// inFold is the triangular flap in the top-right corner, with its right
// angle facing the page and its long edge facing outward.
func inFold(x, y float64) bool {
	fx, fy := docX1-fold, docY0+fold
	return x >= fx && y <= fy && x-fx <= y-docY0
}

// inDocCutout is the artwork punched out of the page: a note and two text lines.
func inDocCutout(x, y float64) bool {
	if inNote(x, y, 390, 480, 0.42) {
		return true
	}
	return inRoundRect(x, y, 290, 640, 470, 680, 20) ||
		inRoundRect(x, y, 290, 725, 410, 765, 20)
}

// ---- Beamed eighth note, defined around (515, 500) at scale 1 ----

var heads = [2][2]float64{{385, 700}, {645, 640}}

const (
	headRx, headRy = 78, 58
	headTilt       = -0.38 // radians
	stemW          = 34
	beamThick      = 78
	stemTop0       = 300
	stemTop1       = 240
)

func stemX(i int) float64 { return heads[i][0] + headRx*0.82 }

// inNote reports whether p is inside the note drawn centred at (cx, cy).
func inNote(x, y, cx, cy, scale float64) bool {
	x = 515 + (x-cx)/scale
	y = 500 + (y-cy)/scale
	for _, h := range heads {
		dx, dy := x-h[0], y-h[1]
		c, s := math.Cos(headTilt), math.Sin(headTilt)
		u, v := dx*c+dy*s, -dx*s+dy*c
		if u*u/(headRx*headRx)+v*v/(headRy*headRy) <= 1 {
			return true
		}
	}
	x0, x1 := stemX(0)-stemW, stemX(1)
	if x < x0 || x > x1 {
		return false
	}
	top := stemTop0 + (stemTop1-stemTop0)*(x-x0)/(x1-x0)
	if y >= top && y <= top+beamThick {
		return true
	}
	for i, h := range heads {
		sx := stemX(i)
		if x >= sx-stemW && x <= sx && y >= top && y <= h[1] {
			return true
		}
	}
	return false
}

// ---- Pencil, lying diagonally with its tip pointing down-left ----

var (
	pencilStart = [2]float64{835, 395} // eraser end
	pencilTip   = [2]float64{520, 710}
)

const (
	pencilW  float64 = 118 // body width
	eraserL  float64 = 64  // eraser length
	tipL     float64 = 130 // tip length
	band     float64 = 20  // gaps between eraser/body/tip
	haloSize float64 = 26  // clearance cut into the page around the pencil
)

func pencilLen() float64 {
	return math.Hypot(pencilTip[0]-pencilStart[0], pencilTip[1]-pencilStart[1])
}

// pencilCoords maps p to (u along the pencil from the eraser, v across it).
func pencilCoords(x, y float64) (u, v float64) {
	l := pencilLen()
	dx, dy := (pencilTip[0]-pencilStart[0])/l, (pencilTip[1]-pencilStart[1])/l
	px, py := x-pencilStart[0], y-pencilStart[1]
	return px*dx + py*dy, -px*dy + py*dx
}

// tipHalfWidth is the half width of the tapered tip at u.
func tipHalfWidth(u float64) float64 {
	return pencilW / 2 * (pencilLen() - u) / tipL
}

func inPencil(u, v float64) bool {
	l := pencilLen()
	hw := pencilW / 2
	switch {
	case u >= 0 && u <= eraserL: // eraser, rounded at the end
		if u < hw*0.5 {
			r := hw * 0.5
			cu := r - u
			cv := math.Max(math.Abs(v)-(hw-r), 0)
			return cu*cu+cv*cv <= r*r
		}
		return math.Abs(v) <= hw
	case u >= eraserL+band && u <= l-tipL: // body
		return math.Abs(v) <= hw
	case u >= l-tipL+band && u <= l: // tip
		return math.Abs(v) <= tipHalfWidth(u)
	}
	return false
}

func inPencilHalo(u, v float64) bool {
	l := pencilLen()
	if u < -haloSize || u > l+haloSize {
		return false
	}
	if u <= l-tipL {
		return math.Abs(v) <= pencilW/2+haloSize
	}
	// Offset the tip's edges outward by haloSize.
	slope := (pencilW / 2) / tipL
	return math.Abs(v) <= tipHalfWidth(u)+haloSize*math.Sqrt(1+slope*slope)
}
