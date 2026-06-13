package gamestate

import "encoding/binary"

const (
	atlasPassivesOff = 0x170
	atlasAllocMax    = 4096
)

func resolvePerPlayerSubData(r Reader, gsoSlot uint64) uint64 {
	sd, err := resolveServerData(r, gsoSlot)
	if err != nil {
		return 0
	}
	p1 := ReadU64(r, sd+goldHop1Off)
	if p1 < HeapLo || p1 >= HeapHi {
		return 0
	}
	p2 := ReadU64(r, p1+goldHop2Off)
	if p2 < HeapLo || p2 >= HeapHi {
		return 0
	}
	sub := ReadU64(r, p2+goldHop3Off)
	if sub < HeapLo || sub >= HeapHi {
		return 0
	}
	return sub
}

func atlasPassiveVector(r Reader, gsoSlot uint64) (begin uint64, count int, ok bool) {
	sub := resolvePerPlayerSubData(r, gsoSlot)
	if sub == 0 {
		return 0, 0, false
	}
	begin = ReadU64(r, sub+atlasPassivesOff)
	end := ReadU64(r, sub+atlasPassivesOff+8)
	if begin == end {
		return 0, 0, true
	}
	if begin < HeapLo || begin >= HeapHi || end <= begin || (end-begin)%2 != 0 {
		return 0, 0, false
	}
	count = int((end - begin) / 2)
	if count > atlasAllocMax {
		return 0, 0, false
	}
	return begin, count, true
}

func ReadAllocatedAtlasPassives(r Reader, gsoSlot uint64) []int {
	begin, count, ok := atlasPassiveVector(r, gsoSlot)
	if !ok || count == 0 {
		return nil
	}
	buf, err := r.ReadBytes(begin, count*2)
	if err != nil || len(buf) < count*2 {
		return nil
	}
	out := make([]int, count)
	for i := range count {
		out[i] = int(binary.LittleEndian.Uint16(buf[i*2:]))
	}
	return out
}

func AtlasPassiveVectorOK(r Reader, gsoSlot uint64) bool {
	_, _, ok := atlasPassiveVector(r, gsoSlot)
	return ok
}
