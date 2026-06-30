package gamestate

import (
	"bytes"
	"encoding/binary"
)

const ItemEntityVtable uint64 = 0x143467980

func ScanItemEntities(r Reader, regions []HeapRegion) []uint64 {
	needle := make([]byte, 8)
	binary.LittleEndian.PutUint64(needle, ItemEntityVtable)
	const chunkSize = 1 << 20
	var out []uint64
	for _, reg := range regions {
		for off := uint64(0); off < reg.Size; off += chunkSize {
			n := uint64(chunkSize)
			if reg.Size-off < n {
				n = reg.Size - off
			}
			data, err := r.ReadBytes(reg.Start+off, int(n))
			if err != nil || len(data) < 8 {
				continue
			}
			idx := 0
			for {
				i := bytes.Index(data[idx:], needle)
				if i < 0 {
					break
				}
				abs := reg.Start + off + uint64(idx+i)
				idx += i + 1
				if abs&7 != 0 {
					continue
				}
				out = append(out, abs)
			}
		}
	}
	return out
}
