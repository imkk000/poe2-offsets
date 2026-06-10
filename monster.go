package gamestate

import (
	"encoding/binary"
	"strings"
)

const (
	// ObjectMagicProperties holds a ModsAndObjectMagicProperties details block at
	// +0xB0 (reference layout) — the SAME struct as the item Mods component, so rarity is
	// at +0xB0+0x94 = +0x144 and the 5-slot mod array at +0xB0+0xA0 = +0x150.
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

// ReadMonsterMods reads a monster's affixes/properties from its
// ObjectMagicProperties component. The details block at +0xB0 mirrors the item
// Mods component, so the item mod-entry reader decodes it directly: each entry is
// a mod ID (e.g. MonsterFireResistance, MonsterFast) + rolled value(s). Verified
// live 2026-06-10 (minion mods: SkeletonWarriorPlayerMinionBlockChance=[30,0]).
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
