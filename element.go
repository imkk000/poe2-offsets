package gamestate

import (
	"encoding/binary"
	"strings"
)

type NativeUtf16Text struct {
	Ptr    uint64
	Length uint32
}

func ReadNativeUtf16Text(r Reader, addr uint64) string {
	const maxUtf16TextRunes = 128
	t := ReadNativeUtf16TextStruct(r, addr)
	if t.Length == 0 || t.Length > maxUtf16TextRunes {
		return ""
	}
	if t.Ptr < HeapLo || t.Ptr >= HeapHi {
		return ""
	}
	want := int(t.Length) * 2
	raw, err := r.ReadBytes(t.Ptr, want)
	if err != nil || len(raw) < want {
		return ""
	}
	var b strings.Builder
	b.Grow(int(t.Length))
	for i := 0; i+2 <= len(raw); i += 2 {
		c := binary.LittleEndian.Uint16(raw[i : i+2])
		if c == 0 {
			break
		}
		if c < 0x20 || c > 0x7E {

			return ""
		}
		b.WriteByte(byte(c))
	}
	return b.String()
}

// ReadStdWString reads a MSVC std::u16string/wstring (the layout PoE2 uses for
// stash tab names): 16-byte union @ +0x00 (inline UTF-16 when capacity < 8, else a
// char16* heap pointer), length @ +0x10, capacity @ +0x18.
func ReadStdWString(r Reader, addr uint64) string {
	hdr, err := r.ReadBytes(addr, 0x20)
	if err != nil || len(hdr) < 0x20 {
		return ""
	}
	n := binary.LittleEndian.Uint64(hdr[0x10:])
	cap_ := binary.LittleEndian.Uint64(hdr[0x18:])
	if n == 0 || n > 256 {
		return ""
	}
	var data []byte
	if cap_ < 8 {
		if n > 7 { // SSO holds at most 7 wchars; larger len with inline cap = garbage
			return ""
		}
		data = hdr[:n*2]
	} else {
		p := binary.LittleEndian.Uint64(hdr[:8])
		if p < HeapLo || p >= HeapHi {
			return ""
		}
		if data, err = r.ReadBytes(p, int(n)*2); err != nil {
			return ""
		}
	}
	var b strings.Builder
	b.Grow(int(n))
	for i := 0; i+2 <= len(data); i += 2 {
		c := binary.LittleEndian.Uint16(data[i:])
		if c == 0 {
			break
		}
		b.WriteRune(rune(c))
	}
	return b.String()
}

func ReadNativeUtf16TextStruct(r Reader, addr uint64) NativeUtf16Text {
	buf, err := r.ReadBytes(addr, 12)
	if err != nil || len(buf) < 12 {
		return NativeUtf16Text{}
	}
	return NativeUtf16Text{
		Ptr:    binary.LittleEndian.Uint64(buf[0:8]),
		Length: binary.LittleEndian.Uint32(buf[8:12]),
	}
}

const (
	MapElementLargeMapOffset      uint64 = 856
	MapElementSmallMapOffset      uint64 = 864
	MapElementMapPropertiesOffset uint64 = 744
	MapElementOrangeWordsOffset   uint64 = 752
	MapElementBlueWordsOffset     uint64 = 920

	MapSubElementShiftOffset        uint64 = 832
	MapSubElementShiftXOffset       uint64 = 832
	MapSubElementShiftYOffset       uint64 = 836
	MapSubElementDefaultShiftOffset uint64 = 840
	MapSubElementZoomOffset         uint64 = 900
)

const (
	ElementSelfOff     uint64 = 0x08
	ElementChildBegOff uint64 = 0x10
	ElementChildEndOff uint64 = 0x18
	ElementPositionOff uint64 = 0x118
	ElementFlagsOff    uint64 = 0x180
	ElementSizeOff     uint64 = 0x288

	UIDesignCanvasW = 2560.0
	UIDesignCanvasH = 1600.0

	SkillIconElementVtable uint64 = 0x143231B88
)

func ElementAbsPos(r Reader, e uint64) (x, y float32, ok bool) {
	for range 32 {
		if e < HeapLo || e >= HeapHi {
			break
		}
		if ReadU64(r, e+ElementSelfOff) != e {
			return 0, 0, false
		}
		px := ReadFloat32(r, e+ElementPositionOff)
		py := ReadFloat32(r, e+ElementPositionOff+4)
		x += px
		y += py
		e = ReadU64(r, e+ElementParentOff)
		if e == 0 {
			return x, y, true
		}
	}
	return x, y, true
}

func ElementSize(r Reader, e uint64) (w, h float32) {
	return ReadFloat32(r, e+ElementSizeOff), ReadFloat32(r, e+ElementSizeOff+4)
}

const (
	PerPlayerServerDataQuestFlagsOffset uint64 = 560
)
