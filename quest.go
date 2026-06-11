package gamestate

const (
	questCardVtable = 0x142F6DFB8
	questDefOff     = 0x2E0
	questEntryOff   = 0x2F0
	questRowIDOff   = 0x00
	questRowNameOff = 0x0C
	questEntStateO  = 0x3C
	questEntObjOff  = 0x3D

	questMaxNodes    = 400000
	questMaxChildren = 16384
)

type TrackedQuest struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Objective string `json:"objective"`
	Complete  bool   `json:"complete"`
}

func questChildren(r Reader, e uint64) []uint64 {
	begin := ReadU64(r, e+ElementChildBegOff)
	end := ReadU64(r, e+ElementChildEndOff)
	if begin < HeapLo || begin >= HeapHi || end <= begin || (end-begin)%8 != 0 {
		return nil
	}
	n := int((end - begin) / 8)
	if n <= 0 || n > questMaxChildren {
		return nil
	}
	buf, err := r.ReadBytes(begin, n*8)
	if err != nil || len(buf) < n*8 {
		return nil
	}
	out := make([]uint64, 0, n)
	for i := range n {
		out = append(out, ReadU64Bytes(buf, uint64(i*8)))
	}
	return out
}

func questUtf16(r Reader, p uint64) string {
	if p < HeapLo || p >= HeapHi {
		return ""
	}
	buf, err := r.ReadBytes(p, 256)
	if err != nil {
		return ""
	}
	out := make([]byte, 0, 64)
	for i := 0; i+2 <= len(buf); i += 2 {
		c := uint16(buf[i]) | uint16(buf[i+1])<<8
		if c == 0 {
			break
		}
		if c < 0x20 || c > 0x7E {
			return ""
		}
		out = append(out, byte(c))
	}
	return string(out)
}

func ReadTrackedQuests(r Reader, gsoSlot uint64) []TrackedQuest {
	root, err := ResolveTrueUiRoot(r, gsoSlot)
	if err != nil {
		return nil
	}
	var out []TrackedQuest
	seen := make(map[uint64]struct{})
	stack := []uint64{root}
	for len(stack) > 0 && len(seen) < questMaxNodes {
		e := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if _, ok := seen[e]; ok || e < HeapLo || e >= HeapHi {
			continue
		}
		seen[e] = struct{}{}
		if ReadU64(r, e+ElementSelfOff) != e {
			continue
		}
		if ReadU64(r, e) == questCardVtable {
			q := ReadU64(r, e+questDefOff)
			o := ReadU64(r, e+questEntryOff)
			state := byte(0)
			if b, err := r.ReadBytes(o+questEntStateO, 1); err == nil && len(b) == 1 {
				state = b[0]
			}
			out = append(out, TrackedQuest{
				ID:        questUtf16(r, ReadU64(r, q+questRowIDOff)),
				Name:      questUtf16(r, ReadU64(r, q+questRowNameOff)),
				Objective: questUtf16(r, ReadU64(r, o+questEntObjOff)),
				Complete:  state == 1,
			})
		}
		stack = append(stack, questChildren(r, e)...)
	}
	return out
}
