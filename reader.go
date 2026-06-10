package gamestate

import "encoding/binary"

type Reader interface {
	ReadBytes(addr uint64, n int) ([]byte, error)
}

func ReadU32(r Reader, addr uint64) uint32 {
	b, err := r.ReadBytes(addr, 4)
	if err != nil || len(b) < 4 {
		return 0
	}
	return binary.LittleEndian.Uint32(b)
}

func ReadU64(r Reader, addr uint64) uint64 {
	b, err := r.ReadBytes(addr, 8)
	if err != nil || len(b) < 8 {
		return 0
	}
	return binary.LittleEndian.Uint64(b)
}

func ReadByte(r Reader, addr uint64) byte {
	b, err := r.ReadBytes(addr, 1)
	if err != nil || len(b) < 1 {
		return 0
	}
	return b[0]
}
