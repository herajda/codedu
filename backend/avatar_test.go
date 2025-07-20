package main

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

func createDataURL() string {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for x := 0; x < 100; x++ {
		for y := 0; y < 100; y++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 0, 255})
		}
	}
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestResizeAvatar(t *testing.T) {
	data := createDataURL()
	out, err := resizeAvatar(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	parts := strings.SplitN(out, ",", 2)
	if len(parts) != 2 {
		t.Fatalf("invalid output")
	}
	b, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	img, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("decode resized: %v", err)
	}
	if img.Bounds().Dx() != 256 || img.Bounds().Dy() != 256 {
		t.Fatalf("expected 256x256, got %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
	}
}
