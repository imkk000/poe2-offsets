package gamestate

const (
	passiveSDHop1      = 0x60
	passiveSDHop2      = 0x2C0
	passiveAllocVecOff = 0x8A8
	passiveEntryStride = 8
	passiveAllocMax    = 1024
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
	sd := ReadU64(r, area+areaServerDataOff)
	if sd < HeapLo || sd >= HeapHi {
		return 0
	}
	o1 := ReadU64(r, sd+passiveSDHop1)
	if o1 < HeapLo || o1 >= HeapHi {
		return 0
	}
	o2 := ReadU64(r, o1+passiveSDHop2)
	if o2 < HeapLo || o2 >= HeapHi {
		return 0
	}
	return o2
}
