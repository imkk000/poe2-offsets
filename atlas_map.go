package gamestate

import "strings"

const (
	atlasNodeIDOff    = 0x300
	atlasContentOff   = 0x310
	atlasStateOff     = 0x32c
	atlasBiomeIdxOff  = 0x32e
	atlasFlagsOff     = 0x32f
	atlasContentVecB  = 0x350
	atlasContentVecE  = 0x358
	atlasScaleOff     = 0x130
	atlasMaxWalk      = 250000
	atlasMaxDepth     = 24
	atlasContentDepth = 4
	atlasContentSteps = 1500
	structSweep       = 0x340

	atlasNodeCompIDOff  = 0x339
	atlasMgrGlobalBelow = 0x10
	atlasMgrSelectorOff = 0x348
	atlasMgrVecOff      = 0x48
	atlasLeagueCompOff  = 0x188
	atlasCompletionSet  = 0x260
)

type AtlasNode struct {
	ID          string
	X, Y        float32
	W, H        float32
	Scale       float32
	Elem        uint64
	Container   uint64
	BiomeIndex  int
	State       int
	Completed   bool
	ContentKeys []int
}

func (n AtlasNode) Unlocked() bool { return n.State&0x1 != 0 }
func (n AtlasNode) Visited() bool  { return n.State&0x2 != 0 }

func atlasCompletionContains(set []byte, v uint32) bool {
	n := len(set) / 9
	key := byte((v >> 6) & 0xff)
	lo, hi := 0, n
	for lo < hi {
		mid := (lo + hi) / 2
		if set[mid*9] < key {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo >= n || set[lo*9] != key {
		return false
	}
	bit := v & 0x3f
	return set[lo*9+1+int(bit>>3)]&(1<<(bit&7)) != 0
}

type atlasCompletionChain struct {
	gsoSlotDelta int64
	selOff       uint64
	vecOff       uint64
	csOff        uint64
	setOff       uint64
}

func applyCompletionChain(r Reader, gsoSlot uint64, ch atlasCompletionChain) []byte {
	mgrInner := ReadU64(r, uint64(int64(gsoSlot)+ch.gsoSlotDelta))
	if mgrInner < HeapLo || mgrInner >= HeapHi {
		return nil
	}
	selector := ReadU64(r, mgrInner+ch.selOff)
	if selector < HeapLo || selector >= HeapHi {
		return nil
	}
	vecBegin := ReadU64(r, selector+ch.vecOff)
	if vecBegin < HeapLo || vecBegin >= HeapHi {
		return nil
	}
	league := ReadU64(r, vecBegin)
	if league < HeapLo || league >= HeapHi {
		return nil
	}
	cs := ReadU64(r, league+ch.csOff)
	if cs < HeapLo || cs >= HeapHi {
		return nil
	}
	begin := ReadU64(r, cs+ch.setOff)
	end := ReadU64(r, cs+ch.setOff+8)
	if begin < HeapLo || begin >= HeapHi || end < begin || end >= HeapHi {
		return nil
	}
	span := end - begin
	if span == 0 || span%9 != 0 || span > 0x20000 {
		return nil
	}
	buf, err := r.ReadBytes(begin, int(span))
	if err != nil || len(buf) < int(span) {
		return nil
	}

	for i := 9; i+9 <= len(buf); i += 9 {
		if buf[i] < buf[i-9] {
			return nil
		}
	}

	n := len(buf) / 9
	if n < 5 || n > 50 {
		return nil
	}
	return buf
}

func sweepCompletionChain(r Reader, gsoSlot uint64) *atlasCompletionChain {
	centre := atlasCompletionChain{
		gsoSlotDelta: -int64(atlasMgrGlobalBelow),
		selOff:       atlasMgrSelectorOff,
		vecOff:       atlasMgrVecOff,
		csOff:        atlasLeagueCompOff,
		setOff:       atlasCompletionSet,
	}
	if buf := applyCompletionChain(r, gsoSlot, centre); buf != nil {
		return &centre
	}
	for gD := int64(-0x18); gD <= 0x18; gD += 8 {
		mgrInner := ReadU64(r, uint64(int64(gsoSlot)+centre.gsoSlotDelta+gD))
		if mgrInner < HeapLo || mgrInner >= HeapHi {
			continue
		}
		for sO := centre.selOff - 0x10; sO <= centre.selOff+0x10; sO += 8 {
			selector := ReadU64(r, mgrInner+sO)
			if selector < HeapLo || selector >= HeapHi {
				continue
			}
			for vO := centre.vecOff - 0x8; vO <= centre.vecOff+0x8; vO += 8 {
				vecBegin := ReadU64(r, selector+vO)
				if vecBegin < HeapLo || vecBegin >= HeapHi {
					continue
				}
				league := ReadU64(r, vecBegin)
				if league < HeapLo || league >= HeapHi {
					continue
				}
				for cO := centre.csOff - 0x8; cO <= centre.csOff+0x8; cO += 8 {
					cs := ReadU64(r, league+cO)
					if cs < HeapLo || cs >= HeapHi {
						continue
					}

					for sO2 := centre.setOff - 0x8; sO2 <= centre.setOff+0x8; sO2 += 8 {
						candidate := atlasCompletionChain{
							gsoSlotDelta: centre.gsoSlotDelta + gD,
							selOff:       sO,
							vecOff:       vO,
							csOff:        cO,
							setOff:       sO2,
						}
						if buf := applyCompletionChain(r, gsoSlot, candidate); buf != nil {
							return &candidate
						}
					}
				}
			}
		}
	}
	return nil
}

func ReadAtlasNodePos(r Reader, elem, container uint64, requireVisible bool) (x, y float32, ok bool) {
	if elem < HeapLo || elem >= HeapHi || ReadU64(r, elem+ElementSelfOff) != elem {
		return 0, 0, false
	}
	if requireVisible && !atlasEffectiveVisible(r, elem) {
		return 0, 0, false
	}
	x, y, _ = atlasChainScreen(r, elem, container)
	return x, y, true
}

type AtlasMapReader struct {
	elems     []uint64
	container uint64
	chain     *atlasCompletionChain
}

func NewAtlasMapReader() *AtlasMapReader { return &AtlasMapReader{} }

func (ar *AtlasMapReader) Read(r Reader, gsoSlot uint64, known map[string]bool, showAll bool) []AtlasNode {
	if len(known) == 0 {
		return nil
	}
	if !ar.cacheValid(r, known) {
		root, err := ResolveTrueUiRoot(r, gsoSlot)
		if err != nil || root == 0 {
			ar.elems = nil
			return nil
		}
		ar.elems = ar.findNodes(r, root, known)
	}

	if showAll {
		open := false
		for _, e := range ar.elems {
			if atlasEffectiveVisible(r, e) {
				open = true
				break
			}
		}
		if !open {
			return nil
		}
	}

	completionSet := ar.resolveCompletionSet(r, gsoSlot)
	out := make([]AtlasNode, 0, len(ar.elems))
	for _, e := range ar.elems {
		if !showAll && !atlasEffectiveVisible(r, e) {
			continue
		}
		id := readAtlasMapID(r, e)
		if id == "" || !known[id] {
			continue
		}
		x, y, sc := atlasChainScreen(r, e, ar.container)
		w := ReadFloat32(r, e+ElementSizeOff)
		h := ReadFloat32(r, e+ElementSizeOff+4)

		bi := -1
		if b := ReadByte(r, e+atlasBiomeIdxOff); b != 0xFF {
			bi = int(b)
		}
		state := int(ReadByte(r, e+atlasFlagsOff) & 3)
		completed := completionSet != nil && atlasCompletionContains(completionSet, ReadU32(r, e+atlasNodeCompIDOff))

		var keys []int
		if vb, ve := ReadU64(r, e+atlasContentVecB), ReadU64(r, e+atlasContentVecE); vb >= HeapLo && vb < HeapHi && ve > vb && ve-vb < 0x1000 {
			if buf, err := r.ReadBytes(vb, int(ve-vb)); err == nil {
				for i := 0; i+4 <= len(buf); i += 4 {
					keys = append(keys, int(buf[i])|int(buf[i+1])<<8)
				}
			}
		}
		out = append(out, AtlasNode{ID: id, X: x, Y: y, W: w * sc, H: h * sc, Scale: sc, Elem: e, Container: ar.container, BiomeIndex: bi, State: state, Completed: completed, ContentKeys: keys})
	}
	return out
}

func (ar *AtlasMapReader) resolveCompletionSet(r Reader, gsoSlot uint64) []byte {
	if ar.chain != nil {
		if buf := applyCompletionChain(r, gsoSlot, *ar.chain); buf != nil {
			return buf
		}
		ar.chain = nil
	}
	ar.chain = sweepCompletionChain(r, gsoSlot)
	if ar.chain == nil {
		return nil
	}
	return applyCompletionChain(r, gsoSlot, *ar.chain)
}

func (ar *AtlasMapReader) cacheValid(r Reader, known map[string]bool) bool {
	if len(ar.elems) == 0 {
		return false
	}

	step := 1 + len(ar.elems)/16
	for i := 0; i < len(ar.elems); i += step {
		e := ar.elems[i]
		if ReadU64(r, e+ElementSelfOff) != e {
			return false
		}
		if id := readAtlasMapID(r, e); id == "" || !known[id] {
			return false
		}
	}
	return true
}

func (ar *AtlasMapReader) findNodes(r Reader, root uint64, known map[string]bool) []uint64 {
	var out []uint64
	seen := make(map[uint64]bool)
	n := 0
	var walk func(e, parent uint64, depth int)
	walk = func(e, parent uint64, depth int) {
		if e < HeapLo || e >= HeapHi || depth > atlasMaxDepth || seen[e] || n > atlasMaxWalk {
			return
		}
		seen[e] = true
		n++
		if ReadU64(r, e+ElementSelfOff) == e {
			if id := readAtlasMapID(r, e); id != "" && known[id] {
				out = append(out, e)
				if ar.container == 0 {
					ar.container = parent
				}
			}
		}
		begin := ReadU64(r, e+ElementChildBegOff)
		end := ReadU64(r, e+ElementChildEndOff)
		if begin < HeapLo || end <= begin || end-begin > 0x20000 {
			return
		}
		buf, err := r.ReadBytes(begin, int((end-begin)/8)*8)
		if err != nil {
			return
		}
		for i := 0; i+8 <= len(buf); i += 8 {
			walk(ReadU64Bytes(buf, uint64(i)), e, depth+1)
		}
	}
	ar.container = 0
	walk(root, 0, 0)
	return out
}

func ScanAtlasContent(r Reader, node uint64, want map[string]string) []string {
	root := ReadU64(r, node+atlasContentOff)
	if root < HeapLo || root >= HeapHi {
		return nil
	}
	visited := make(map[uint64]bool)
	found := make(map[string]bool)
	var out []string
	type qn struct {
		a uint64
		d int
	}
	q := []qn{{root, 0}}
	steps := 0
	for len(q) > 0 {
		c := q[0]
		q = q[1:]
		if c.a < HeapLo || c.a >= HeapHi || visited[c.a] || c.d > atlasContentDepth || steps > atlasContentSteps {
			continue
		}
		visited[c.a] = true
		steps++
		if s := readAtlasPathStr(r, c.a); s != "" {
			for kw, disp := range want {
				if !found[disp] && strings.Contains(s, kw) {
					found[disp] = true
					out = append(out, disp)
				}
			}
		}
		buf, err := r.ReadBytes(c.a, structSweep)
		if err != nil {
			continue
		}
		for o := 0; o+8 <= len(buf); o += 8 {
			v := ReadU64Bytes(buf, uint64(o))
			if v >= HeapLo && v < HeapHi && !visited[v] {
				q = append(q, qn{v, c.d + 1})
			}
		}
	}
	return out
}

func readAtlasPathStr(r Reader, p uint64) string {
	raw, err := r.ReadBytes(p, 320)
	if err != nil || len(raw) < 8 {
		return ""
	}
	out := make([]byte, 0, 160)
	for i := 0; i+2 <= len(raw); i += 2 {
		c := uint16(raw[i]) | uint16(raw[i+1])<<8
		if c == 0 {
			break
		}
		if c < 0x20 || c > 0x7E {
			return ""
		}
		out = append(out, byte(c))
	}
	if len(out) < 6 {
		return ""
	}
	return string(out)
}

func atlasEffectiveVisible(r Reader, e uint64) bool {
	for hop := 0; hop < 16 && e >= HeapLo && e < HeapHi; hop++ {
		if ReadU32(r, e+ElementFlagsOff)&(1<<0x0B) == 0 {
			return false
		}
		e = ReadU64(r, e+ElementParentOff)
	}
	return true
}

func readAtlasMapID(r Reader, node uint64) string {
	a1 := ReadU64(r, node+atlasNodeIDOff)
	if a1 < HeapLo || a1 >= HeapHi {
		return ""
	}
	a2 := ReadU64(r, a1)
	if a2 < HeapLo || a2 >= HeapHi {
		return ""
	}
	a3 := ReadU64(r, a2)
	if a3 < HeapLo || a3 >= HeapHi {
		return ""
	}
	return readAtlasUTF16(r, a3)
}

func readAtlasUTF16(r Reader, p uint64) string {
	raw, err := r.ReadBytes(p, 96)
	if err != nil || len(raw) < 6 {
		return ""
	}
	out := make([]byte, 0, 48)
	for i := 0; i+2 <= len(raw); i += 2 {
		c := uint16(raw[i]) | uint16(raw[i+1])<<8
		if c == 0 {
			break
		}
		if c < 0x20 || c > 0x7E {
			return ""
		}
		out = append(out, byte(c))
	}
	if len(out) < 3 {
		return ""
	}
	return string(out)
}

func atlasChainScreen(r Reader, node, container uint64) (x, y, scale float32) {
	x = ReadFloat32(r, node+ElementPositionOff)
	y = ReadFloat32(r, node+ElementPositionOff+4)
	scale = 1.0
	if container >= HeapLo && container < HeapHi {
		if ps := ReadFloat32(r, container+atlasScaleOff); ps > 0 && ps <= 8 {
			scale = ps
			x = x*ps + ReadFloat32(r, container+ElementPositionOff)
			y = y*ps + ReadFloat32(r, container+ElementPositionOff+4)
		}
	}
	return x, y, scale
}
