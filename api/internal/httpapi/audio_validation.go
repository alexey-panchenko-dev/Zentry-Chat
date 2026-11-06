package httpapi

// Structural validation keeps truncated or header-only uploads out of storage.
// It does not decode codec payloads or attest to their perceptual/playback quality.
// References: RFC 7845 (Ogg Opus), Matroska element specification, ISO BMFF.
import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
)

var errAudioContainer = errors.New("invalid or unsupported audio container")

func opusPacketSamples(p []byte) (int, error) {
	if len(p) < 2 {
		return 0, errAudioContainer
	}
	config := p[0] >> 3
	var frameSamples int
	if config >= 16 {
		frameSamples = 120 << (config & 3)
	} else if config >= 12 {
		frameSamples = 480 << (config & 1)
	} else {
		frameSamples = []int{480, 960, 1920, 2880}[config&3]
	}
	count, offset, end := 1, 1, len(p)
	readLength := func() (int, bool) {
		if offset >= end {
			return 0, false
		}
		n := int(p[offset])
		offset++
		if n >= 252 {
			if offset >= end {
				return 0, false
			}
			n += 4 * int(p[offset])
			offset++
		}
		return n, n > 0 && n <= 1275
	}
	switch p[0] & 3 {
	case 0:
		if end-offset > 1275 {
			return 0, errAudioContainer
		}
	case 1:
		count = 2
		if (end-offset)%2 != 0 || (end-offset)/2 > 1275 {
			return 0, errAudioContainer
		}
	case 2:
		count = 2
		first, ok := readLength()
		if !ok || end-offset-first < 1 || end-offset-first > 1275 {
			return 0, errAudioContainer
		}
	case 3:
		control := p[1]
		offset = 2
		count = int(control & 63)
		if count == 0 || count > 48 {
			return 0, errAudioContainer
		}
		if control&64 != 0 {
			padding := 0
			for {
				if offset >= end {
					return 0, errAudioContainer
				}
				n := int(p[offset])
				offset++
				if n == 255 {
					padding += 254
				} else {
					padding += n
					break
				}
			}
			end -= padding
			if end < offset {
				return 0, errAudioContainer
			}
		}
		if control&128 != 0 {
			used := 0
			for i := 0; i < count-1; i++ {
				n, ok := readLength()
				if !ok {
					return 0, errAudioContainer
				}
				used += n
			}
			last := end - offset - used
			if last < 1 || last > 1275 {
				return 0, errAudioContainer
			}
		} else if (end-offset)%count != 0 || (end-offset)/count > 1275 {
			return 0, errAudioContainer
		}
	}
	if end-offset < count || count*frameSamples > 5760 {
		return 0, errAudioContainer
	}
	return count * frameSamples, nil
}
func validOpusHead(p []byte) bool {
	return len(p) >= 19 && bytes.Equal(p[:8], []byte("OpusHead")) && p[8] <= 15 && p[9] > 0 && p[9] <= 2 && p[18] == 0 && len(p) == 19
}
func validateOgg(b []byte) error {
	var serial, sequence uint32
	var packet []byte
	packets := 0
	totalSamples := 0
	seenEnd := false
	var preSkip uint64
	for off := 0; off < len(b); {
		if seenEnd || len(b)-off < 27 || string(b[off:off+4]) != "OggS" || b[off+4] != 0 {
			return errAudioContainer
		}
		n := int(b[off+26])
		if len(b)-off < 27+n {
			return errAudioContainer
		}
		end := off + 27 + n
		for _, v := range b[off+27 : end] {
			end += int(v)
		}
		if end > len(b) {
			return errAudioContainer
		}
		page := b[off:end]
		flags := page[5]
		if flags&^byte(7) != 0 {
			return errAudioContainer
		}
		if off == 0 {
			if flags&2 == 0 || flags&1 != 0 {
				return errAudioContainer
			}
			serial = binary.LittleEndian.Uint32(page[14:18])
			sequence = binary.LittleEndian.Uint32(page[18:22])
		} else {
			sequence++
			if flags&2 != 0 || binary.LittleEndian.Uint32(page[14:18]) != serial || binary.LittleEndian.Uint32(page[18:22]) != sequence {
				return errAudioContainer
			}
		}
		if (flags&1 != 0) != (len(packet) > 0) {
			return errAudioContainer
		}
		var crc uint32
		for i, v := range page {
			if i >= 22 && i < 26 {
				v = 0
			}
			crc ^= uint32(v) << 24
			for j := 0; j < 8; j++ {
				if crc&0x80000000 != 0 {
					crc = (crc << 1) ^ 0x04c11db7
				} else {
					crc <<= 1
				}
			}
		}
		if crc != binary.LittleEndian.Uint32(page[22:26]) {
			return errAudioContainer
		}
		pos := 27 + n
		for _, v := range page[27 : 27+n] {
			packet = append(packet, page[pos:pos+int(v)]...)
			pos += int(v)
			if len(packet) > 65536 {
				return errAudioContainer
			}
			if v < 255 {
				switch packets {
				case 0:
					if !validOpusHead(packet) {
						return errAudioContainer
					}
					preSkip = uint64(binary.LittleEndian.Uint16(packet[10:12]))
				case 1:
					if len(packet) < 16 || string(packet[:8]) != "OpusTags" {
						return errAudioContainer
					}
					vendor := uint64(binary.LittleEndian.Uint32(packet[8:12]))
					if vendor > uint64(len(packet)-16) {
						return errAudioContainer
					}
				default:
					s, e := opusPacketSamples(packet)
					if e != nil {
						return e
					}
					totalSamples += s
					if totalSamples > 120*48000+int(preSkip)+5760 {
						return errAudioContainer
					}
				}
				packets++
				packet = nil
			}
		}
		granule := binary.LittleEndian.Uint64(page[6:14])
		if granule != math.MaxUint64 && granule > preSkip+120*48000 {
			return errAudioContainer
		}
		seenEnd = flags&4 != 0
		off = end
	}
	if !seenEnd || len(packet) != 0 || packets < 3 || totalSamples == 0 {
		return errAudioContainer
	}
	return nil
}

type ebmlElement struct {
	id   uint64
	data []byte
}

func ebmlVint(b []byte, keepMarker bool) (uint64, int, bool) {
	if len(b) == 0 || b[0] == 0 {
		return 0, 0, false
	}
	n := 1
	mask := byte(0x80)
	for b[0]&mask == 0 {
		n++
		mask >>= 1
	}
	if n > 8 || n > len(b) {
		return 0, 0, false
	}
	v := uint64(b[0])
	if !keepMarker {
		v &= uint64(mask - 1)
	}
	for _, x := range b[1:n] {
		v = v<<8 | uint64(x)
	}
	return v, n, true
}
func ebmlChildren(b []byte, allowUnknown bool) ([]ebmlElement, error) {
	var out []ebmlElement
	for len(b) > 0 {
		if len(out) > 100000 {
			return nil, errAudioContainer
		}
		id, n, ok := ebmlVint(b, true)
		if !ok || n > 4 {
			return nil, errAudioContainer
		}
		b = b[n:]
		size, n, ok := ebmlVint(b, false)
		if !ok {
			return nil, errAudioContainer
		}
		b = b[n:]
		if size == (uint64(1)<<(7*n))-1 {
			if !allowUnknown || (id != 0x18538067 && id != 0x1f43b675) {
				return nil, errAudioContainer
			}
			size = uint64(len(b))
			if id == 0x1f43b675 {
				length, err := ebmlUnknownClusterLength(b)
				if err != nil {
					return nil, err
				}
				size = uint64(length)
			}
		}
		if size > uint64(len(b)) {
			return nil, errAudioContainer
		}
		out = append(out, ebmlElement{id, b[:int(size)]})
		b = b[int(size):]
	}
	return out, nil
}

// An unknown-size Cluster ends before the next Segment-level element. Scan
// complete child elements, never byte-search inside encoded frame payloads.
func ebmlUnknownClusterLength(b []byte) (int, error) {
	for offset, count := 0, 0; offset < len(b); count++ {
		if count > 100000 {
			return 0, errAudioContainer
		}
		id, n, ok := ebmlVint(b[offset:], true)
		if !ok || n > 4 {
			return 0, errAudioContainer
		}
		switch id {
		case 0x114d9b74, 0x1549a966, 0x1654ae6b, 0x1f43b675, 0x1c53bb6b, 0x1941a469, 0x1043a770, 0x1254c367:
			return offset, nil
		}
		size, k, ok := ebmlVint(b[offset+n:], false)
		if !ok || size == (uint64(1)<<(7*k))-1 || size > uint64(len(b)-offset-n-k) {
			return 0, errAudioContainer
		}
		offset += n + k + int(size)
	}
	return len(b), nil
}
func ebmlUint(b []byte) uint64 {
	if len(b) > 8 {
		return math.MaxUint64
	}
	var v uint64
	for _, x := range b {
		v = v<<8 | uint64(x)
	}
	return v
}
func ebmlFloat(b []byte) float64 {
	if len(b) == 4 {
		return float64(math.Float32frombits(binary.BigEndian.Uint32(b)))
	}
	if len(b) == 8 {
		return math.Float64frombits(binary.BigEndian.Uint64(b))
	}
	return math.NaN()
}
func validateWebM(b []byte) error {
	roots, e := ebmlChildren(b, true)
	if e != nil || len(roots) < 2 || roots[0].id != 0x1a45dfa3 {
		return errAudioContainer
	}
	header, e := ebmlChildren(roots[0].data, false)
	if e != nil {
		return e
	}
	doc := false
	for _, x := range header {
		if x.id == 0x4282 && string(x.data) == "webm" {
			doc = true
		}
	}
	if !doc {
		return errAudioContainer
	}
	var segment []ebmlElement
	for _, x := range roots[1:] {
		if x.id == 0x18538067 {
			if segment != nil {
				return errAudioContainer
			}
			segment, e = ebmlChildren(x.data, true)
			if e != nil {
				return e
			}
		}
	}
	if segment == nil {
		return errAudioContainer
	}
	scale := uint64(1000000)
	duration := float64(0)
	tracks := map[uint64]string{}
	for _, x := range segment {
		if x.id == 0x1549a966 {
			info, err := ebmlChildren(x.data, false)
			if err != nil {
				return err
			}
			for _, i := range info {
				if i.id == 0x2ad7b1 {
					scale = ebmlUint(i.data)
				}
				if i.id == 0x4489 {
					duration = ebmlFloat(i.data)
				}
			}
		}
		if x.id == 0x1654ae6b {
			entries, err := ebmlChildren(x.data, false)
			if err != nil {
				return err
			}
			for _, entry := range entries {
				if entry.id != 0xae {
					continue
				}
				values, err := ebmlChildren(entry.data, false)
				if err != nil {
					return err
				}
				var num, typ uint64
				var codec string
				var private []byte
				audio := false
				for _, v := range values {
					switch v.id {
					case 0xd7:
						num = ebmlUint(v.data)
					case 0x83:
						typ = ebmlUint(v.data)
					case 0x86:
						codec = string(v.data)
					case 0x63a2:
						private = v.data
					case 0xe1:
						vals, err := ebmlChildren(v.data, false)
						if err != nil {
							return err
						}
						audio = len(vals) > 0
					}
				}
				if typ == 2 {
					if num == 0 || !audio || codec != "A_OPUS" || !validOpusHead(private) {
						return errAudioContainer
					}
					tracks[num] = codec
				}
			}
		}
	}
	if len(tracks) == 0 || scale == 0 || scale > 1000000000 || math.IsNaN(duration) || math.IsInf(duration, 0) || duration < 0 || duration*float64(scale) > 120e9 {
		return errAudioContainer
	}
	blocks := 0
	cluster := func(data []byte) error {
		values, err := ebmlChildren(data, true)
		if err != nil {
			return err
		}
		var timestamp uint64
		for _, v := range values {
			if v.id == 0xe7 {
				timestamp = ebmlUint(v.data)
			}
		}
		for _, v := range values {
			var block []byte
			if v.id == 0xa3 {
				block = v.data
			}
			if v.id == 0xa0 {
				group, err := ebmlChildren(v.data, false)
				if err != nil {
					return err
				}
				for _, g := range group {
					if g.id == 0xa1 {
						block = g.data
					}
				}
			}
			if block == nil {
				continue
			}
			track, n, ok := ebmlVint(block, false)
			if !ok || len(block) < n+4 {
				return errAudioContainer
			}
			if tracks[track] == "" {
				return errAudioContainer
			}
			relative := int64(int16(binary.BigEndian.Uint16(block[n : n+2])))
			ms := (float64(timestamp) + float64(relative)) * float64(scale) / 1e6
			if ms < 0 || ms > 120000 {
				return errAudioContainer
			}
			flags := block[n+2]
			payload := block[n+3:]
			if flags&6 != 0 {
				return errAudioContainer
			}
			samples, err := opusPacketSamples(payload)
			if err != nil || ms+float64(samples)*1000/48000 > 120001 {
				return errAudioContainer
			}
			blocks++
		}
		return nil
	}
	for _, x := range segment {
		if x.id == 0x1f43b675 {
			if e = cluster(x.data); e != nil {
				return e
			}
		}
	}
	if blocks == 0 {
		return errAudioContainer
	}
	return nil
}

type mp4Box struct {
	kind                  string
	start, end, dataStart int
	data                  []byte
}

func mp4Boxes(b []byte, base int) ([]mp4Box, error) {
	var out []mp4Box
	for off := 0; off < len(b); {
		if len(b)-off < 8 || len(out) > 100000 {
			return nil, errAudioContainer
		}
		size := uint64(binary.BigEndian.Uint32(b[off : off+4]))
		header := 8
		if size == 1 {
			if len(b)-off < 16 {
				return nil, errAudioContainer
			}
			size = binary.BigEndian.Uint64(b[off+8 : off+16])
			header = 16
		}
		if size == 0 {
			size = uint64(len(b) - off)
		}
		if size < uint64(header) || size > uint64(len(b)-off) {
			return nil, errAudioContainer
		}
		end := off + int(size)
		out = append(out, mp4Box{string(b[off+4 : off+8]), base + off, base + end, base + off + header, b[off+header : end]})
		off = end
	}
	return out, nil
}
func mp4Child(parent mp4Box, kind string) (mp4Box, bool, error) {
	children, err := mp4Boxes(parent.data, parent.dataStart)
	if err != nil {
		return mp4Box{}, false, err
	}
	for _, b := range children {
		if b.kind == kind {
			return b, true, nil
		}
	}
	return mp4Box{}, false, nil
}
func mp4Required(p mp4Box, kind string) (mp4Box, error) {
	b, ok, e := mp4Child(p, kind)
	if e != nil || !ok {
		return b, errAudioContainer
	}
	return b, nil
}
func u32(b []byte) uint32 { return binary.BigEndian.Uint32(b) }
func mp4Inside(offset, size uint64, mdats []mp4Box) bool {
	if size == 0 || offset+size < offset {
		return false
	}
	for _, m := range mdats {
		if offset >= uint64(m.dataStart) && offset+size <= uint64(m.end) {
			return true
		}
	}
	return false
}
func validateMP4(b []byte) error {
	boxes, err := mp4Boxes(b, 0)
	if err != nil || len(boxes) < 3 || boxes[0].kind != "ftyp" || len(boxes[0].data) < 8 {
		return errAudioContainer
	}
	var moov mp4Box
	var mdats, moofs []mp4Box
	for _, x := range boxes {
		switch x.kind {
		case "moov":
			moov = x
		case "mdat":
			mdats = append(mdats, x)
		case "moof":
			moofs = append(moofs, x)
		}
	}
	if moov.kind == "" || len(mdats) == 0 {
		return errAudioContainer
	}
	children, err := mp4Boxes(moov.data, moov.dataStart)
	if err != nil {
		return err
	}
	audioTracks := map[uint32]uint32{}
	sampleCount := uint64(0)
	for _, trak := range children {
		if trak.kind != "trak" {
			continue
		}
		tkhd, e := mp4Required(trak, "tkhd")
		if e != nil || len(tkhd.data) < 20 {
			return errAudioContainer
		}
		idPos := 12
		if tkhd.data[0] == 1 {
			idPos = 20
		}
		if len(tkhd.data) < idPos+4 {
			return errAudioContainer
		}
		trackID := u32(tkhd.data[idPos : idPos+4])
		mdia, e := mp4Required(trak, "mdia")
		if e != nil {
			return e
		}
		hdlr, e := mp4Required(mdia, "hdlr")
		if e != nil || len(hdlr.data) < 12 {
			return errAudioContainer
		}
		if string(hdlr.data[8:12]) != "soun" {
			return errAudioContainer
		}
		mdhd, e := mp4Required(mdia, "mdhd")
		if e != nil || len(mdhd.data) < 20 {
			return errAudioContainer
		}
		timePos := 12
		durationSize := 4
		if mdhd.data[0] == 1 {
			timePos = 20
			durationSize = 8
		}
		if len(mdhd.data) < timePos+4+durationSize {
			return errAudioContainer
		}
		scale := u32(mdhd.data[timePos : timePos+4])
		duration := ebmlUint(mdhd.data[timePos+4 : timePos+4+durationSize])
		if scale == 0 || duration == math.MaxUint64 || duration > uint64(scale)*120 {
			return errAudioContainer
		}
		audioTracks[trackID] = scale
		minf, e := mp4Required(mdia, "minf")
		if e != nil {
			return e
		}
		stbl, e := mp4Required(minf, "stbl")
		if e != nil {
			return e
		}
		stsd, e := mp4Required(stbl, "stsd")
		if e != nil || len(stsd.data) < 8 || u32(stsd.data[4:8]) != 1 {
			return errAudioContainer
		}
		descriptions, e := mp4Boxes(stsd.data[8:], stsd.dataStart+8)
		if e != nil || len(descriptions) != 1 {
			return errAudioContainer
		}
		desc := descriptions[0]
		if (desc.kind != "mp4a" && desc.kind != "Opus") || len(desc.data) < 28 || binary.BigEndian.Uint16(desc.data[16:18]) == 0 {
			return errAudioContainer
		}
		codec, e := mp4Boxes(desc.data[28:], desc.dataStart+28)
		if e != nil {
			return e
		}
		codecOK := false
		for _, c := range codec {
			if desc.kind == "mp4a" && c.kind == "esds" && len(c.data) >= 12 {
				codecOK = true
			}
			if desc.kind == "Opus" && c.kind == "dOps" && len(c.data) >= 11 {
				codecOK = true
			}
		}
		if !codecOK {
			return errAudioContainer
		}
		stsz, e := mp4Required(stbl, "stsz")
		if e != nil || len(stsz.data) < 12 {
			return errAudioContainer
		}
		fixed, n := u32(stsz.data[4:8]), u32(stsz.data[8:12])
		if n > uint32(maxVoice) {
			return errAudioContainer
		}
		if n == 0 {
			if len(moofs) == 0 {
				return errAudioContainer
			}
			continue
		}
		if fixed == 0 && uint64(n)*4+12 != uint64(len(stsz.data)) {
			return errAudioContainer
		}
		sizes := make([]uint32, n)
		for i := range sizes {
			sizes[i] = fixed
			if fixed == 0 {
				sizes[i] = u32(stsz.data[12+i*4 : 16+i*4])
			}
			if sizes[i] == 0 {
				return errAudioContainer
			}
		}
		stco, ok, e := mp4Child(stbl, "stco")
		if e != nil {
			return e
		}
		width := 4
		if !ok {
			stco, ok, e = mp4Child(stbl, "co64")
			width = 8
		}
		if e != nil || !ok || len(stco.data) < 8 {
			return errAudioContainer
		}
		chunks := u32(stco.data[4:8])
		if chunks == 0 || uint64(chunks)*uint64(width)+8 != uint64(len(stco.data)) {
			return errAudioContainer
		}
		stsc, e := mp4Required(stbl, "stsc")
		if e != nil || len(stsc.data) < 8 {
			return errAudioContainer
		}
		entries := u32(stsc.data[4:8])
		if entries == 0 || uint64(entries)*12+8 != uint64(len(stsc.data)) {
			return errAudioContainer
		}
		entry := uint32(0)
		sample := 0
		previous := uint32(0)
		for i := uint32(0); i < entries; i++ {
			first := u32(stsc.data[8+i*12 : 12+i*12])
			per := u32(stsc.data[12+i*12 : 16+i*12])
			description := u32(stsc.data[16+i*12 : 20+i*12])
			if first <= previous || first > chunks || per == 0 || description != 1 || (i == 0 && first != 1) {
				return errAudioContainer
			}
			previous = first
		}
		for chunk := uint32(1); chunk <= chunks; chunk++ {
			if entry+1 < entries && u32(stsc.data[8+(entry+1)*12:12+(entry+1)*12]) == chunk {
				entry++
			}
			per := u32(stsc.data[12+entry*12 : 16+entry*12])
			offset := ebmlUint(stco.data[8+int(chunk-1)*width : 8+int(chunk)*width])
			for j := uint32(0); j < per; j++ {
				if sample >= len(sizes) || !mp4Inside(offset, uint64(sizes[sample]), mdats) {
					return errAudioContainer
				}
				offset += uint64(sizes[sample])
				sample++
			}
		}
		if sample != len(sizes) {
			return errAudioContainer
		}
		sampleCount += uint64(sample)
	}
	if len(audioTracks) == 0 {
		return errAudioContainer
	}
	defaults := map[uint32][2]uint32{}
	if mvex, ok, e := mp4Child(moov, "mvex"); e != nil {
		return e
	} else if ok {
		v, e := mp4Boxes(mvex.data, mvex.dataStart)
		if e != nil {
			return e
		}
		for _, x := range v {
			if x.kind == "trex" {
				if len(x.data) < 24 {
					return errAudioContainer
				}
				defaults[u32(x.data[4:8])] = [2]uint32{u32(x.data[12:16]), u32(x.data[16:20])}
			}
		}
	}
	durations := map[uint32]uint64{}
	for _, moof := range moofs {
		items, e := mp4Boxes(moof.data, moof.dataStart)
		if e != nil {
			return e
		}
		for _, traf := range items {
			if traf.kind != "traf" {
				continue
			}
			tfhd, e := mp4Required(traf, "tfhd")
			if e != nil || len(tfhd.data) < 8 {
				return errAudioContainer
			}
			track := u32(tfhd.data[4:8])
			scale := audioTracks[track]
			if scale == 0 {
				return errAudioContainer
			}
			flags := u32(tfhd.data[:4]) & 0xffffff
			base := uint64(moof.start)
			pos := 8
			read := func(n int) ([]byte, bool) {
				if pos+n > len(tfhd.data) {
					return nil, false
				}
				v := tfhd.data[pos : pos+n]
				pos += n
				return v, true
			}
			if flags&1 != 0 {
				v, ok := read(8)
				if !ok {
					return errAudioContainer
				}
				base = ebmlUint(v)
			}
			if flags&2 != 0 {
				if _, ok := read(4); !ok {
					return errAudioContainer
				}
			}
			d := defaults[track]
			if flags&8 != 0 {
				v, ok := read(4)
				if !ok {
					return errAudioContainer
				}
				d[0] = u32(v)
			}
			if flags&16 != 0 {
				v, ok := read(4)
				if !ok {
					return errAudioContainer
				}
				d[1] = u32(v)
			}
			if flags&32 != 0 {
				if _, ok := read(4); !ok {
					return errAudioContainer
				}
			}
			if pos != len(tfhd.data) {
				return errAudioContainer
			}
			runs, e := mp4Boxes(traf.data, traf.dataStart)
			if e != nil {
				return e
			}
			var nextOffset uint64
			hasOffset := false
			for _, run := range runs {
				if run.kind == "tfdt" {
					if len(run.data) < 8 {
						return errAudioContainer
					}
					decodeTime := ebmlUint(run.data[4:])
					if decodeTime > uint64(scale)*120 {
						return errAudioContainer
					}
				}
				if run.kind != "trun" {
					continue
				}
				if len(run.data) < 8 {
					return errAudioContainer
				}
				f := u32(run.data[:4]) & 0xffffff
				n := u32(run.data[4:8])
				if n == 0 || n > maxVoice {
					return errAudioContainer
				}
				p := 8
				if f&1 != 0 {
					if len(run.data) < p+4 {
						return errAudioContainer
					}
					signed := int64(int32(u32(run.data[p : p+4])))
					p += 4
					if signed < 0 && uint64(-signed) > base {
						return errAudioContainer
					}
					nextOffset = uint64(int64(base) + signed)
					hasOffset = true
				}
				if !hasOffset {
					return errAudioContainer
				}
				if f&4 != 0 {
					p += 4
				}
				for i := uint32(0); i < n; i++ {
					duration, size := d[0], d[1]
					for _, flag := range []uint32{0x100, 0x200, 0x400, 0x800} {
						if f&flag != 0 {
							if p+4 > len(run.data) {
								return errAudioContainer
							}
							if flag == 0x100 {
								duration = u32(run.data[p : p+4])
							}
							if flag == 0x200 {
								size = u32(run.data[p : p+4])
							}
							p += 4
						}
					}
					if duration == 0 || !mp4Inside(nextOffset, uint64(size), mdats) {
						return errAudioContainer
					}
					nextOffset += uint64(size)
					durations[track] += uint64(duration)
					if durations[track] > uint64(scale)*120 {
						return errAudioContainer
					}
					sampleCount++
				}
				if p != len(run.data) {
					return errAudioContainer
				}
			}
		}
	}
	if sampleCount == 0 {
		return errAudioContainer
	}
	return nil
}
