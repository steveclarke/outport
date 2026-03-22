package qrcode

import (
	"strings"
	"testing"
)

func TestSVG_ValidURL(t *testing.T) {
	svg, err := SVG("http://192.168.1.50:18472")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(svg, "<svg") {
		t.Errorf("expected SVG output, got: %.50s", svg)
	}
	if !strings.Contains(svg, "</svg>") {
		t.Error("SVG output not closed")
	}
}

func TestSVG_EmptyURL(t *testing.T) {
	_, err := SVG("")
	if err == nil {
		t.Error("expected error for empty URL")
	}
}

func TestTerminal_ValidURL(t *testing.T) {
	output, err := Terminal("http://192.168.1.50:18472")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(output) == 0 {
		t.Error("expected non-empty terminal output")
	}
}
