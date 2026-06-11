package gamestate

import (
	"encoding/binary"
	"slices"
)

const (
	ElementTextOff = 0x3E0
	ControlNameOff = 0x98

	controlWalkMax    = 20000
	controlChildLimit = 0x20000
	controlRefScanLen = 0x400

	stdStringSizeOff = 0x10
	stdStringCapOff  = 0x18
	stdStringSSOCap  = 8
	stdStringMaxLen  = 256
)

type ControlText struct {
	Element uint64
	Text    string
}

func DecodeCaesarText(r Reader, structAddr uint64) string {
	return caesarUTF16(r, structAddr)
}

func readStdWString(r Reader, str uint64) string {
	size := ReadU64(r, str+stdStringSizeOff)
	if size == 0 || size > stdStringMaxLen {
		return ""
	}
	data := str
	if ReadU64(r, str+stdStringCapOff) >= stdStringSSOCap {
		data = ReadU64(r, str)
		if data < HeapLo || data >= HeapHi {
			return ""
		}
	}
	raw, err := r.ReadBytes(data, int(size)*2)
	if err != nil || len(raw) < int(size)*2 {
		return ""
	}
	out := make([]byte, 0, size)
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
	return string(out)
}

func ReadControlName(r Reader, control uint64) string {
	if control < HeapLo || control >= HeapHi {
		return ""
	}
	return readStdWString(r, control+ControlNameOff)
}

func FindControlByName(r Reader, root uint64, name string) uint64 {
	found := uint64(0)
	walkControlTree(r, root, func(e uint64) bool {
		if ReadControlName(r, e) == name {
			found = e
			return false
		}
		return true
	})
	return found
}

func validElement(r Reader, e uint64) bool {
	return e >= HeapLo && e < HeapHi && ReadU64(r, e+ElementSelfOff) == e
}

func walkControlTree(r Reader, root uint64, visit func(e uint64) bool) {
	if !validElement(r, root) {
		return
	}
	seen := map[uint64]struct{}{root: {}}
	queue := []uint64{root}
	for len(queue) > 0 && len(seen) < controlWalkMax {
		e := queue[0]
		queue = queue[1:]
		if !visit(e) {
			return
		}
		begin := ReadU64(r, e+ElementChildBegOff)
		end := ReadU64(r, e+ElementChildEndOff)
		if begin < HeapLo || begin >= HeapHi || end <= begin || end-begin > controlChildLimit || (end-begin)%8 != 0 {
			continue
		}
		buf, err := r.ReadBytes(begin, int(end-begin))
		if err != nil {
			continue
		}
		for i := 0; i+8 <= len(buf); i += 8 {
			c := binary.LittleEndian.Uint64(buf[i:])
			if _, ok := seen[c]; ok {
				continue
			}
			if !validElement(r, c) {
				continue
			}
			seen[c] = struct{}{}
			queue = append(queue, c)
		}
	}
}

func ControlSubtreeText(r Reader, control uint64) []ControlText {
	var out []ControlText
	walkControlTree(r, control, func(e uint64) bool {
		if s := caesarUTF16(r, e+ElementTextOff); s != "" {
			out = append(out, ControlText{Element: e, Text: s})
		}
		return true
	})
	return out
}

func FindControlsByRef(r Reader, root, target uint64) []uint64 {
	var out []uint64
	walkControlTree(r, root, func(e uint64) bool {
		buf, err := r.ReadBytes(e, controlRefScanLen)
		if err != nil {
			return true
		}
		for off := 0; off+8 <= len(buf); off += 8 {
			if binary.LittleEndian.Uint64(buf[off:]) == target {
				out = append(out, e)
				return true
			}
		}
		return true
	})
	return out
}

func FindControlByRef(r Reader, root, target uint64) uint64 {
	if c := FindControlsByRef(r, root, target); len(c) > 0 {
		return c[0]
	}
	return 0
}

func ReadHUDVitalsText(r Reader, gsoSlot uint64) []ControlText {
	root, err := ResolveTrueUiRoot(r, gsoSlot)
	if err != nil {
		return nil
	}
	player, err := ResolveLocalPlayer(r, gsoSlot)
	if err != nil {
		return nil
	}
	life := ResolveComponentByName(r, player, "Life")
	if life == 0 {
		return nil
	}
	var best []ControlText
	for _, ctrl := range FindControlsByRef(r, root, life) {
		if t := ControlSubtreeText(r, ctrl); len(t) > len(best) {
			best = t
		}
	}
	return best
}

func ReadControlTextByName(r Reader, gsoSlot uint64, name string) []ControlText {
	root, err := ResolveTrueUiRoot(r, gsoSlot)
	if err != nil {
		return nil
	}
	ctrl := FindControlByName(r, root, name)
	if ctrl == 0 {
		return nil
	}
	return ControlSubtreeText(r, ctrl)
}

func ReadQuestTracker(r Reader, gsoSlot uint64) []ControlText {
	return ReadControlTextByName(r, gsoSlot, "quest_selector")
}

var itemModMarkers = []string{"increased", "reduced", "to maximum", "added", "resistance",
	"requires", "critical", "attack speed", "cast speed", "energy shield", "% more", "% less"}

func isItemModText(s string) bool {
	if len(s) < 4 {
		return false
	}
	low := toLowerASCII(s)
	for _, m := range itemModMarkers {
		if containsASCII(low, m) {
			return true
		}
	}
	return false
}

func toLowerASCII(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

func containsASCII(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

const (
	itemTooltipMaxHops     = 14
	itemTooltipMinLines    = 4
	itemTooltipClusterSpan = 0x8000
)

func FindItemTooltip(r Reader, gsoSlot uint64) uint64 {
	root, err := ResolveTrueUiRoot(r, gsoSlot)
	if err != nil {
		return 0
	}
	var modNodes []uint64
	walkControlTree(r, root, func(e uint64) bool {
		if isItemModText(caesarUTF16(r, e+ElementTextOff)) {
			modNodes = append(modNodes, e)
		}
		return true
	})
	if len(modNodes) < itemTooltipMinLines {
		return 0
	}
	slices.Sort(modNodes)
	lo, n := 0, 0
	for hi, j := 0, 0; hi < len(modNodes); hi++ {
		for modNodes[hi]-modNodes[j] > itemTooltipClusterSpan {
			j++
		}
		if hi-j+1 > n {
			n, lo = hi-j+1, j
		}
	}
	if n < itemTooltipMinLines {
		return 0
	}
	modNodes = modNodes[lo : lo+n]
	count := make(map[uint64]int)
	hopSum := make(map[uint64]int)
	for _, n := range modNodes {
		e := n
		for h := 1; h <= itemTooltipMaxHops; h++ {
			p := ReadU64(r, e+ElementParentOff)
			if p < HeapLo || p >= HeapHi {
				break
			}
			count[p]++
			hopSum[p] += h
			e = p
		}
	}
	best := uint64(0)
	bestCount, bestHops := 0, 1<<30
	for a, c := range count {
		if c > bestCount || (c == bestCount && hopSum[a] < bestHops) {
			best, bestCount, bestHops = a, c, hopSum[a]
		}
	}
	return best
}

func ReadHoveredItemText(r Reader, gsoSlot uint64) []ControlText {
	tip := FindItemTooltip(r, gsoSlot)
	if tip == 0 {
		return nil
	}
	return ControlSubtreeText(r, tip)
}
