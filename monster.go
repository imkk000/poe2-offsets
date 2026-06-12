package gamestate

import (
	"encoding/binary"
	"strings"
)

const (
	objectMagicPropsDetailsOff = 0xB0
	monsterRarityOffset        = 0x144

	monsterHPMaxOffset    = 0x1DC
	monsterHPCurOffset    = 0x1E0
	monsterMPMaxOffset    = 0x234
	monsterMPCurOffset    = 0x238
	monsterESMaxOffset    = 0x274
	monsterESCurOffset    = 0x278
	monsterBuffsVecOffset = 0x58
	renderNameOffset      = 0x160
	renderNameSizeOff     = 0x170
	renderNameCapOff      = 0x178
	renderNameSSOCap      = 7
	renderNameNamedOff    = 0x1B9

	entityIsValidOff = 0x84
)

var monsterRarityNames = []string{"White", "Magic", "Rare", "Unique"}

type MonsterStats struct {
	HPCur, HPMax int
	MPCur, MPMax int
	ESCur, ESMax int
	BuffCount    int
}

func ReadMonsterStats(r Reader, entity uint64) MonsterStats {
	var s MonsterStats
	comp := ResolveComponentByName(r, entity, "Life")
	if comp == 0 {
		return s
	}
	if b, err := r.ReadBytes(comp+monsterHPMaxOffset, 8); err == nil && len(b) >= 8 {
		s.HPMax = int(binary.LittleEndian.Uint32(b[0:4]))
		s.HPCur = int(binary.LittleEndian.Uint32(b[4:8]))
	}
	if b, err := r.ReadBytes(comp+monsterMPMaxOffset, 8); err == nil && len(b) >= 8 {
		s.MPMax = int(binary.LittleEndian.Uint32(b[0:4]))
		s.MPCur = int(binary.LittleEndian.Uint32(b[4:8]))
	}
	if b, err := r.ReadBytes(comp+monsterESMaxOffset, 8); err == nil && len(b) >= 8 {
		s.ESMax = int(binary.LittleEndian.Uint32(b[0:4]))
		s.ESCur = int(binary.LittleEndian.Uint32(b[4:8]))
	}
	if buffs := ResolveComponentByName(r, entity, "Buffs"); buffs != 0 {
		begin := ReadU64(r, buffs+buffsVecBeginOff)
		end := ReadU64(r, buffs+buffsVecBeginOff+8)
		if begin >= HeapLo && begin < HeapHi && end >= begin {
			n := int((end - begin) / 8)
			if n >= 0 && n < 100 {
				s.BuffCount = n
			}
		}
	}
	return s
}

func ReadMonsterRarity(r Reader, entity uint64) string {
	comp := ResolveComponentByName(r, entity, "ObjectMagicProperties")
	if comp == 0 {
		return ""
	}
	v := ReadByte(r, comp+monsterRarityOffset)
	if int(v) < len(monsterRarityNames) {
		return monsterRarityNames[v]
	}
	return ""
}

func ReadMonsterMods(r Reader, entity uint64) []ItemModEntry {
	comp := ResolveComponentByName(r, entity, "ObjectMagicProperties")
	if comp == 0 {
		return nil
	}
	return ReadItemModEntries(r, comp+objectMagicPropsDetailsOff)
}

func ReadIsNamed(r Reader, entity uint64) bool {
	comp := ResolveComponentByName(r, entity, "Render")
	if comp == 0 {
		return false
	}
	return ReadByte(r, comp+renderNameNamedOff) != 0
}

func ReadRenderName(r Reader, entity uint64) string {
	comp := ResolveComponentByName(r, entity, "Render")
	if comp == 0 {
		return ""
	}
	size := ReadU64(r, comp+renderNameSizeOff)
	cap_ := ReadU64(r, comp+renderNameCapOff)
	if size == 0 || size > 256 {
		return ""
	}
	want := int(size * 2)
	var raw []byte
	var err error
	if cap_ <= renderNameSSOCap {
		raw, err = r.ReadBytes(comp+renderNameOffset, want)
	} else {
		strPtr := ReadU64(r, comp+renderNameOffset)
		if !validDataPtr(strPtr) {
			return ""
		}
		raw, err = r.ReadBytes(strPtr, want)
	}
	if err != nil || len(raw) < want {
		return ""
	}
	var b strings.Builder
	for j := 0; j+2 <= len(raw); j += 2 {
		c := binary.LittleEndian.Uint16(raw[j : j+2])
		if c == 0 {
			break
		}
		if c < 0x20 || c > 0x7E {
			return ""
		}
		b.WriteByte(byte(c))
	}
	return b.String()
}

func ReadIsFriendly(r Reader, entity uint64) bool {
	v := ReadByte(r, entity+entityIsValidOff)
	return (v & 0x7F) == 0x01
}

func IsEntityInvalid(r Reader, entity uint64) bool {
	return ReadByte(r, entity+entityIsValidOff)&0x01 != 0
}

func IsAllyMonsterPath(path string) bool {
	switch {
	case strings.Contains(path, "PlayerSummoned"):
		return true
	case strings.Contains(path, "VaalMonsters/Living"):
		return true
	}
	return false
}

func ReadMonsterEffectiveness(r Reader, entity uint64) (int, bool) {
	comp := ResolveComponentByName(r, entity, "Monster")
	if comp == 0 {
		return 0, false
	}
	row := ReadU64(r, ReadU64(r, comp+0x18)+0x10)
	if row < HeapLo || row >= HeapHi {
		return 0, false
	}
	return int(ReadU32(r, row+0xA8)), true
}

func IsUsableCorpse(r Reader, entity uint64) bool {
	comp := ResolveComponentByName(r, entity, "Life")
	if comp == 0 {
		return false
	}
	if ReadByte(r, comp+0x3E2) == 0 || ReadU32(r, comp+0x1E0) > 0 {
		return false
	}
	sub := ReadU64(r, comp+0x190)
	if sub < HeapLo || sub >= HeapHi {
		return false
	}
	return ReadByte(r, sub+0xE6) == 0
}
