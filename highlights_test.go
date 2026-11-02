package goinsta

import (
	"errors"
	"testing"
)

func TestCoverCrop(t *testing.T) {
	cover := &Item{OriginalWidth: 1080, OriginalHeight: 1920}
	for _, tc := range []struct {
		crop []float64
		want string
	}{
		{nil, "[0.0,0.21875,1.0,0.78125]"},
		{[]float64{0, 0, 1, 0.5625}, "[0.0,0.0,1.0,0.5625]"},
	} {
		if got, err := coverCrop(cover, tc.crop); err != nil || got != tc.want {
			t.Errorf("coverCrop(%v) = %q, %v; want %q", tc.crop, got, err, tc.want)
		}
	}
	for _, bad := range [][]float64{{0, 0, 1}, {-0.1, 0, 1, 0.5}, {0, 0, 1, 1.2}, {0, 0.5, 1, 0.5}, {1, 0, 0, 1}} {
		if _, err := coverCrop(cover, bad); !errors.Is(err, ErrCoverCrop) {
			t.Errorf("coverCrop(%v) = %v, want ErrCoverCrop", bad, err)
		}
	}
}

func TestHighlightCoverCropIsCheckedBeforeAnyRequest(t *testing.T) {
	insta := &Instagram{}
	bad := []float64{0, 0.6, 1, 0.4}
	if _, err := insta.CreateHighlight(t.Context(), "a", HighlightOptions{CoverCrop: bad}, &Item{ID: "1_2"}); !errors.Is(err, ErrCoverCrop) {
		t.Errorf("CreateHighlight = %v, want ErrCoverCrop", err)
	}
	reel := &Reel{insta: insta}
	if err := reel.Edit(t.Context(), HighlightEdit{Cover: &Item{ID: "1_2"}, CoverCrop: bad}); !errors.Is(err, ErrCoverCrop) {
		t.Errorf("Edit = %v, want ErrCoverCrop", err)
	}
}
