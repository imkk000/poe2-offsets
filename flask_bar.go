package gamestate

import "time"

const FlaskBarVtable uint64 = 0x142FC8540

const (
	flaskBarMaxWalk   = 60000
	flaskBarSlotCount = 5

	// findFlaskBarByShape walks the whole UI tree; this is the floor between
	// attempts when it comes up empty.
	flaskBarRescanBackoff = 5 * time.Second
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
	elem    uint64
	belt    uint64
	console bool

	// nextScan throttles the shape search, which walks the whole UI tree. Without
	// it a hidden or absent bar would trigger that walk on every caller tick.
	nextScan time.Time
}

func NewFlaskBarReader() *FlaskBarReader { return &FlaskBarReader{} }

func (fr *FlaskBarReader) Read(r Reader, gsoSlot uint64) []FlaskSlot {
	if !fr.elemValid(r) {
		fr.resolveElem(r, gsoSlot)
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

// resolveElem locates the flask bar, by vtable on the mouse-and-keyboard HUD and
// by shape on the controller HUD, which never instantiates the FlaskBar class.
func (fr *FlaskBarReader) resolveElem(r Reader, gsoSlot uint64) {
	root, err := ResolveTrueUiRoot(r, gsoSlot)
	if err != nil || root == 0 {
		fr.elem = 0
		return
	}
	if fr.elem = findElementByVtable(r, root, FlaskBarVtable, flaskBarMaxWalk); fr.elem != 0 {
		fr.console = false
		return
	}
	if time.Now().Before(fr.nextScan) {
		return
	}
	if fr.belt == 0 {
		fr.belt = ResolveFlaskBelt(r, gsoSlot)
	}
	fr.elem = findFlaskBarByShape(r, root, beltFlaskCount(r, fr.belt))
	fr.console = fr.elem != 0
	if fr.elem == 0 {
		fr.nextScan = time.Now().Add(flaskBarRescanBackoff)
	}
}

func (fr *FlaskBarReader) elemValid(r Reader) bool {
	if fr.elem == 0 || ReadU64(r, fr.elem+ElementSelfOff) != fr.elem {
		return false
	}
	// The console bar has no distinguishing vtable, so a live self-pointer plus
	// surviving children is all that can be revalidated cheaply.
	if fr.console {
		beg := ReadU64(r, fr.elem+ElementChildBegOff)
		end := ReadU64(r, fr.elem+ElementChildEndOff)
		return beg >= HeapLo && end > beg
	}
	return ReadU64(r, fr.elem) == FlaskBarVtable
}

const (
	flaskShapeMinPx   = 40
	flaskShapeMaxPx   = 260
	flaskShapeMinY    = 1000
	flaskShapeMaxKids = 8
)

// beltFlaskCount reports how many belt slots actually hold a flask.
func beltFlaskCount(r Reader, belt uint64) int {
	if belt == 0 {
		return 0
	}
	n := 0
	for i := 1; i <= flaskBarSlotCount; i++ {
		if _, ok := FlaskChargesInSlot(r, belt, i); ok {
			n++
		}
	}
	return n
}

// findFlaskBarByShape walks the UI tree for the flask row. Requiring the child
// count to equal the belt's flask count is what separates it from the many other
// rows of equally sized icons in the HUD.
func findFlaskBarByShape(r Reader, root uint64, wantSlots int) uint64 {
	if wantSlots <= 0 {
		return 0
	}
	seen := make(map[uint64]struct{}, 4096)
	queue := []uint64{root}
	for len(queue) > 0 && len(seen) < flaskBarMaxWalk {
		e := queue[0]
		queue = queue[1:]
		if e < HeapLo || e >= HeapHi {
			continue
		}
		if _, dup := seen[e]; dup {
			continue
		}
		seen[e] = struct{}{}
		if ReadU64(r, e+ElementSelfOff) != e {
			continue
		}
		beg := ReadU64(r, e+ElementChildBegOff)
		end := ReadU64(r, e+ElementChildEndOff)
		if beg < HeapLo || end <= beg || end-beg > 0x4000 {
			continue
		}
		kids := int((end - beg) / 8)
		for i := range kids {
			queue = append(queue, ReadU64(r, beg+uint64(i)*8))
		}
		if kids != wantSlots || kids > flaskShapeMaxKids {
			continue
		}
		if flaskRowShaped(r, beg, kids) {
			return e
		}
	}
	return 0
}

func flaskRowShaped(r Reader, beg uint64, kids int) bool {
	var w0, h0 float32
	for i := range kids {
		c := ReadU64(r, beg+uint64(i)*8)
		if c < HeapLo || c >= HeapHi || ReadU64(r, c+ElementSelfOff) != c {
			return false
		}
		w, h := ElementSize(r, c)
		if w < flaskShapeMinPx || w > flaskShapeMaxPx || h < flaskShapeMinPx || h > flaskShapeMaxPx {
			return false
		}
		if i == 0 {
			w0, h0 = w, h
		} else if w != w0 || h != h0 {
			return false
		}
		if _, y, ok := ElementAbsPos(r, c); !ok || y < flaskShapeMinY {
			return false
		}
	}
	return true
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
