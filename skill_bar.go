package gamestate

import "strings"

const (
	skillIconSkillOff = 0x2F0
	skillBarMaxWalk   = 60000

	skillMgrOff       = 0x9C0
	skillArrBeginOff  = 0x148
	skillArrEndOff    = 0x150
	skillInstStride   = 0x10
	skillNameChainOff = 0x58
	maxSkillInstances = 80
)

type SkillSlot struct {
	Name string
	X, Y float32
	W, H float32
}

type SkillBarReader struct {
	elems []uint64
}

func NewSkillBarReader() *SkillBarReader { return &SkillBarReader{} }

func (sr *SkillBarReader) Read(r Reader, gsoSlot, actor uint64) []SkillSlot {
	names := managerInstanceNames(r, actor)
	if len(names) == 0 {
		return nil
	}
	if !sr.cacheValid(r, names) {
		root, err := ResolveTrueUiRoot(r, gsoSlot)
		if err != nil || root == 0 {
			return nil
		}
		sr.elems = sr.findSlots(r, root, names)
	}
	out := make([]SkillSlot, 0, len(sr.elems))
	for _, e := range sr.elems {
		nm, ok := names[ReadU64(r, e+skillIconSkillOff)]
		if !ok {
			continue
		}

		if !ElementVisibleHierarchical(r, e) {
			continue
		}
		x, y, ok := ElementAbsPos(r, e)
		if !ok {
			continue
		}
		w, h := ElementSize(r, e)
		out = append(out, SkillSlot{Name: nm, X: x, Y: y, W: w, H: h})
	}
	return out
}

func (sr *SkillBarReader) cacheValid(r Reader, names map[uint64]string) bool {
	if len(sr.elems) == 0 {
		return false
	}
	for _, e := range sr.elems {
		if _, ok := names[ReadU64(r, e+skillIconSkillOff)]; !ok {
			return false
		}
	}
	return true
}

func (sr *SkillBarReader) findSlots(r Reader, root uint64, names map[uint64]string) []uint64 {
	var out []uint64
	seen := make(map[uint64]bool)
	n := 0
	var walk func(e uint64, depth int)
	walk = func(e uint64, depth int) {
		if e < HeapLo || e >= HeapHi || depth > 14 || seen[e] || n > skillBarMaxWalk {
			return
		}
		seen[e] = true
		n++
		if _, ok := names[ReadU64(r, e+skillIconSkillOff)]; ok && ReadU64(r, e+ElementSelfOff) == e {
			out = append(out, e)
		}
		begin := ReadU64(r, e+ElementChildBegOff)
		end := ReadU64(r, e+ElementChildEndOff)
		if begin < HeapLo || end <= begin || end-begin > 0x8000 {
			return
		}
		buf, err := r.ReadBytes(begin, int((end-begin)/8)*8)
		if err != nil {
			return
		}
		for i := 0; i+8 <= len(buf); i += 8 {
			walk(ReadU64Bytes(buf, uint64(i)), depth+1)
		}
	}
	walk(root, 0)
	return out
}

func managerInstanceNames(r Reader, actor uint64) map[uint64]string {
	if actor < HeapLo || actor >= HeapHi {
		return nil
	}
	mgr := actor + skillMgrOff
	begin := ReadU64(r, mgr+skillArrBeginOff)
	end := ReadU64(r, mgr+skillArrEndOff)
	if begin < HeapLo || end <= begin || end-begin > 0x4000 {
		return nil
	}
	count := int((end - begin) / skillInstStride)
	out := make(map[uint64]string, count)
	for i := 0; i < count && i < maxSkillInstances; i++ {
		inst := ReadU64(r, begin+uint64(i)*skillInstStride)
		if inst < HeapLo || inst >= HeapHi {
			continue
		}
		if nm := skillInstName(r, inst); nm != "" {
			out[inst] = nm
		}
	}
	return out
}

func skillInstName(r Reader, inst uint64) string {
	a := ReadU64(r, inst+skillNameChainOff)
	for _, base := range []uint64{a, ReadU64(r, a), ReadU64(r, ReadU64(r, a))} {
		if base < HeapLo || base >= HeapHi {
			continue
		}
		if s := readSkillInstStr(r, base); s != "" {
			return displaySkillName(s)
		}
	}
	return ""
}

func readSkillInstStr(r Reader, p uint64) string {
	raw, err := r.ReadBytes(p, 64)
	if err != nil || len(raw) < 6 {
		return ""
	}
	var b strings.Builder
	for i := 0; i+2 <= len(raw); i += 2 {
		c := uint16(raw[i]) | uint16(raw[i+1])<<8
		if c == 0 {
			break
		}
		if c < 'A' || c > 'z' || (c > 'Z' && c < 'a') {
			return ""
		}
		b.WriteByte(byte(c))
	}
	if s := b.String(); len(s) >= 4 {
		return s
	}
	return ""
}

func displaySkillName(name string) string {
	name = strings.TrimSuffix(name, "Player")
	var b strings.Builder
	for i, c := range name {
		if i > 0 && c >= 'A' && c <= 'Z' && name[i-1] >= 'a' && name[i-1] <= 'z' {
			b.WriteByte(' ')
		}
		b.WriteRune(c)
	}
	return b.String()
}
