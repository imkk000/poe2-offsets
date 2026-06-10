package gamestate

import "encoding/binary"

const (
	buffsVecBeginOff       = 0x160
	statusEffectBuffDefOff = 0x08
	buffDefNamePtrOff      = 0x00
	buffDefSpiritFlatOff   = 0xC4
	buffDefSpiritPctOff    = 0x1F4
	buffDefNameMaxBytes    = 128

	maxPlayerBuffs  = 64
	maxMonsterBuffs = 32
)

type PlayerBuff struct {
	Template    string `json:"template"`
	TemplateVT  string `json:"template_vt"`
	Reserved    int    `json:"reserved,omitempty"`
	ReservedPct int    `json:"reserved_pct,omitempty"`
	Label       string `json:"label,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Invisible   bool   `json:"invisible,omitempty"`
}

func ReadPlayerBuffs(r Reader, entity uint64, spiritMax int) ([]PlayerBuff, int) {
	return readBuffsFromEntity(r, entity, spiritMax, maxPlayerBuffs)
}

func ReadMonsterBuffs(r Reader, entity uint64) []PlayerBuff {
	out, _ := readBuffsFromEntity(r, entity, 0, maxMonsterBuffs)
	return out
}

func readBuffsFromEntity(r Reader, entity uint64, spiritMax, cap int) ([]PlayerBuff, int) {
	comp := ResolveComponentByName(r, entity, "Buffs")
	if comp == 0 {
		return nil, 0
	}
	begin := ReadU64(r, comp+buffsVecBeginOff)
	end := ReadU64(r, comp+buffsVecBeginOff+8)
	if begin < HeapLo || end < begin {
		return nil, 0
	}
	n := int((end - begin) / 8)
	if n <= 0 || n > cap {
		return nil, 0
	}
	data, err := r.ReadBytes(begin, n*8)
	if err != nil || len(data) < n*8 {
		return nil, 0
	}
	out := make([]PlayerBuff, 0, n)
	total := 0
	for i := range n {
		se := binary.LittleEndian.Uint64(data[i*8 : i*8+8])
		if se < HeapLo || se >= HeapHi {
			continue
		}
		bdef := ReadU64(r, se+statusEffectBuffDefOff)
		if bdef < HeapLo || bdef >= HeapHi {
			continue
		}
		namePtr := ReadU64(r, bdef+buffDefNamePtrOff)
		flat := readU32(r, bdef+buffDefSpiritFlatOff)
		pct := readU32(r, bdef+buffDefSpiritPctOff)
		reserved := 0
		if spiritMax > 0 && flat > 0 && flat <= spiritMax {
			reserved = flat
			total += flat
		}
		pb := PlayerBuff{
			Template:   formatHex(bdef),
			TemplateVT: formatHex(namePtr),
			Reserved:   reserved,
		}
		if pct > 0 && pct <= 100 {
			pb.ReservedPct = pct
		}
		pb.Label = readBuffName(r, namePtr)
		if pb.Label != "" {
			if entry, ok := BuffByID(pb.Label); ok {
				pb.Name = entry.Name
				pb.Description = entry.Description
				pb.Invisible = entry.Invisible
			}
		}
		out = append(out, pb)
	}
	return out, total
}

func readBuffName(r Reader, namePtr uint64) string {
	if namePtr < HeapLo || namePtr >= HeapHi {
		return ""
	}
	raw, err := r.ReadBytes(namePtr, buffDefNameMaxBytes)
	if err != nil || len(raw) < 2 {
		return ""
	}
	out := make([]byte, 0, len(raw)/2)
	for j := 0; j+2 <= len(raw); j += 2 {
		c := binary.LittleEndian.Uint16(raw[j : j+2])
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

func formatHex(v uint64) string {
	const digits = "0123456789ABCDEF"
	if v == 0 {
		return "0"
	}
	var buf [16]byte
	n := 0
	for v > 0 {
		buf[n] = digits[v&0xF]
		v >>= 4
		n++
	}
	out := make([]byte, n)
	for i := range n {
		out[i] = buf[n-1-i]
	}
	return string(out)
}
