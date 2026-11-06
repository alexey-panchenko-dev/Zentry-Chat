package httpapi

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"testing"
)

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
