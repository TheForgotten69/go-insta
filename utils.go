package goinsta

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"strconv"
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
func createUserAgent(device Device) string {
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

func MergeMapS(one map[string]string, extra ...map[string]string) map[string]string {
	for _, e := range extra {
		for k, v := range e {
			one[k] = v
		}
	}
	return one
}
