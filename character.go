package gamestate

import "encoding/binary"

const (
	charNameOff     = 0x1B0
	charNameSizeOff = 0x1C0
	charNameCapOff  = 0x1C8
	charNameSSOCap  = 7
	charNameMaxLen  = 32
	charCurEXPOff   = 0x1D8
	charLevelOff    = 0x204
)

type Character struct {
	Name     string `json:"name,omitempty"`
	Level    int    `json:"level,omitempty"`
	CurEXP   uint32 `json:"cur_exp,omitempty"`
	LevelEXP uint32 `json:"level_exp,omitempty"`
	NextEXP  uint32 `json:"next_exp,omitempty"`
	LevelMax int    `json:"level_max,omitempty"`
}

var ExpForLevel = [...]uint64{
	0,
	0, 525, 1760, 3781, 7184, 12186, 19324, 29377, 43181, 61693,
	85990, 117506, 157384, 207736, 269997, 346462, 439268, 551295, 685171, 843709,
	1030734, 1249629, 1504995, 1800847, 2142652, 2535122, 2984677, 3496798, 4080655, 4742836,
	5490247, 6334393, 7283446, 8348398, 9541110, 10874351, 12361842, 14018289, 15859432, 17905634,
	20171471, 22679999, 25456123, 28517857, 31897771, 35621447, 39721017, 44225461, 49176560, 54607467,
	60565335, 67094245, 74247659, 82075627, 90631041, 99984974, 110197515, 121340161, 133497202, 146749362,
	161191120, 176922628, 194049893, 212684946, 232956711, 255001620, 278952403, 304972236, 333233648, 363906163,
	397194041, 433312945, 472476370, 514937180, 560961898, 610815862, 664824416, 723298169, 786612664, 855129128,
	929261318, 1009443795, 1096169525, 1189918242, 1291270350, 1400795257, 1519130326, 1646943474, 1784977296, 1934009687,
	2094900291, 2268549086, 2455921256, 2658074992, 2876116901, 3111280300, 3364828162, 3638186694, 3932818530, 4250334444,
}

func ReadCharacter(r Reader, entity uint64) Character {
	var c Character

	comp := ResolveComponentByName(r, entity, "Player")
	if comp == 0 {
		return c
	}
	c.Name = readCharName(r, comp)
	c.CurEXP = uint32(readU32(r, comp+charCurEXPOff))
	c.Level = int(ReadByte(r, comp+charLevelOff))
	c.LevelMax = 100
	if c.Level > 0 && c.Level < len(ExpForLevel) {
		c.LevelEXP = uint32(ExpForLevel[c.Level])
	}
	if c.Level > 0 && c.Level < len(ExpForLevel)-1 {
		c.NextEXP = uint32(ExpForLevel[c.Level+1])
	}
	return c
}

func readUTF16String(r Reader, addr uint64, maxChars int) string {
	buf, err := r.ReadBytes(addr, maxChars*2)
	if err != nil || len(buf) < 2 {
		return ""
	}
	var out []byte
	for i := 0; i+2 <= len(buf); i += 2 {
		c := binary.LittleEndian.Uint16(buf[i : i+2])
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

func readCharName(r Reader, comp uint64) string {
	size := ReadU64(r, comp+charNameSizeOff)
	cap_ := ReadU64(r, comp+charNameCapOff)
	if size == 0 || size > charNameMaxLen {
		return ""
	}
	want := int(size) * 2
	var raw []byte
	var err error
	if cap_ <= charNameSSOCap {
		raw, err = r.ReadBytes(comp+charNameOff, want)
	} else {
		ptr := ReadU64(r, comp+charNameOff)
		if !validDataPtr(ptr) {
			return ""
		}
		raw, err = r.ReadBytes(ptr, want)
	}
	if err != nil || len(raw) < want {
		return ""
	}
	return decodeUTF16ToUTF8(raw)
}

func decodeUTF16ToUTF8(raw []byte) string {
	out := make([]byte, 0, len(raw))
	for i := 0; i+2 <= len(raw); i += 2 {
		c := binary.LittleEndian.Uint16(raw[i : i+2])
		if c == 0 {
			break
		}
		switch {
		case c < 0x80:
			out = append(out, byte(c))
		case c < 0x800:
			out = append(out, 0xC0|byte(c>>6), 0x80|byte(c&0x3F))
		default:
			out = append(out, 0xE0|byte(c>>12), 0x80|byte((c>>6)&0x3F), 0x80|byte(c&0x3F))
		}
	}
	return string(out)
}
