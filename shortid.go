package goinsta

import (
	"fmt"
	"strconv"
	"strings"
)

const base64UrlCharmap = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"

// MediaIDFromShortID decodes a post shortcode (the /p/<code>/ part of a URL).
func MediaIDFromShortID(code string) (string, error) {
	if code == "" || len(code) > 11 {
		return "", fmt.Errorf("shortcode %q: want 1 to 11 characters", code)
	}
	var id uint64
	for i := 0; i < len(code); i++ {
		v := strings.IndexByte(base64UrlCharmap, code[i])
		if v < 0 {
			return "", fmt.Errorf("shortcode %q: invalid character %q", code, code[i])
		}
		if id > (1<<64-1)>>6 {
			return "", fmt.Errorf("shortcode %q overflows a media id", code)
		}
		id = id<<6 | uint64(v)
	}
	return strconv.FormatUint(id, 10), nil
}
