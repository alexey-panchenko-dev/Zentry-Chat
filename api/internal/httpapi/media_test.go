package httpapi

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"testing"
)

func TestOptimizedDetailedAvatarFitsStoredPNG(t *testing.T) {
	photo := image.NewRGBA(image.Rect(0, 0, 512, 512))
	seed := uint32(42)
	for i := 0; i < len(photo.Pix); i += 4 {
		for channel := 0; channel < 3; channel++ {
			seed = seed*1664525 + 1013904223
			photo.Pix[i+channel] = byte(seed >> 24)
		}
		photo.Pix[i+3] = 255
	}
	var uploaded bytes.Buffer
	if err := jpeg.Encode(&uploaded, photo, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatal(err)
	}
	stored, mime, err := normalizePhoto(uploaded.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if mime != "image/png" || len(stored) == 0 || len(stored) > 2*1024*1024 {
		t.Fatalf("normalized avatar: MIME=%s size=%d", mime, len(stored))
	}
}

func TestAvatarRenditionsHaveFixedMaximumDimensions(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 1024, 1024))
	seed := uint32(7)
	for i := 0; i < len(source.Pix); i += 4 {
		seed = seed*1664525 + 1013904223
		source.Pix[i] = byte(seed >> 24)
		seed = seed*1664525 + 1013904223
		source.Pix[i+1] = byte(seed >> 24)
		seed = seed*1664525 + 1013904223
		source.Pix[i+2] = byte(seed >> 24)
		source.Pix[i+3] = 255
	}
	var original bytes.Buffer
	if err := png.Encode(&original, source); err != nil {
		t.Fatal(err)
	}
	for _, size := range []int{512, 128} {
		variant, err := avatarRendition(original.Bytes(), size)
		if err != nil {
			t.Fatal(err)
		}
		config, _, err := image.DecodeConfig(bytes.NewReader(variant))
		if err != nil {
			t.Fatal(err)
		}
		if config.Width > size || config.Height > size {
			t.Fatalf("%d rendition dimensions=%dx%d", size, config.Width, config.Height)
		}
		if len(variant) == 0 {
			t.Fatalf("empty %d rendition", size)
		}
	}
}

func TestMediaValidation(t *testing.T) {
	for _, v := range []struct {
		mime string
		b    []byte
		ok   bool
	}{{"audio/webm", []byte{0x1a, 0x45, 0xdf, 0xa3, 1}, false}, {"audio/ogg", []byte("OggSx"), false}, {"audio/mp4", []byte("xxxxftypisom"), false}, {"audio/webm", []byte("garbage"), false}, {"text/html", []byte("OggSx"), false}} {
		if (validateAudio(v.mime, v.b) == nil) != v.ok {
			t.Errorf("audio %s", v.mime)
		}
	}
	var b bytes.Buffer
	png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	if _, _, err := normalizePhoto(b.Bytes()); err != nil {
		t.Fatal(err)
	}
	if _, _, err := normalizePhoto([]byte("<svg/>")); err == nil {
		t.Fatal("accepted SVG")
	}
	if _, _, err := normalizePhoto(make([]byte, 2*1024*1024+1)); err == nil {
		t.Fatal("accepted oversized photo")
	}
}

func audioFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func TestEncodedAudioContainers(t *testing.T) {
	for _, f := range []struct{ name, mime string }{{"silence.webm", "audio/webm"}, {"silence.ogg", "audio/ogg"}, {"silence.mp4", "audio/mp4"}, {"silence-fragmented.mp4", "audio/mp4"}} {
		t.Run(f.name, func(t *testing.T) {
			b := audioFixture(t, f.name)
			if err := validateAudio(f.mime, b); err != nil {
				t.Fatalf("encoded fixture rejected: %v", err)
			}
			for _, n := range []int{0, 4, 12, len(b) / 2} {
				if validateAudio(f.mime, b[:n]) == nil {
					t.Errorf("accepted truncation at %d", n)
				}
			}
		})
	}
}

func ebmlTestElement(id uint64, data []byte) []byte {
	var idBytes [8]byte
	binary.BigEndian.PutUint64(idBytes[:], id)
	start := 0
	for idBytes[start] == 0 {
		start++
	}
	size := uint64(len(data))
	n := 1
	for size >= (uint64(1)<<(7*n))-1 {
		n++
	}
	var sizeBytes [8]byte
	binary.BigEndian.PutUint64(sizeBytes[:], size)
	sizeBytes[8-n] |= 1 << uint(8-n)
	out := append([]byte(nil), idBytes[start:]...)
	out = append(out, sizeBytes[8-n:]...)
	return append(out, data...)
}
func webmTestVariant(t *testing.T, b []byte, mode string) []byte {
	t.Helper()
	roots, e := ebmlChildren(b, true)
	if e != nil {
		t.Fatal(e)
	}
	var out []byte
	for _, root := range roots {
		if root.id != 0x18538067 {
			out = append(out, ebmlTestElement(root.id, root.data)...)
			continue
		}
		children, e := ebmlChildren(root.data, true)
		if e != nil {
			t.Fatal(e)
		}
		var segment []byte
		for _, c := range children {
			if mode == "no-audio" && c.id == 0x1654ae6b {
				continue
			}
			if mode == "no-blocks" && c.id == 0x1f43b675 {
				continue
			}
			if mode == "duration" && c.id == 0x1549a966 {
				v, e := ebmlChildren(c.data, false)
				if e != nil {
					t.Fatal(e)
				}
				c.data = nil
				for _, x := range v {
					if x.id == 0x4489 {
						var f [8]byte
						binary.BigEndian.PutUint64(f[:], math.Float64bits(120001))
						x.data = f[:]
					}
					c.data = append(c.data, ebmlTestElement(x.id, x.data)...)
				}
			}
			encoded := ebmlTestElement(c.id, c.data)
			if mode == "unknown" && c.id == 0x1f43b675 {
				encoded = append([]byte{0x1f, 0x43, 0xb6, 0x75, 1, 255, 255, 255, 255, 255, 255, 255}, c.data...)
			}
			segment = append(segment, encoded...)
		}
		if mode == "unknown" {
			out = append(out, []byte{0x18, 0x53, 0x80, 0x67, 1, 255, 255, 255, 255, 255, 255, 255}...)
			out = append(out, segment...)
		} else {
			out = append(out, ebmlTestElement(root.id, segment)...)
		}
	}
	return out
}
func TestWebMTrackPayloadAndUnknownLengths(t *testing.T) {
	b := audioFixture(t, "silence.webm")
	for _, mode := range []string{"no-audio", "no-blocks", "duration"} {
		if validateAudio("audio/webm", webmTestVariant(t, b, mode)) == nil {
			t.Fatalf("accepted %s", mode)
		}
	}
	if err := validateAudio("audio/webm", webmTestVariant(t, b, "unknown")); err != nil {
		t.Fatalf("unknown Segment/Cluster rejected: %v", err)
	}
}
func TestAudioDeclaredBoundsAndDurations(t *testing.T) {
	b := audioFixture(t, "silence.ogg")
	last := bytes.LastIndex(b, []byte("OggS"))
	page := b[last:]
	binary.LittleEndian.PutUint64(page[6:14], 121*48000)
	for i := 22; i < 26; i++ {
		page[i] = 0
	}
	var crc uint32
	for _, v := range page {
		crc ^= uint32(v) << 24
		for j := 0; j < 8; j++ {
			if crc&0x80000000 != 0 {
				crc = (crc << 1) ^ 0x04c11db7
			} else {
				crc <<= 1
			}
		}
	}
	binary.LittleEndian.PutUint32(page[22:26], crc)
	if validateAudio("audio/ogg", b) == nil {
		t.Fatal("accepted overlong Ogg granule")
	}
	b = audioFixture(t, "silence.mp4")
	idx := bytes.Index(b, []byte("mdhd"))
	if idx < 0 {
		t.Fatal("missing mdhd")
	}
	p := b[idx+4:]
	scale := u32(p[12:16])
	binary.BigEndian.PutUint32(p[16:20], scale*121)
	if validateAudio("audio/mp4", b) == nil {
		t.Fatal("accepted overlong MP4 duration")
	}
	for _, name := range []string{"silence.mp4", "silence-fragmented.mp4"} {
		b = audioFixture(t, name)
		idx = bytes.Index(b, []byte("mdat"))
		binary.BigEndian.PutUint32(b[idx-4:idx], uint32(len(b)+100))
		if validateAudio("audio/mp4", b) == nil {
			t.Fatal("accepted invalid MP4 box extent")
		}
	}
	b = audioFixture(t, "silence.ogg")
	b[len(b)-1] ^= 1
	if validateAudio("audio/ogg", b) == nil {
		t.Fatal("accepted corrupt Ogg checksum")
	}
}
func TestWebMManyStreamingUnknownClusters(t *testing.T) {
	b := audioFixture(t, "silence.webm")
	roots, e := ebmlChildren(b, true)
	if e != nil {
		t.Fatal(e)
	}
	var result []byte
	for _, r := range roots {
		if r.id != 0x18538067 {
			result = append(result, ebmlTestElement(r.id, r.data)...)
			continue
		}
		items, e := ebmlChildren(r.data, true)
		if e != nil {
			t.Fatal(e)
		}
		var segment []byte
		for _, item := range items {
			if item.id != 0x1f43b675 {
				segment = append(segment, ebmlTestElement(item.id, item.data)...)
				continue
			}
			vals, e := ebmlChildren(item.data, false)
			if e != nil {
				t.Fatal(e)
			}
			for i := 0; i < 8; i++ {
				var payload []byte
				for _, v := range vals {
					if v.id == 0xe7 {
						var ts [2]byte
						binary.BigEndian.PutUint16(ts[:], uint16(i*250))
						v.data = ts[:]
					}
					payload = append(payload, ebmlTestElement(v.id, v.data)...)
				}
				segment = append(segment, []byte{0x1f, 0x43, 0xb6, 0x75, 1, 255, 255, 255, 255, 255, 255, 255}...)
				segment = append(segment, payload...)
			}
		}
		result = append(result, []byte{0x18, 0x53, 0x80, 0x67, 1, 255, 255, 255, 255, 255, 255, 255}...)
		result = append(result, segment...)
	}
	if err := validateAudio("audio/webm", result); err != nil {
		t.Fatalf("eight legal streaming Clusters rejected: %v", err)
	}
}

func TestProfilePhotoCenterCrop(t *testing.T) {
	for _, size := range []image.Point{{X: 7, Y: 12}, {X: 12, Y: 7}} {
		for _, format := range []string{"png", "jpeg"} {
			t.Run(fmt.Sprintf("%s-%dx%d", format, size.X, size.Y), func(t *testing.T) {
				photo := image.NewNRGBA(image.Rect(0, 0, size.X, size.Y))
				for y := 0; y < size.Y; y++ {
					for x := 0; x < size.X; x++ {
						photo.SetNRGBA(x, y, color.NRGBA{R: uint8(15 * x), G: uint8(15 * y), B: 90, A: 255})
					}
				}
				var input bytes.Buffer
				var err error
				if format == "png" {
					err = png.Encode(&input, photo)
				} else {
					err = jpeg.Encode(&input, photo, nil)
				}
				if err != nil {
					t.Fatal(err)
				}
				original, _, err := image.Decode(bytes.NewReader(input.Bytes()))
				if err != nil {
					t.Fatal(err)
				}
				stored, mime, err := normalizePhoto(input.Bytes())
				if err != nil {
					t.Fatal(err)
				}
				cropped, err := png.Decode(bytes.NewReader(stored))
				if err != nil {
					t.Fatal(err)
				}
				side := min(size.X, size.Y)
				if mime != "image/png" || cropped.Bounds().Dx() != side || cropped.Bounds().Dy() != side {
					t.Fatalf("expected square %d got %v %s", side, cropped.Bounds(), mime)
				}
				origin := image.Pt((size.X-side)/2, (size.Y-side)/2)
				for _, p := range []image.Point{{X: 0, Y: 0}, {X: side - 1, Y: side - 1}} {
					want := color.NRGBAModel.Convert(original.At(p.X+origin.X, p.Y+origin.Y))
					got := color.NRGBAModel.Convert(cropped.At(p.X, p.Y))
					if got != want {
						t.Fatalf("crop pixel %v got %v want %v", p, got, want)
					}
				}
			})
		}
	}
}
func TestProfilePhotoCropPreservesTransparencyAndDimensionLimit(t *testing.T) {
	photo := image.NewNRGBA(image.Rect(0, 0, 2, 4))
	photo.SetNRGBA(0, 1, color.NRGBA{R: 80, G: 120, B: 160, A: 64})
	var input bytes.Buffer
	if err := png.Encode(&input, photo); err != nil {
		t.Fatal(err)
	}
	b, _, err := normalizePhoto(input.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	cropped, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if cropped.Bounds() != image.Rect(0, 0, 2, 2) || color.NRGBAModel.Convert(cropped.At(0, 0)) != photo.NRGBAAt(0, 1) {
		t.Fatalf("transparent crop changed bounds/pixel %v %v", cropped.Bounds(), cropped.At(0, 0))
	}
	input.Reset()
	if err = png.Encode(&input, image.NewNRGBA(image.Rect(0, 0, 4097, 1))); err != nil {
		t.Fatal(err)
	}
	if _, _, err = normalizePhoto(input.Bytes()); err == nil {
		t.Fatal("oversized dimensions must be rejected before crop")
	}
}
