package gamestate

import (
	"encoding/binary"
	"fmt"
	"strings"
)

type AreaInfo struct {
	Code  string
	Name  string
	Level int
	Hash  uint32
}

type TerrainInfo struct {
	Addr   string
	Width  int
	Height int
	Layers []string
}

func ReadAreaInfo(r Reader, areaInstance uint64) AreaInfo {
	var info AreaInfo
	info.Level = int(ReadByte(r, areaInstance+0xC4))
	info.Hash = ReadU32(r, areaInstance+0x11C)
	infoStruct := ReadU64(r, areaInstance+0xA0)
	if infoStruct < HeapLo || infoStruct >= HeapHi {
		return info
	}
	strPtr := ReadU64(r, infoStruct)
	if strPtr < HeapLo || strPtr >= HeapHi {
		return info
	}
	buf, err := r.ReadBytes(strPtr, 256)
	if err != nil {
		return info
	}
	strs := SplitUTF16LE(buf)
	if len(strs) > 0 {
		info.Code = strs[0]
	}
	if len(strs) > 1 {
		info.Name = strs[1]
	}
	return info
}

const (
	terrainWidthOff  = 0x928
	terrainHeightOff = 0x92C
	terrainLayer0Off = 0x970
	terrainLayer1Off = 0x988
	terrainLayer2Off = 0x9A0
	terrainLayer3Off = 0x9B8
	terrainStrideOff = 0x9D0
)

func ReadTerrainInfo(r Reader, areaInstance uint64) TerrainInfo {
	var info TerrainInfo
	for _, off := range []uint64{terrainLayer0Off, terrainLayer1Off, terrainLayer2Off, terrainLayer3Off} {
		begin := ReadU64(r, areaInstance+off)
		if validDataPtr(begin) {
			info.Layers = append(info.Layers, fmt.Sprintf("%X", begin))
		}
	}
	if len(info.Layers) > 0 {
		info.Addr = info.Layers[0]
	}
	info.Width = int(ReadU32(r, areaInstance+terrainStrideOff))
	if info.Width > 0 {
		size := ReadU64(r, areaInstance+terrainLayer0Off+8) - ReadU64(r, areaInstance+terrainLayer0Off)
		if size > 0 {
			info.Height = int(size) / info.Width
		}
	}
	return info
}

func SplitUTF16LE(buf []byte) []string {
	var out []string
	var b strings.Builder
	for i := 0; i+2 <= len(buf); i += 2 {
		c := binary.LittleEndian.Uint16(buf[i : i+2])
		if c == 0 {
			if b.Len() > 0 {
				out = append(out, b.String())
				b.Reset()
			}
			continue
		}
		if c < 0x20 || c > 0x7E {
			b.Reset()
			continue
		}
		b.WriteByte(byte(c))
	}
	return out
}
