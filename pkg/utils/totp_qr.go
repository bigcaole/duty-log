package utils

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image/png"
	"strings"

	"github.com/pquerna/otp"
)

// GenerateOTPQRCodeDataURL creates a PNG QR image for otpauth URL and returns it as data URL.
func GenerateOTPQRCodeDataURL(otpAuthURL string, size int) (string, error) {
	rawURL := strings.TrimSpace(otpAuthURL)
	if rawURL == "" {
		return "", fmt.Errorf("otpauth url is empty")
	}
	if size <= 0 {
		size = 240
	}

	key, err := otp.NewKeyFromURL(rawURL)
	if err != nil {
		return "", err
	}
	img, err := key.Image(size, size)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
