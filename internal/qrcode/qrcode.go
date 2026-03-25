// Package qrcode generates QR codes in two output formats: SVG for embedding
// in the web dashboard, and Unicode block characters for display in the
// terminal. Both formats encode a URL so that mobile devices can scan the code
// to open a locally shared service. The package wraps the go-qrcode library,
// using medium error correction for a good balance between density and
// scan reliability.
package qrcode

import (
	"bytes"
	"fmt"
	"strings"

	goqrcode "github.com/skip2/go-qrcode"
)

func newBitmap(url string) ([][]bool, error) {
	if url == "" {
		return nil, fmt.Errorf("URL must not be empty")
	}
	qr, err := goqrcode.New(url, goqrcode.Medium)
	if err != nil {
		return nil, fmt.Errorf("generating QR code: %w", err)
	}
	return qr.Bitmap(), nil
}

// SVG generates a self-contained SVG string containing a QR code for the given
// URL. The SVG uses a white background with black modules and includes a 16px
// margin around the code. Each module is rendered as a 4x4 pixel rectangle.
// The output is suitable for embedding directly in HTML (e.g., the dashboard's
// share modal). Returns an error if the URL is empty or cannot be encoded.
func SVG(url string) (string, error) {
	bitmap, err := newBitmap(url)
	if err != nil {
		return "", err
	}
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

// Terminal generates a QR code as a string for terminal display using Unicode
// block characters (half-blocks). Each character represents two vertical pixels
// by combining upper-half and lower-half block characters, which halves the
// vertical space needed. This makes the QR code compact enough to fit in a
// typical terminal window. The output includes a trailing newline on each row.
// Returns an error if the URL is empty or cannot be encoded.
func Terminal(url string) (string, error) {
	bitmap, err := newBitmap(url)
	if err != nil {
		return "", err
	}
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
