package httpapi

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"
)

func TestNormalizeImageCaption(t *testing.T) {
	caption, err := normalizeImageCaption("  подпись к фото  ")
	if err != nil || caption != "подпись к фото" {
		t.Fatalf("got %q, %v", caption, err)
	}
	if caption, err = normalizeImageCaption(" \t "); err != nil || caption != "" {
		t.Fatalf("empty caption got %q, %v", caption, err)
	}
	if _, err = normalizeImageCaption(strings.Repeat("я", maxImageCaption+1)); err == nil {
		t.Fatal("caption above the character limit was accepted")
	}
}

func TestNormalizeMessageImage(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 32, 24))
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	out, err := normalizeMessageImage(b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(out))
	if err != nil || format != "jpeg" || cfg.Width != 32 {
		t.Fatalf("normalized %s %+v %v", format, cfg, err)
	}
	b.Reset()
	_ = jpeg.Encode(&b, img, nil)
	if _, err = normalizeMessageImage(b.Bytes()); err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]byte{nil, []byte("not an image"), make([]byte, maxMessageImage+1)} {
		if _, err = normalizeMessageImage(bad); err == nil {
			t.Fatal("invalid image accepted")
		}
	}
	b.Reset()
	_ = png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 4097, 1)))
	if _, err = normalizeMessageImage(b.Bytes()); err == nil {
		t.Fatal("oversized dimensions accepted")
	}
}
