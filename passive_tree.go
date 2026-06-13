package gamestate

import "encoding/binary"

var passiveSPDHops = []uint64{0x60, 0x40, 0xCE0, 0x418}

const (
	passiveAllocVecOff = 0x8A8
	passiveEntryStride = 4
	passiveAllocMax    = 1024

	passiveScanChunk    = 64 << 20
	passiveScanMinCount = 10
	passiveScanMaxCount = 400
)

type PassiveNode struct {
	Name     string `json:"name"`
	Notable  bool   `json:"notable"`
	Keystone bool   `json:"keystone"`
}

type AllocatedPassive struct {
	GraphID  int
	Name     string
	Notable  bool
	Keystone bool
}

func PassiveNodeByID(id int) (PassiveNode, bool) {
	m := passiveNodesLoaded.Load()
	if m == nil {
		return PassiveNode{}, false
	}
	v, ok := (*m)[id]
	return v, ok
}

func allocatedVector(r Reader, gsoSlot uint64) (begin uint64, count int, ok bool) {
	area, err := ResolveAreaInstance(r, gsoSlot)
	if err != nil {
		return 0, 0, false
	}
	spd := resolveServerPlayerData(r, area)
	if spd == 0 {
		return 0, 0, false
	}
	begin = ReadU64(r, spd+passiveAllocVecOff)
	end := ReadU64(r, spd+passiveAllocVecOff+8)
	if begin < HeapLo || begin >= HeapHi || end <= begin || (end-begin)%passiveEntryStride != 0 {
		return 0, 0, false
	}
	count = int((end - begin) / passiveEntryStride)
	if count <= 0 || count > passiveAllocMax {
		return 0, 0, false
	}
	return begin, count, true
}

func AllocatedPassiveCount(r Reader, gsoSlot uint64) (int, bool) {
	_, count, ok := allocatedVector(r, gsoSlot)
	return count, ok
}

func ReadAllocatedPassives(r Reader, gsoSlot uint64) []AllocatedPassive {
	begin, count, ok := allocatedVector(r, gsoSlot)
	if !ok {
		return nil
	}
	buf, err := r.ReadBytes(begin, count*passiveEntryStride)
	if err != nil || len(buf) < count*passiveEntryStride {
		return nil
	}
	out := make([]AllocatedPassive, 0, count)
	for i := range count {
		gid := int(uint32(buf[i*passiveEntryStride]) | uint32(buf[i*passiveEntryStride+1])<<8)
		node, ok := PassiveNodeByID(gid)
		if !ok {
			continue
		}
		out = append(out, AllocatedPassive{
			GraphID:  gid,
			Name:     node.Name,
			Notable:  node.Notable,
			Keystone: node.Keystone,
		})
	}
	return out
}

func decodePassiveVec(r Reader, begin uint64, count int) (ids []int, valid, distinct int) {
	buf, err := r.ReadBytes(begin, count*passiveEntryStride)
	if err != nil || len(buf) < count*passiveEntryStride {
		return nil, 0, 0
	}
	ids = make([]int, count)
	seen := make(map[int]struct{}, count)
	for i := range count {
		gid := int(binary.LittleEndian.Uint16(buf[i*passiveEntryStride:]))
		ids[i] = gid
		seen[gid] = struct{}{}
		if _, ok := PassiveNodeByID(gid); ok {
			valid++
		}
	}
	return ids, valid, len(seen)
}

// ReadAllocatedPassivesScan finds the PassiveSkillIds vector by its content signature
// (stride-4 u32, distinct, mostly-valid graphIds) instead of a fixed pointer chain, which
// is NOT area-stable (the chain to ServerPlayerData reshuffles between hideout/town/map).
// The caller supplies rw regions (as ScanItemEntities does) and should cache the result.
func ReadAllocatedPassivesScan(r Reader, regions []HeapRegion) []AllocatedPassive {
	var bestIDs []int
	bestValid := 0
	for _, reg := range regions {
		for off := uint64(0); off < reg.Size; off += passiveScanChunk {
			n := passiveScanChunk + 24
			if uint64(n) > reg.Size-off {
				n = int(reg.Size - off)
			}
			data, err := r.ReadBytes(reg.Start+off, n)
			if err != nil {
				continue
			}
			for i := 0; i+24 <= len(data); i += 8 {
				begin := binary.LittleEndian.Uint64(data[i:])
				end := binary.LittleEndian.Uint64(data[i+8:])
				capp := binary.LittleEndian.Uint64(data[i+16:])
				if begin < HeapLo || begin >= HeapHi || end <= begin || capp < end {
					continue
				}
				span := end - begin
				if span%passiveEntryStride != 0 {
					continue
				}
				cnt := int(span / passiveEntryStride)
				if cnt < passiveScanMinCount || cnt > passiveScanMaxCount {
					continue
				}
				ids, valid, distinct := decodePassiveVec(r, begin, cnt)
				if valid*100 < cnt*70 || distinct*100 < cnt*90 {
					continue
				}
				if valid > bestValid {
					bestValid, bestIDs = valid, ids
				}
			}
		}
	}
	out := make([]AllocatedPassive, 0, len(bestIDs))
	for _, gid := range bestIDs {
		node, ok := PassiveNodeByID(gid)
		if !ok {
			continue
		}
		out = append(out, AllocatedPassive{GraphID: gid, Name: node.Name, Notable: node.Notable, Keystone: node.Keystone})
	}
	return out
}

func AllocatedNotables(r Reader, gsoSlot uint64) []string {
	var out []string
	for _, p := range ReadAllocatedPassives(r, gsoSlot) {
		if p.Notable || p.Keystone {
			out = append(out, p.Name)
		}
	}
	return out
}

func resolveServerPlayerData(r Reader, area uint64) uint64 {
	p := ReadU64(r, area+areaServerDataOff)
	for _, hop := range passiveSPDHops {
		if p < HeapLo || p >= HeapHi {
			return 0
		}
		p = ReadU64(r, p+hop)
	}
	if p < HeapLo || p >= HeapHi {
		return 0
	}
	return p
}
