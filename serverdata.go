package gamestate

import (
	"encoding/binary"
	"errors"
)

const (
	areaServerDataOff   = 0x580
	serverIconStride    = 0xC0
	serverIconRowOff    = 0x00
	serverIconIDOff     = 0x10
	serverIconGridXOff  = 0x14
	serverIconGridYOff  = 0x18
	serverIconScanSpan  = 0x28000
	serverGridToWorld   = 250.0 / 23.0
	serverIconHeapLo    = 0x100000000
	serverIconHeapHi    = 0x800000000000
	serverIconGridLimit = 5000
)

type ServerMinimapIcon struct {
	GridX, GridY   int32
	WorldX, WorldY float32
	ID             uint32
	Name           string
}

func serverInHeap(p uint64) bool { return p >= serverIconHeapLo && p < serverIconHeapHi }

func readServerIconName(r Reader, row uint64) string {
	if !validDataPtr(row) {
		return ""
	}
	str := ReadU64(r, row)
	if !validDataPtr(str) {
		return ""
	}
	return readUTF16String(r, str, 64)
}

func resolveServerData(r Reader, gsoSlot uint64) (uint64, error) {
	area, err := ResolveAreaInstance(r, gsoSlot)
	if err != nil {
		return 0, err
	}
	sd := ReadU64(r, area+areaServerDataOff)
	if !serverInHeap(sd) {
		return 0, errors.New("ServerData not a heap pointer")
	}
	return sd, nil
}

const (
	goldHop1Off   = 0x60
	goldHop2Off   = 0x608
	goldHop3Off   = 0x790
	playerGoldOff = 0x798
)

func ReadPlayerGold(r Reader, gsoSlot uint64) (int, bool) {
	sd, err := resolveServerData(r, gsoSlot)
	if err != nil {
		return 0, false
	}
	p1 := ReadU64(r, sd+goldHop1Off)
	if !serverInHeap(p1) {
		return 0, false
	}
	p2 := ReadU64(r, p1+goldHop2Off)
	if !serverInHeap(p2) {
		return 0, false
	}
	p3 := ReadU64(r, p2+goldHop3Off)
	if !serverInHeap(p3) {
		return 0, false
	}
	return int(ReadU32(r, p3+playerGoldOff)), true
}

func readServerIconsAt(r Reader, serverData, off uint64) ([]ServerMinimapIcon, bool) {
	begin := ReadU64(r, serverData+off)
	end := ReadU64(r, serverData+off+8)
	if !serverInHeap(begin) || end <= begin || !serverInHeap(end) {
		return nil, false
	}
	span := end - begin
	if span%serverIconStride != 0 {
		return nil, false
	}
	n := int(span / serverIconStride)
	if n < 1 || n > 4000 {
		return nil, false
	}
	buf, err := r.ReadBytes(begin, n*serverIconStride)
	if err != nil || len(buf) < n*serverIconStride {
		return nil, false
	}
	out := make([]ServerMinimapIcon, 0, n)
	varied := false
	var firstX int32
	for i := range n {
		b := buf[i*serverIconStride:]
		gx := int32(binary.LittleEndian.Uint32(b[serverIconGridXOff:]))
		gy := int32(binary.LittleEndian.Uint32(b[serverIconGridYOff:]))
		if gx <= 0 || gx > serverIconGridLimit || gy <= 0 || gy > serverIconGridLimit {
			return nil, false
		}
		if i == 0 {
			firstX = gx
		} else if gx != firstX {
			varied = true
		}
		out = append(out, ServerMinimapIcon{
			GridX: gx, GridY: gy,
			WorldX: float32(gx) * serverGridToWorld,
			WorldY: float32(gy) * serverGridToWorld,
			ID:     binary.LittleEndian.Uint32(b[serverIconIDOff:]),
			Name:   readServerIconName(r, binary.LittleEndian.Uint64(b[serverIconRowOff:])),
		})
	}
	if len(out) >= 2 && !varied {
		return nil, false
	}
	return out, true
}

func ReadServerMinimapIcons(r Reader, gsoSlot uint64, cachedOff *uint64) []ServerMinimapIcon {
	sd, err := resolveServerData(r, gsoSlot)
	if err != nil {
		return nil
	}
	if *cachedOff != 0 {
		if icons, ok := readServerIconsAt(r, sd, *cachedOff); ok {
			return icons
		}
	}
	for off := uint64(0); off <= serverIconScanSpan; off += 8 {
		if icons, ok := readServerIconsAt(r, sd, off); ok {
			*cachedOff = off
			return icons
		}
	}
	*cachedOff = 0
	return nil
}
