package goinsta

import (
	"encoding/binary"
	"iter"
)

type mp4Info struct {
	w, h       int
	durationMs int64
}

// parseMP4 reads the display size of the first video track and the movie
// duration from an MP4's moov box.
func parseMP4(b []byte) (mp4Info, error) {
	var info mp4Info
	if len(b) < 12 || string(b[4:8]) != "ftyp" {
		return info, ErrInvalidVideo
	}
	moov := child(b, "moov")
	if mvhd := child(moov, "mvhd"); len(mvhd) >= 32 {
		var timescale, duration uint64
		if mvhd[0] == 1 {
			timescale, duration = uint64(binary.BigEndian.Uint32(mvhd[20:])), binary.BigEndian.Uint64(mvhd[24:])
		} else {
			timescale, duration = uint64(binary.BigEndian.Uint32(mvhd[12:])), uint64(binary.BigEndian.Uint32(mvhd[16:]))
		}
		if timescale > 0 {
			info.durationMs = int64(duration * 1000 / timescale)
		}
	}
	for typ, trak := range boxes(moov) {
		if typ != "trak" {
			continue
		}
		hdlr := child(child(trak, "mdia"), "hdlr")
		if len(hdlr) < 12 || string(hdlr[8:12]) != "vide" {
			continue
		}
		info.w, info.h = trackSize(child(trak, "tkhd"))
		break
	}
	if info.w <= 0 || info.h <= 0 || info.durationMs <= 0 {
		return info, ErrInvalidVideo
	}
	return info, nil
}

// trackSize is the tkhd width and height (16.16 fixed point), swapped when the
// matrix rotates by 90 or 270 degrees.
func trackSize(tkhd []byte) (w, h int) {
	matrix := 40
	if len(tkhd) > 0 && tkhd[0] == 1 {
		matrix = 52
	}
	if len(tkhd) < matrix+44 {
		return 0, 0
	}
	w = int(binary.BigEndian.Uint32(tkhd[matrix+36:]) >> 16)
	h = int(binary.BigEndian.Uint32(tkhd[matrix+40:]) >> 16)
	a, b := int32(binary.BigEndian.Uint32(tkhd[matrix:])), int32(binary.BigEndian.Uint32(tkhd[matrix+4:]))
	if a == 0 && b != 0 {
		w, h = h, w
	}
	return w, h
}

// boxes walks the ISO-BMFF boxes in b, yielding each type and payload.
func boxes(b []byte) iter.Seq2[string, []byte] {
	return func(yield func(string, []byte) bool) {
		for len(b) >= 8 {
			size, hdr := uint64(binary.BigEndian.Uint32(b)), uint64(8)
			typ := string(b[4:8])
			switch size {
			case 0:
				size = uint64(len(b))
			case 1:
				if len(b) < 16 {
					return
				}
				size, hdr = binary.BigEndian.Uint64(b[8:]), 16
			}
			if size < hdr || size > uint64(len(b)) {
				return
			}
			if !yield(typ, b[hdr:size]) {
				return
			}
			b = b[size:]
		}
	}
}

func child(b []byte, typ string) []byte {
	for t, p := range boxes(b) {
		if t == typ {
			return p
		}
	}
	return nil
}
