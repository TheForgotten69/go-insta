package goinsta

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	"strconv"
	"strings"
	"time"

	_ "golang.org/x/image/webp"
)

func getImageSize(b []byte) (int, int, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil {
		return 0, 0, err
	}
	return cfg.Width, cfg.Height, nil
}

func getTimeOffset() string {
	_, offset := time.Now().Zone()
	return strconv.Itoa(offset)
}

// createUserAgent formats e.g.
// Instagram 453.0.0.55.79 Android (33/13; 243dpi; 3839x2160; Waydroid/waydroid; WayDroid x86_64 Device; waydroid_x86_64; unknown; en_US; 1100455565)
func createUserAgent(device Device, locale string) string {
	return fmt.Sprintf("Instagram %s Android (%d/%d; %s; %s; %s; %s; %s; %s; %s; %s)",
		appVersion,
		device.AndroidVersion,
		device.AndroidRelease,
		device.ScreenDpi,
		device.ScreenResolution,
		device.Manufacturer,
		device.Model,
		device.CodeName,
		device.Chipset,
		locale,
		appVersionCode,
	)
}

func parseUserAgent(ua string) (Device, bool) {
	_, rest, ok := strings.Cut(ua, " Android (")
	if !ok {
		return Device{}, false
	}
	f := strings.Split(strings.TrimSuffix(rest, ")"), "; ")
	if len(f) != 9 {
		return Device{}, false
	}
	av, ar, ok := strings.Cut(f[0], "/")
	if !ok {
		return Device{}, false
	}
	ver, err1 := strconv.Atoi(av)
	rel, err2 := strconv.Atoi(ar)
	if err1 != nil || err2 != nil {
		return Device{}, false
	}
	return Device{
		AndroidVersion: ver, AndroidRelease: rel, ScreenDpi: f[1], ScreenResolution: f[2],
		Manufacturer: f[3], Model: f[4], CodeName: f[5], Chipset: f[6],
	}, true
}
