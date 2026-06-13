package gamestate

import "encoding/binary"

const (
	passivePanelVtable = 0x142FE0C88
	uiChildrenBeginOff = 0x10
	uiChildrenEndOff   = 0x18
	elementFlagsOff    = 0x180
	elementVisibleBit  = 0x0B
	uiChildrenMax      = 4096
)

func uiChildren(r Reader, gsoSlot uint64) []byte {
	ui, err := ResolveUiRoot(r, gsoSlot)
	if err != nil || ui == 0 {
		return nil
	}
	begin := ReadU64(r, ui+uiChildrenBeginOff)
	end := ReadU64(r, ui+uiChildrenEndOff)
	if begin < HeapLo || end <= begin || end >= HeapHi {
		return nil
	}
	n := int((end - begin) / 8)
	if n <= 0 || n > uiChildrenMax {
		return nil
	}
	buf, err := r.ReadBytes(begin, n*8)
	if err != nil || len(buf) < n*8 {
		return nil
	}
	return buf
}

func screenOpenByVtable(r Reader, gsoSlot, vtable uint64) bool {
	buf := uiChildren(r, gsoSlot)
	for i := 0; i+8 <= len(buf); i += 8 {
		child := binary.LittleEndian.Uint64(buf[i:])
		if child < HeapLo || child >= HeapHi || ReadU64(r, child) != vtable {
			continue
		}
		if ReadU32(r, child+elementFlagsOff)&(1<<elementVisibleBit) != 0 {
			return true
		}
	}
	return false
}

func PassiveScreenOpen(r Reader, gsoSlot uint64) bool {
	return screenOpenByVtable(r, gsoSlot, passivePanelVtable)
}
