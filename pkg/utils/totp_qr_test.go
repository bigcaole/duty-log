package utils

import (
	"strings"
	"testing"
)

func TestGenerateOTPQRCodeDataURL(t *testing.T) {
	url := "otpauth://totp/Duty-Log-System:admin%40example.com?algorithm=SHA1&digits=6&issuer=Duty-Log-System&period=30&secret=JBSWY3DPEHPK3PXP"
	dataURL, err := GenerateOTPQRCodeDataURL(url, 240)
	if err != nil {
		t.Fatalf("GenerateOTPQRCodeDataURL failed: %v", err)
	}
	if !strings.HasPrefix(dataURL, "data:image/png;base64,") {
		t.Fatalf("unexpected data url prefix: %s", dataURL)
	}
	if len(dataURL) <= len("data:image/png;base64,") {
		t.Fatalf("expected non-empty base64 payload")
	}
}

func TestGenerateOTPQRCodeDataURLEmptyInput(t *testing.T) {
	if _, err := GenerateOTPQRCodeDataURL("  ", 240); err == nil {
		t.Fatalf("expected empty input to fail")
	}
}
