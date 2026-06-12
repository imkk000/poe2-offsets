package gamestate

import (
	"encoding/binary"
	"math"
)

const (
	compScanSize = 0x600
	worldCoordLo = 50.0
	worldCoordHi = 100000.0
	altitudeMin  = 5.0
	altitudeMax  = 500.0

	playerWrapperOff = 0x288
	playerPosPtrOff  = 0x308
)

func ReadFloat32(r Reader, addr uint64) float32 {
	return math.Float32frombits(ReadU32(r, addr))
}

func ReadPlayerPos(r Reader, cameraAddr uint64) (float32, float32) {
	return ReadFloat32(r, cameraAddr+0x378), ReadFloat32(r, cameraAddr+0x37C)
}

func ReadEntityPos(r Reader, entity uint64) (float32, float32, bool) {
	if x, y, ok := readEntityPosViaRender(r, entity); ok {
		return x, y, true
	}
	if x, y, ok := readEntityPosViaComponentList(r, entity); ok {
		return x, y, true
	}
	return readEntityPosViaPlayerChain(r, entity)
}

const (
	renderPosXOff       = 0x138
	renderPosYOff       = 0x13C
	renderBearingOff    = 0x188
	renderVisualSizeOff = 0x3F5
)

func ReadEntityBearing(r Reader, entity uint64) (float32, bool) {
	comp := ResolveComponentByName(r, entity, "Render")
	if comp == 0 {
		return 0, false
	}
	return ReadFloat32(r, comp+renderBearingOff), true
}

func ReadEntityVisualSize(r Reader, entity uint64) (byte, bool) {
	comp := ResolveComponentByName(r, entity, "Render")
	if comp == 0 {
		return 0, false
	}
	return ReadByte(r, comp+renderVisualSizeOff), true
}

func readEntityPosViaRender(r Reader, entity uint64) (float32, float32, bool) {
	comp := ResolveComponentByName(r, entity, "Render")
	if comp == 0 {
		return 0, 0, false
	}
	x := ReadFloat32(r, comp+renderPosXOff)
	y := ReadFloat32(r, comp+renderPosYOff)
	if !plausibleWorldCoord(x) || !plausibleWorldCoord(y) {
		return 0, 0, false
	}
	return x, y, true
}

func ReadU64Bytes(buf []byte, off uint64) uint64 {
	if off+8 > uint64(len(buf)) {
		return 0
	}
	var v uint64
	for i := uint64(0); i < 8; i++ {
		v |= uint64(buf[off+i]) << (i * 8)
	}
	return v
}

func readEntityPosViaPlayerChain(r Reader, entity uint64) (float32, float32, bool) {
	wrapper := ReadU64(r, entity+playerWrapperOff)
	if wrapper < HeapLo || wrapper >= HeapHi {
		return 0, 0, false
	}
	pos := ReadU64(r, wrapper+playerPosPtrOff)
	if pos < HeapLo || pos >= HeapHi {
		return 0, 0, false
	}
	return scanForWorldCoordPair(r, pos)
}

func readEntityPosViaComponentList(r Reader, entity uint64) (float32, float32, bool) {
	data, count, ok := readComponentList(r, entity)
	if !ok || count == 0 {
		return 0, 0, false
	}
	for i := uint64(0); i < count; i++ {
		comp := binary.LittleEndian.Uint64(data[i*8 : i*8+8])
		if comp < HeapLo || comp >= HeapHi {
			continue
		}
		if x, y, ok := scanForWorldCoordPair(r, comp); ok {
			return x, y, true
		}
	}
	return 0, 0, false
}

func plausibleWorldCoord(f float32) bool {
	if math.IsNaN(float64(f)) || math.IsInf(float64(f), 0) {
		return false
	}
	abs := math.Abs(float64(f))
	return abs >= worldCoordLo && abs <= worldCoordHi
}

func scanForWorldCoordPair(r Reader, addr uint64) (float32, float32, bool) {
	buf, err := r.ReadBytes(addr, compScanSize)
	if err != nil || len(buf) < 12 {
		return 0, 0, false
	}
	for off := 0; off+12 <= len(buf); off += 4 {
		x := math.Float32frombits(binary.LittleEndian.Uint32(buf[off : off+4]))
		y := math.Float32frombits(binary.LittleEndian.Uint32(buf[off+4 : off+8]))
		z := math.Float32frombits(binary.LittleEndian.Uint32(buf[off+8 : off+12]))
		if !plausibleWorldCoord(x) || !plausibleWorldCoord(y) {
			continue
		}
		if math.IsNaN(float64(z)) || math.IsInf(float64(z), 0) {
			continue
		}
		absZ := math.Abs(float64(z))
		if absZ < altitudeMin || absZ > altitudeMax {
			continue
		}
		return x, y, true
	}
	return 0, 0, false
}
