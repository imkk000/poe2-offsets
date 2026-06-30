package gamestate

import (
	"encoding/binary"
	"errors"
)

const (
	serverDataGameWorldOff = 0x2170
	gameWorldMapStatOff    = 0x130
	mapStatBeginOff        = 0x08
	mapStatEndOff          = 0x10

	mapStatEntryStride = 8
	mapStatMaxEntries  = 4096
)

type MapStat struct {
	ID    uint32
	Value int32
}

func ResolveGameWorld(r Reader, gsoSlot uint64) (uint64, error) {
	area, err := ResolveAreaInstance(r, gsoSlot)
	if err != nil {
		return 0, err
	}
	serverData := ReadU64(r, area+areaInstanceServerDataOff)
	if !validWideHeap(serverData) {
		return 0, errors.New("ServerData null at AreaInstance+0x598")
	}
	gw := ReadU64(r, serverData+serverDataGameWorldOff)
	if !validWideHeap(gw) {
		return 0, errors.New("GameWorld null at ServerData+0x2170")
	}
	return gw, nil
}

func ReadMapStats(r Reader, gw uint64) ([]MapStat, error) {
	storage := gw + gameWorldMapStatOff
	begin := ReadU64(r, storage+mapStatBeginOff)
	end := ReadU64(r, storage+mapStatEndOff)
	if !validWideHeap(begin) {
		return nil, errors.New("MapStatStorage begin not a heap pointer")
	}
	if end <= begin {
		return nil, nil
	}
	sz := end - begin
	if sz%mapStatEntryStride != 0 {
		return nil, errors.New("MapStatStorage size not a multiple of 8")
	}
	n := int(sz / mapStatEntryStride)
	if n == 0 || n > mapStatMaxEntries {
		return nil, errors.New("MapStatStorage entry count out of range")
	}
	buf, err := r.ReadBytes(begin, int(sz))
	if err != nil || len(buf) < int(sz) {
		return nil, errors.New("MapStatStorage read failed")
	}
	out := make([]MapStat, n)
	for i := range out {
		off := i * mapStatEntryStride
		out[i].ID = binary.LittleEndian.Uint32(buf[off : off+4])
		out[i].Value = int32(binary.LittleEndian.Uint32(buf[off+4 : off+8]))
	}
	return out, nil
}

func GetMapStat(r Reader, gw uint64, statID uint32) (int32, bool) {
	stats, err := ReadMapStats(r, gw)
	if err != nil {
		return 0, false
	}
	lo, hi := 0, len(stats)
	for lo < hi {
		mid := (lo + hi) >> 1
		if stats[mid].ID < statID {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo < len(stats) && stats[lo].ID == statID {
		return stats[lo].Value, true
	}
	return 0, false
}

const (
	areaInstanceServerDataOff = 0x598
)

func validWideHeap(p uint64) bool {
	return p >= 0x100000000 && p < 0x800000000000
}
