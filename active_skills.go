package gamestate

const (
	cdElemStride = 0x48
	cdDatIDOff   = 0x08
	cdListBegOff = 0x10
	cdListEndOff = 0x18
	cdMaxUsesOff = 0x30
	cdTotalMsOff = 0x34
	cdListStride = 0x10
	cdMaxElems   = 64
	cdScanLimit  = 0x1200
	cdMinTotalMs = 100
	cdMaxTotalMs = 600000
	cdMaxDatID   = 60000
)

type ActiveSkill struct {
	Name       string
	DatID      int
	Cur        float32
	Max        float32
	MaxUses    int
	OnCooldown bool
}

type SkillReader struct {
	arrOff   uint64
	arrValid bool
}

func NewSkillReader() *SkillReader { return &SkillReader{} }

func (sr *SkillReader) Read(r Reader, actor uint64) []ActiveSkill {
	if actor < HeapLo || actor >= HeapHi {
		return nil
	}
	if !sr.arrValid || !sr.validArray(r, actor, sr.arrOff) {
		sr.arrOff, sr.arrValid = sr.findArray(r, actor)
		if !sr.arrValid {
			return nil
		}
	}
	begin := ReadU64(r, actor+sr.arrOff)
	end := ReadU64(r, actor+sr.arrOff+8)
	if begin < HeapLo || end <= begin {
		sr.arrValid = false
		return nil
	}
	n := int((end - begin) / cdElemStride)
	if n <= 0 || n > cdMaxElems {
		return nil
	}

	byID := make(map[int]ActiveSkill, n)
	order := make([]int, 0, n)
	for i := range n {
		obj := begin + uint64(i)*cdElemStride
		datID := int(ReadU32(r, obj+cdDatIDOff))
		if datID <= 0 || datID >= cdMaxDatID {
			continue
		}
		clBeg := ReadU64(r, obj+cdListBegOff)
		clEnd := ReadU64(r, obj+cdListEndOff)
		if clBeg < HeapLo || clEnd <= clBeg || clEnd-clBeg > 0x400 {
			continue
		}

		cnt := int((clEnd - clBeg) / cdListStride)
		rem, total := float32(-1), float32(0)
		for e := 0; e < cnt && e < 16; e++ {
			el := clBeg + uint64(e)*cdListStride
			rv := ReadFloat32(r, el)
			if rv <= 0.02 {
				continue
			}
			if rem < 0 || rv < rem {
				rem, total = rv, ReadFloat32(r, el+4)
			}
		}
		if rem < 0 {
			continue
		}
		if total <= 0 {
			total = float32(int32(ReadU32(r, obj+cdTotalMsOff))) / 1000
		}
		if prev, ok := byID[datID]; ok {
			if rem < prev.Cur {
				prev.Cur, prev.Max = rem, total
				byID[datID] = prev
			}
			continue
		}
		byID[datID] = ActiveSkill{
			DatID:      datID,
			Cur:        rem,
			Max:        total,
			MaxUses:    int(int32(ReadU32(r, obj+cdMaxUsesOff))),
			OnCooldown: true,
		}
		order = append(order, datID)
	}
	out := make([]ActiveSkill, 0, len(order))
	for _, id := range order {
		out = append(out, byID[id])
	}
	return out
}

func (sr *SkillReader) validArray(r Reader, actor, off uint64) bool {
	begin := ReadU64(r, actor+off)
	end := ReadU64(r, actor+off+8)
	if begin < HeapLo || begin >= HeapHi || end <= begin {
		return false
	}
	span := end - begin
	if span%cdElemStride != 0 || span/cdElemStride > cdMaxElems {
		return false
	}
	return sr.elementsLookValid(r, begin, int(span/cdElemStride))
}

func (sr *SkillReader) findArray(r Reader, actor uint64) (uint64, bool) {
	buf, err := r.ReadBytes(actor, cdScanLimit)
	if err != nil {
		return 0, false
	}
	for o := 0; o+16 <= len(buf); o += 8 {
		begin := readU64LE(buf, o)
		end := readU64LE(buf, o+8)
		if begin < HeapLo || begin >= HeapHi || end <= begin {
			continue
		}
		span := end - begin
		if span%cdElemStride != 0 {
			continue
		}
		n := int(span / cdElemStride)
		if n < 1 || n > cdMaxElems {
			continue
		}
		if sr.elementsLookValid(r, begin, n) {
			return uint64(o), true
		}
	}
	return 0, false
}

func (sr *SkillReader) elementsLookValid(r Reader, begin uint64, n int) bool {
	good := 0
	for i := 0; i < n && i < 6; i++ {
		obj := begin + uint64(i)*cdElemStride
		datID := int(ReadU32(r, obj+cdDatIDOff))
		totalMs := int(int32(ReadU32(r, obj+cdTotalMsOff)))
		if datID <= 0 || datID >= cdMaxDatID || totalMs < cdMinTotalMs || totalMs > cdMaxTotalMs {
			return false
		}
		good++
	}
	return good > 0
}

func readU64LE(b []byte, o int) uint64 {
	_ = b[o+7]
	return uint64(b[o]) | uint64(b[o+1])<<8 | uint64(b[o+2])<<16 | uint64(b[o+3])<<24 |
		uint64(b[o+4])<<32 | uint64(b[o+5])<<40 | uint64(b[o+6])<<48 | uint64(b[o+7])<<56
}
