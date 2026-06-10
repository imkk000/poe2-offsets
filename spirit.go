package gamestate

import "encoding/binary"

const (
	spiritSlotSentinelOffset = 16
	spiritSlotSentinelValue  = 0xFFFFFFFF
	spiritSlotPayloadOffset  = 32
)

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
