package gamestate

const FlaskBarVtable uint64 = 0x142F34638

const (
	flaskBarMaxWalk   = 60000
	flaskBarSlotCount = 5
)

type FlaskSlot struct {
	Slot    int
	Name    string
	X, Y    float32
	W, H    float32
	Current int
	PerUse  int
}

type FlaskBarReader struct {
	elem uint64
	belt uint64
}

func NewFlaskBarReader() *FlaskBarReader { return &FlaskBarReader{} }

func (fr *FlaskBarReader) Read(r Reader, gsoSlot uint64) []FlaskSlot {
	if !fr.elemValid(r) {
		root, err := ResolveTrueUiRoot(r, gsoSlot)
		if err != nil || root == 0 {
			fr.elem = 0
			return nil
		}
		fr.elem = findElementByVtable(r, root, FlaskBarVtable, flaskBarMaxWalk)
	}
	if fr.elem == 0 {
		return nil
	}
	if fr.belt == 0 {
		fr.belt = ResolveFlaskBelt(r, gsoSlot)
	}
	begin := ReadU64(r, fr.elem+ElementChildBegOff)
	end := ReadU64(r, fr.elem+ElementChildEndOff)
	if begin < HeapLo || end <= begin {
		return nil
	}
	n := min(int((end-begin)/8), flaskBarSlotCount)
	out := make([]FlaskSlot, 0, n)
	for i := range n {
		child := ReadU64(r, begin+uint64(i)*8)
		if child < HeapLo || child >= HeapHi || ReadU64(r, child+ElementSelfOff) != child {
			continue
		}
		c, ok := FlaskChargesInSlot(r, fr.belt, i+1)
		if !ok {
			continue
		}
		x, y, ok := ElementAbsPos(r, child)
		if !ok {
			continue
		}
		w, h := ElementSize(r, child)
		out = append(out, FlaskSlot{
			Slot: i + 1, Name: flaskSlotName(i + 1),
			X: x, Y: y, W: w, H: h,
			Current: c.Current, PerUse: c.PerUse,
		})
	}
	if len(out) == 0 {
		fr.belt = 0
	}
	return out
}

func (fr *FlaskBarReader) elemValid(r Reader) bool {
	return fr.elem != 0 && ReadU64(r, fr.elem) == FlaskBarVtable &&
		ReadU64(r, fr.elem+ElementSelfOff) == fr.elem
}

func flaskSlotName(slot int) string {
	switch slot {
	case 1:
		return "Life"
	case 2:
		return "Mana"
	default:
		return "Utility"
	}
}

func findElementByVtable(r Reader, root, vtable uint64, maxWalk int) uint64 {
	var found uint64
	seen := make(map[uint64]bool)
	n := 0
	var walk func(e uint64, depth int)
	walk = func(e uint64, depth int) {
		if found != 0 || e < HeapLo || e >= HeapHi || depth > 16 || seen[e] || n > maxWalk {
			return
		}
		seen[e] = true
		n++
		if ReadU64(r, e) == vtable && ReadU64(r, e+ElementSelfOff) == e {
			found = e
			return
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
	return found
}
