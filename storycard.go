package goinsta

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"

	xdraw "golang.org/x/image/draw"
)

// A phone-shaped story with the post card at most 75% wide and 60% tall,
// close to how the app lays out a reshared post.
const (
	storyW, storyH = 1080, 1920
	cardMaxW       = 0.75 * storyW
	cardMaxH       = 0.6 * storyH
	cardRadius     = 32
)

// storyCard is a rendered reshare: the image to upload and the card's size as
// a fraction of the frame, which becomes the sticker's tap area.
type storyCard struct {
	photo *photo
	w, h  float64
}

// renderStoryCard centers the post image, with rounded corners, on a vertical
// gradient of its own top and bottom colors.
func renderStoryCard(post []byte) (*storyCard, error) {
	src, _, err := image.Decode(bytes.NewReader(post))
	if err != nil {
		return nil, fmt.Errorf("story card: %w", err)
	}
	b := src.Bounds()
	scale := min(cardMaxW/float64(b.Dx()), cardMaxH/float64(b.Dy()))
	cw, ch := max(1, int(float64(b.Dx())*scale)), max(1, int(float64(b.Dy())*scale))

	frame := image.NewRGBA(image.Rect(0, 0, storyW, storyH))
	third := b.Dy() / 3
	top := averageColor(src, image.Rect(b.Min.X, b.Min.Y, b.Max.X, b.Min.Y+third))
	bottom := averageColor(src, image.Rect(b.Min.X, b.Max.Y-third, b.Max.X, b.Max.Y))
	for y := range storyH {
		row := image.NewUniform(blend(top, bottom, float64(y)/(storyH-1)))
		draw.Draw(frame, image.Rect(0, y, storyW, y+1), row, image.Point{}, draw.Src)
	}

	card := image.NewRGBA(image.Rect(0, 0, cw, ch))
	xdraw.CatmullRom.Scale(card, card.Bounds(), src, b, draw.Src, nil)
	at := image.Rect(0, 0, cw, ch).Add(image.Pt((storyW-cw)/2, (storyH-ch)/2))
	draw.DrawMask(frame, at, card, image.Point{}, roundedRect{cw, ch, cardRadius}, image.Point{}, draw.Over)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, frame, &jpeg.Options{Quality: 90}); err != nil {
		return nil, err
	}
	return &storyCard{
		photo: &photo{data: buf.Bytes(), mime: "image/jpeg", w: storyW, h: storyH},
		w:     float64(cw) / storyW,
		h:     float64(ch) / storyH,
	}, nil
}

// averageColor samples about 64x64 points of r, darkened so the card stands out.
func averageColor(img image.Image, r image.Rectangle) color.RGBA {
	stepX, stepY := max(1, r.Dx()/64), max(1, r.Dy()/64)
	var sr, sg, sb, n uint64
	for y := r.Min.Y; y < r.Max.Y; y += stepY {
		for x := r.Min.X; x < r.Max.X; x += stepX {
			cr, cg, cb, _ := img.At(x, y).RGBA()
			sr, sg, sb, n = sr+uint64(cr>>8), sg+uint64(cg>>8), sb+uint64(cb>>8), n+1
		}
	}
	if n == 0 {
		return color.RGBA{A: 255}
	}
	const dim = 6 // tenths of the average kept
	return color.RGBA{uint8(sr * dim / 10 / n), uint8(sg * dim / 10 / n), uint8(sb * dim / 10 / n), 255}
}

func blend(a, b color.RGBA, t float64) color.RGBA {
	mix := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*t) }
	return color.RGBA{mix(a.R, b.R), mix(a.G, b.G), mix(a.B, b.B), 255}
}

// roundedRect is an alpha mask of a w x h rectangle with corners of radius r.
type roundedRect struct{ w, h, r int }

func (m roundedRect) ColorModel() color.Model { return color.AlphaModel }
func (m roundedRect) Bounds() image.Rectangle { return image.Rect(0, 0, m.w, m.h) }
func (m roundedRect) At(x, y int) color.Color {
	cx := min(max(x, m.r), m.w-1-m.r)
	cy := min(max(y, m.r), m.h-1-m.r)
	if dx, dy := x-cx, y-cy; dx*dx+dy*dy > m.r*m.r {
		return color.Alpha{}
	}
	return color.Alpha{A: 255}
}
