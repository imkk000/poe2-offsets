package gamestate

const (
	HeapLo = 0x10000000000
	HeapHi = 0x800000000000

	dataPtrLo = 0x150000000

	CompListBeginOff = 0x10
	CompListEndOff   = 0x18
	MaxComponents    = 32
)

func validDataPtr(addr uint64) bool {
	return addr >= dataPtrLo && addr < HeapHi
}
