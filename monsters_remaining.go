package gamestate

import "strings"

func caesarUTF16(r Reader, addr uint64) string {
	t := ReadNativeUtf16TextStruct(r, addr)
	if t.Length == 0 || t.Length > 128 || t.Ptr < HeapLo || t.Ptr >= HeapHi {
		return ""
	}
	raw, err := r.ReadBytes(t.Ptr, int(t.Length)*2)
	if err != nil || len(raw) < int(t.Length)*2 {
		return ""
	}
	var b strings.Builder
	for i := 0; i+2 <= len(raw); i += 2 {
		c := uint16(raw[i]) | uint16(raw[i+1])<<8
		if c == 0 {
			break
		}
		d := int(c) + 29
		if d < 0x20 || d > 0x7E {
			return ""
		}
		b.WriteByte(byte(d))
	}
	return b.String()
}

func isCounterText(s string) bool {
	ls := strings.ToLower(s)
	return strings.Contains(ls, "monster") && strings.Contains(ls, "remain")
}

func FindMonstersRemainingElement(r Reader, gsoSlot uint64) uint64 {
	root, err := ResolveUiRoot(r, gsoSlot)
	if err != nil {
		return 0
	}
	seen := make(map[uint64]bool)
	stack := []uint64{root}
	for len(stack) > 0 {
		e := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if e < HeapLo || e >= HeapHi || seen[e] || len(seen) > 300000 {
			continue
		}
		seen[e] = true
		if ReadU64(r, e+ElementSelfOff) != e {
			continue
		}
		if isCounterText(caesarUTF16(r, e+ElementTextOff)) {
			return e
		}
		begin := ReadU64(r, e+ElementChildBegOff)
		end := ReadU64(r, e+ElementChildEndOff)
		if begin < HeapLo || begin >= HeapHi || end <= begin || end-begin > 0x20000 || (end-begin)%8 != 0 {
			continue
		}
		buf, err := r.ReadBytes(begin, int(end-begin))
		if err != nil {
			continue
		}
		for i := 0; i+8 <= len(buf); i += 8 {
			stack = append(stack, ReadU64(r, begin+uint64(i)))
		}
	}
	return 0
}

type MonstersRemaining struct {
	Text  string
	Count int
	Exact bool
}

func ReadMonstersRemaining(r Reader, elem uint64) (MonstersRemaining, bool) {
	if elem < HeapLo || elem >= HeapHi || ReadU64(r, elem+ElementSelfOff) != elem {
		return MonstersRemaining{}, false
	}
	s := caesarUTF16(r, elem+ElementTextOff)
	if !isCounterText(s) {
		return MonstersRemaining{}, false
	}
	m := MonstersRemaining{Text: s}
	if n, ok := leadingInt(s); ok {
		m.Count, m.Exact = n, true
	}
	return m, true
}

func leadingInt(s string) (int, bool) {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 {
		return 0, false
	}
	n := 0
	for _, c := range s[:i] {
		n = n*10 + int(c-'0')
	}
	return n, true
}
