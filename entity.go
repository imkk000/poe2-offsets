package gamestate

import (
	"encoding/binary"
	"strings"
)

func ReadEntityMetadata(r Reader, entity uint64) string {
	details := ReadU64(r, entity+0x08)
	if details < HeapLo || details >= HeapHi {
		return ""
	}
	strPtr := ReadU64(r, details+0x08)
	if strPtr < HeapLo || strPtr >= HeapHi {
		return ""
	}
	buf, err := r.ReadBytes(strPtr, 256)
	if err != nil || len(buf) < 2 {
		return ""
	}
	var b strings.Builder
	for i := 0; i+2 <= len(buf); i += 2 {
		c := binary.LittleEndian.Uint16(buf[i : i+2])
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

func ClassifyEntity(path string) string {
	switch {
	case strings.HasPrefix(path, "Metadata/Monsters/MonsterMods/"),
		strings.HasPrefix(path, "Metadata/Monsters/Anomalies/"):

		return ""
	case strings.HasPrefix(path, "Metadata/Monsters/LeagueExpeditionNew/"):

		return "Expedition"
	case strings.HasPrefix(path, "Metadata/Monsters/NPC/"):
		return "NPC"
	case strings.HasPrefix(path, "Metadata/Monsters/LeagueAzmeri/GuidingLight"):
		return "AzmeriSpirit"
	case strings.HasPrefix(path, "Metadata/Monsters/TormentedSpirits/"):
		return "TormentedSpirit"
	case strings.Contains(path, "PlayerSummoned"):
		return "Minion"
	case strings.HasPrefix(path, "Metadata/Monsters/"):
		return "Monster"
	case strings.HasPrefix(path, "Metadata/MiscellaneousObjects/WorldItem"):
		return "WorldItem"
	case strings.HasPrefix(path, "Metadata/MiscellaneousObjects/Checkpoint"):
		return "Checkpoint"
	case strings.HasPrefix(path, "Metadata/MiscellaneousObjects/Portal"),
		strings.HasPrefix(path, "Metadata/MiscellaneousObjects/MultiplexPortal"):
		return "Portal"
	case strings.HasPrefix(path, "Metadata/MiscellaneousObjects/Shrine"),
		strings.HasPrefix(path, "Metadata/Shrines/"):
		return "Shrine"
	case strings.HasPrefix(path, "Metadata/MiscellaneousObjects/AreaTransition"):
		return "AreaTransition"
	case strings.HasPrefix(path, "Metadata/MiscellaneousObjects/Waypoint"):
		return "Waypoint"
	case strings.HasPrefix(path, "Metadata/MiscellaneousObjects/Stash"),
		strings.HasPrefix(path, "Metadata/MiscellaneousObjects/GuildStash"):
		return "Stash"
	case strings.HasPrefix(path, "Metadata/MiscellaneousObjects/HealingWell"):
		return "Well"
	case strings.HasPrefix(path, "Metadata/MiscellaneousObjects/Monolith"):
		return "Monolith"
	case strings.HasPrefix(path, "Metadata/MiscellaneousObjects/Sanctum/SanctumLocker"):
		return "Locker"
	case strings.HasPrefix(path, "Metadata/MiscellaneousObjects/Abyss/AbyssFinalNodeBase"):
		return "AbyssNode"
	case strings.HasPrefix(path, "Metadata/MiscellaneousObjects/Abyss/"):
		return "Abyss"
	case strings.HasPrefix(path, "Metadata/MiscellaneousObjects/Expedition") &&
		strings.Contains(path, "Encounter"):
		return "Expedition"
	case strings.HasPrefix(path, "Metadata/MiscellaneousObjects/LeagueIncursionNew/IncursionPedestal"):

		return "VaalBeacon"
	case strings.HasPrefix(path, "Metadata/MiscellaneousObjects/"):
		return "Misc"
	case strings.HasPrefix(path, "Metadata/Chests/StrongBoxes/"):
		return "Strongbox"
	case strings.HasPrefix(path, "Metadata/Chests/Abyss/"):

		return "AbyssChest"
	case strings.HasPrefix(path, "Metadata/Chests/QuestChests/"):

		return "QuestObject"
	case strings.HasPrefix(path, "Metadata/Chests/"):
		return "Chest"
	case strings.HasPrefix(path, "Metadata/NPC/") && strings.Contains(path, "Glyph"):
		return "Glyph"
	case strings.HasPrefix(path, "Metadata/NPC/"):
		return "NPC"
	case strings.HasPrefix(path, "Metadata/Characters/"):
		return "Player"
	case strings.HasPrefix(path, "Metadata/Items/"):
		return "Item"
	case strings.HasPrefix(path, "Metadata/Effects/"):
		return "Effect"
	case strings.Contains(path, "RitualAltar"):
		return "Ritual"
	case strings.HasPrefix(path, "Metadata/QuestObjects/"):

		return "QuestObject"
	case strings.HasPrefix(path, "Metadata/Terrain/") &&
		strings.Contains(path, "/Objects/") &&
		strings.Contains(path, "Portal"):

		return "Portal"
	case strings.HasPrefix(path, "Metadata/Terrain/") &&
		strings.Contains(path, "/Objects/") &&
		strings.Contains(path, "Expedition"):

		return "Expedition"
	case strings.HasPrefix(path, "Metadata/Terrain/") &&
		strings.Contains(path, "/Objects/") &&
		strings.Contains(path, "Transition"):

		return "AreaTransition"
	case strings.HasPrefix(path, "Metadata/Terrain/") &&
		strings.Contains(path, "/Objects/") &&
		isInteractiveTerrainObject(path):

		return "QuestObject"
	case strings.HasPrefix(path, "Metadata/Terrain/") &&
		strings.Contains(path, "/Objects/"):

		return "TerrainObject"
	case strings.HasPrefix(path, "Metadata/Terrain/"):
		return "Terrain"
	case path == "":
		return ""
	}
	return "Other"
}

const visualEntityIDThreshold = 0x40000000

func isInteractiveTerrainObject(path string) bool {
	for _, needle := range []string{"SpearWall", "Lever", "Gate", "Switch", "Crank", "Pull", "Door", "Activator", "Generator"} {
		if strings.Contains(path, needle) {
			return true
		}
	}
	return false
}

func WalkEntityMap(r Reader, sentinel uint64, onEntity func(entity uint64, idHi uint32)) {
	if sentinel == 0 {
		return
	}
	leftmost := ReadU64(r, sentinel)
	if leftmost == 0 || leftmost == sentinel {
		return
	}
	const maxWalk = 2000
	node := leftmost
	visited := make(map[uint64]bool)
	for i := 0; i < maxWalk && node != 0 && node != sentinel; i++ {
		if visited[node] {
			return
		}
		visited[node] = true
		nd, err := r.ReadBytes(node, 0x40)
		if err != nil || len(nd) < 0x30 {
			return
		}
		isNil := nd[0x19]
		idHi := binary.LittleEndian.Uint32(nd[0x20:0x24])
		entity := binary.LittleEndian.Uint64(nd[0x28:0x30])
		if isNil == 0 && entity >= HeapLo && entity < HeapHi && idHi < visualEntityIDThreshold {
			onEntity(entity, idHi)
		}
		node = NextInOrder(r, node, sentinel)
	}
}

func NextInOrder(r Reader, current, sentinel uint64) uint64 {
	cur, err := r.ReadBytes(current, 0x20)
	if err != nil || len(cur) < 0x18 {
		return 0
	}
	right := binary.LittleEndian.Uint64(cur[16:24])
	if right != sentinel {
		node := right
		for {
			n, err := r.ReadBytes(node, 0x10)
			if err != nil || len(n) < 8 {
				return node
			}
			left := binary.LittleEndian.Uint64(n[0:8])
			if left == sentinel {
				return node
			}
			node = left
		}
	}
	node := current
	parent := binary.LittleEndian.Uint64(cur[8:16])
	for parent != sentinel {
		p, err := r.ReadBytes(parent, 0x10)
		if err != nil || len(p) < 16 {
			return 0
		}
		pLeft := binary.LittleEndian.Uint64(p[0:8])
		if pLeft == node {
			return parent
		}
		node = parent
		parent = binary.LittleEndian.Uint64(p[8:16])
	}
	return 0
}

func ReadAwakeSize(r Reader, areaInstance uint64) uint32 {
	return ReadU32(r, areaInstance+AreaInstanceEntityListOff+EntityListAwakeSizeOff)
}

func AwakeSentinel(r Reader, areaInstance uint64) uint64 {
	return ReadU64(r, areaInstance+AreaInstanceEntityListOff+EntityListAwakeHeadOff)
}

func SleepingSentinel(r Reader, areaInstance uint64) uint64 {
	return ReadU64(r, areaInstance+AreaInstanceEntityListOff+EntityListSleepHeadOff)
}
