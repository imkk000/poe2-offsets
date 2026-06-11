package gamestate

import "encoding/binary"

const (
	subDataHop1      = 0x2B0
	subDataHop2      = 0x38
	atlasPassivesOff = 0x170
	atlasAllocMax    = 4096
)

func resolvePerPlayerSubData(r Reader, gsoSlot uint64) uint64 {
	sd, err := resolveServerData(r, gsoSlot)
	if err != nil {
		return 0
	}
	p1 := ReadU64(r, sd+subDataHop1)
	if p1 < HeapLo || p1 >= HeapHi {
		return 0
	}
	sub := ReadU64(r, p1+subDataHop2)
	if sub < HeapLo || sub >= HeapHi {
		return 0
	}
	return sub
}

func ReadAllocatedAtlasPassives(r Reader, gsoSlot uint64) []int {
	sub := resolvePerPlayerSubData(r, gsoSlot)
	if sub == 0 {
		return nil
	}
	begin := ReadU64(r, sub+atlasPassivesOff)
	end := ReadU64(r, sub+atlasPassivesOff+8)
	if begin == end {
		return nil
	}
	if begin < HeapLo || begin >= HeapHi || end <= begin || (end-begin)%2 != 0 {
		return nil
	}
	count := int((end - begin) / 2)
	if count > atlasAllocMax {
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
	sub := resolvePerPlayerSubData(r, gsoSlot)
	if sub == 0 {
		return false
	}
	begin := ReadU64(r, sub+atlasPassivesOff)
	end := ReadU64(r, sub+atlasPassivesOff+8)
	if begin == end {
		return true
	}
	return begin >= HeapLo && begin < HeapHi && end > begin && (end-begin)%2 == 0
}
