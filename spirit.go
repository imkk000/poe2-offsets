package gamestate

import "encoding/binary"

const (
	spiritSlotSentinelOffset = 16
	spiritSlotSentinelValue  = 0xFFFFFFFF
	spiritSlotPayloadOffset  = 32

	lifeReservationBeginOff    = 0x380
	lifeReservationEndOff      = 0x388
	lifeReservationIdxPtrOff   = 0x198
	lifeReservationIdxInnerOff = 0x168
	reservationSlotStride      = 0x28
	reservationReservedOff     = 0x18
	reservationMaxOff          = 0x20
)

type SpiritInfo struct {
	Current  int
	Max      int
	Reserved int
}

func ReadPlayerSpirit(r Reader, entity uint64) (SpiritInfo, bool) {
	life := ResolveComponentByName(r, entity, "Life")
	if life == 0 {
		return SpiritInfo{}, false
	}
	begin := ReadU64(r, life+lifeReservationBeginOff)
	end := ReadU64(r, life+lifeReservationEndOff)
	if begin < HeapLo || begin >= HeapHi || end < begin {
		return SpiritInfo{}, false
	}
	count := (end - begin) / reservationSlotStride
	idxPtr := ReadU64(r, life+lifeReservationIdxPtrOff)
	if idxPtr < HeapLo || idxPtr >= HeapHi {
		return SpiritInfo{}, false
	}
	idx := uint64(ReadU32(r, idxPtr+lifeReservationIdxInnerOff))
	if idx >= count {
		return SpiritInfo{}, false
	}
	slot := begin + idx*reservationSlotStride
	reserved := int(ReadU32(r, slot+reservationReservedOff))
	spiritMax := int(ReadU32(r, slot+reservationMaxOff))
	return SpiritInfo{Current: spiritMax - reserved, Max: spiritMax, Reserved: reserved}, true
}

func ValidateSpiritSlot(r Reader, payloadAddr uint64, wantMax uint32) (uint32, bool) {
	if payloadAddr < HeapLo+spiritSlotPayloadOffset || payloadAddr >= HeapHi {
		return 0, false
	}
	base := payloadAddr - spiritSlotPayloadOffset
	buf, err := r.ReadBytes(base, spiritSlotPayloadOffset+8)
	if err != nil || len(buf) < spiritSlotPayloadOffset+8 {
		return 0, false
	}
	for i := 0; i < spiritSlotPayloadOffset; i += 4 {
		v := binary.LittleEndian.Uint32(buf[i : i+4])
		if i == spiritSlotSentinelOffset {
			if v != spiritSlotSentinelValue {
				return 0, false
			}
			continue
		}
		if v != 0 {
			return 0, false
		}
	}
	cur := binary.LittleEndian.Uint32(buf[spiritSlotPayloadOffset : spiritSlotPayloadOffset+4])
	max := binary.LittleEndian.Uint32(buf[spiritSlotPayloadOffset+4 : spiritSlotPayloadOffset+8])
	if max != wantMax || cur > max {
		return 0, false
	}
	return cur, true
}
