package gamestate

import (
	"encoding/binary"
	"strings"
)

// Flask belt resolution for charge-aware auto-flask. The belt is a small inventory
// container (<=5 wide, <=2 tall) holding only flask/charm items; each flask sits at
// grid x = key-slot - 1 (x=0 -> key '1' = Life flask, x=1 -> key '2' = Mana flask).
// Verified live 2026-06-10 (a live probe).

const (
	flaskBeltMaxWidth  = 5
	flaskBeltMaxHeight = 2
	containerWidthOff  = 0x150
	containerHeightOff = 0x154
)

// ResolveFlaskBelt BFS-walks the player Inventories component for the flask belt
// container. Returns 0 if not found. The container is stable within a session —
// cache it and only re-resolve when a charge read fails (zone reload).
func ResolveFlaskBelt(r Reader, gsoSlot uint64) uint64 {
	lp, err := ResolveLocalPlayer(r, gsoSlot)
	if err != nil || lp == 0 {
		return 0
	}
	inv := ResolveComponentByName(r, lp, "Inventories")
	if inv == 0 {
		return 0
	}
	begin := ReadU64(r, inv+0x20)
	end := ReadU64(r, inv+0x28)
	if begin < HeapLo || begin >= HeapHi || end <= begin {
		return 0
	}
	var queue []uint64
	for i := 0; i < int((end-begin)/8); i++ {
		if p := ReadU64(r, begin+uint64(i)*8); p >= HeapLo && p < HeapHi {
			queue = append(queue, p)
		}
	}
	visited := make(map[uint64]bool)
	depth := make(map[uint64]int)
	for _, p := range queue {
		depth[p] = 0
	}
	for len(queue) > 0 && len(visited) < 60000 {
		a := queue[0]
		queue = queue[1:]
		if visited[a] || depth[a] > 7 {
			continue
		}
		visited[a] = true
		buf, err := r.ReadBytes(a, 0x200)
		if err != nil || len(buf) < 0x200 {
			continue
		}
		w := binary.LittleEndian.Uint32(buf[containerWidthOff:])
		h := binary.LittleEndian.Uint32(buf[containerHeightOff:])
		sentinel := binary.LittleEndian.Uint64(buf[backpackMapSentinelOff:])
		if w >= 1 && w <= flaskBeltMaxWidth && h >= 1 && h <= flaskBeltMaxHeight &&
			sentinel >= HeapLo && sentinel < HeapHi && ReadByte(r, sentinel+0x19) == 1 {
			if isFlaskBelt(r, sentinel) {
				return a
			}
		}
		if depth[a] < 7 {
			for o := 0; o+8 <= len(buf); o += 8 {
				if p := binary.LittleEndian.Uint64(buf[o:]); p >= HeapLo && p < HeapHi && !visited[p] {
					queue = append(queue, p)
					if _, ok := depth[p]; !ok {
						depth[p] = depth[a] + 1
					}
				}
			}
		}
	}
	return 0
}

// isFlaskBelt reports whether a container holds at least one flask and only
// flask/charm items (rules out the backpack, which also carries flasks).
func isFlaskBelt(r Reader, sentinel uint64) bool {
	hasFlask, foreign := false, false
	WalkInventoryMap(r, sentinel, func(node uint64) {
		vp := ReadU64(r, node+0x28)
		if vp < HeapLo || vp >= HeapHi {
			return
		}
		ent := ReadU64(r, vp)
		if ent < HeapLo || ent >= HeapHi {
			return
		}
		p := flaskItemPath(r, ent)
		switch {
		case strings.Contains(p, "Flask"):
			hasFlask = true
		case strings.Contains(p, "Charm"):
		default:
			foreign = true
		}
	})
	return hasFlask && !foreign
}

// FlaskChargesInSlot reads the Charges of the flask at belt grid x = slot-1.
func FlaskChargesInSlot(r Reader, beltContainer uint64, slot int) (FlaskCharges, bool) {
	if beltContainer < HeapLo || beltContainer >= HeapHi || slot < 1 {
		return FlaskCharges{}, false
	}
	sentinel := ReadU64(r, beltContainer+backpackMapSentinelOff)
	if sentinel < HeapLo || sentinel >= HeapHi {
		return FlaskCharges{}, false
	}
	wantX := int32(slot - 1)
	var found uint64
	WalkInventoryMap(r, sentinel, func(node uint64) {
		if found != 0 {
			return
		}
		vp := ReadU64(r, node+0x28)
		if vp < HeapLo || vp >= HeapHi {
			return
		}
		if int32(readU32(r, vp+8)) != wantX {
			return
		}
		ent := ReadU64(r, vp)
		if ent >= HeapLo && ent < HeapHi && strings.Contains(flaskItemPath(r, ent), "Flask") {
			found = ent
		}
	})
	if found == 0 {
		return FlaskCharges{}, false
	}
	return ReadItemCharges(r, found)
}

func flaskItemPath(r Reader, ent uint64) string {
	details := ReadU64(r, ent+0x8)
	if details < HeapLo || details >= HeapHi {
		return ""
	}
	return readPathString(r, ReadU64(r, details+0x8))
}
