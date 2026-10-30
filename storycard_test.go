package goinsta

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

func TestRenderStoryCard(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 1000, 500))
	for y := range 500 {
		for x := range 1000 {
			src.Set(x, y, color.RGBA{220, 40, 40, 255})
		}
	}
	var in bytes.Buffer
	if err := jpeg.Encode(&in, src, nil); err != nil {
		t.Fatal(err)
	}
	card, err := renderStoryCard(in.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	// 1000x500 is width-bound: 810x405.
	if card.w != 0.75 || card.h != 405.0/1920 {
		t.Errorf("card = %vx%v", card.w, card.h)
	}
	out, err := jpeg.Decode(bytes.NewReader(card.photo.data))
	if err != nil {
		t.Fatal(err)
	}
	if b := out.Bounds(); b.Dx() != 1080 || b.Dy() != 1920 || card.photo.w != 1080 || card.photo.h != 1920 {
		t.Fatalf("frame = %v", out.Bounds())
	}
	near := func(c color.Color, r, g, b uint8) bool {
		cr, cg, cb, _ := c.RGBA()
		d := func(x uint32, y uint8) bool { return int(x>>8)-int(y) < 12 && int(y)-int(x>>8) < 12 }
		return d(cr, r) && d(cg, g) && d(cb, b)
	}
	if c := out.At(540, 960); !near(c, 220, 40, 40) {
		t.Errorf("card center = %v, want the post color", c)
	}
	// Background is the post color at 60%; the card's corner pixel is cut away.
	if c := out.At(10, 10); !near(c, 132, 24, 24) {
		t.Errorf("background = %v", c)
	}
	if c := out.At(135, 960-202); !near(c, 132, 24, 24) {
		t.Errorf("rounded corner = %v, want background", c)
	}
}
