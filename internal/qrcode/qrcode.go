package qrcode

import (
	"bytes"
	"fmt"
	"strings"

	goqrcode "github.com/skip2/go-qrcode"
)

// SVG generates an SVG string containing a QR code for the given URL.
func SVG(url string) (string, error) {
	if url == "" {
		return "", fmt.Errorf("URL must not be empty")
	}
	qr, err := goqrcode.New(url, goqrcode.Medium)
	if err != nil {
		return "", fmt.Errorf("generating QR code: %w", err)
	}
	bitmap := qr.Bitmap()
	size := len(bitmap)
	moduleSize := 4
	margin := 16
	totalSize := size*moduleSize + margin*2

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="%d">`, totalSize, totalSize, totalSize, totalSize)
	fmt.Fprintf(&b, `<rect width="%d" height="%d" fill="white"/>`, totalSize, totalSize)
	for y, row := range bitmap {
		for x, cell := range row {
			if cell {
				fmt.Fprintf(&b, `<rect x="%d" y="%d" width="%d" height="%d" fill="black"/>`,
					margin+x*moduleSize, margin+y*moduleSize, moduleSize, moduleSize)
			}
		}
	}
	b.WriteString("</svg>")
	return b.String(), nil
}

// Terminal generates a QR code as a string for terminal display using
// Unicode block characters. Returns the rendered string.
func Terminal(url string) (string, error) {
	if url == "" {
		return "", fmt.Errorf("URL must not be empty")
	}
	qr, err := goqrcode.New(url, goqrcode.Medium)
	if err != nil {
		return "", fmt.Errorf("generating QR code: %w", err)
	}
	bitmap := qr.Bitmap()
	var buf bytes.Buffer
	for y := 0; y < len(bitmap); y += 2 {
		for x := 0; x < len(bitmap[0]); x++ {
			top := bitmap[y][x]
			bottom := false
			if y+1 < len(bitmap) {
				bottom = bitmap[y+1][x]
			}
			switch {
			case top && bottom:
				buf.WriteRune('\u2588') // full block
			case top && !bottom:
				buf.WriteRune('\u2580') // upper half
			case !top && bottom:
				buf.WriteRune('\u2584') // lower half
			default:
				buf.WriteRune(' ')
			}
		}
		buf.WriteRune('\n')
	}
	return buf.String(), nil
}
