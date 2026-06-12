package gamestate

import "strings"

func ReadRitualRewards(r Reader, regions []HeapRegion) []InventoryItem {
	cont, ok := FindRitualContainer(r, regions)
	if !ok {
		return nil
	}
	views := scanQwordHits(r, regions, InventoryItemViewVtable)
	seen := make(map[uint64]bool)
	var out []InventoryItem
	for _, v := range views {
		if ReadU64(r, v+inventoryItemViewContainerOff) != cont {
			continue
		}
		item := ReadU64(r, v+hoverViewItemEntityOff)
		if item < HeapLo || item >= HeapHi || seen[item] {
			continue
		}
		seen[item] = true
		mods := ResolveComponentByName(r, item, "Mods")
		if mods == 0 {
			continue
		}
		if it, ok := ReadItemFromRarityComp(r, mods); ok {
			out = append(out, it)
		}
	}
	return out
}

func RitualWindowOpen(r Reader, regions []HeapRegion) bool {
	_, ok := FindRitualContainer(r, regions)
	return ok
}

func FindRitualContainer(r Reader, regions []HeapRegion) (uint64, bool) {
	for _, v := range scanQwordHits(r, regions, InventoryItemViewVtable) {
		item := ReadU64(r, v+hoverViewItemEntityOff)
		if item < HeapLo || item >= HeapHi {
			continue
		}
		if strings.Contains(ReadEntityMetadata(r, item), "Ritual") {
			if c := ReadU64(r, v+inventoryItemViewContainerOff); c >= HeapLo && c < HeapHi {
				return c, true
			}
		}
	}
	return 0, false
}

const renderRitualSacrificePctOff = 0x4BC

func ReadRitualSacrificePercent(r Reader, entity uint64) (float32, bool) {
	rc := ResolveComponentByName(r, entity, "Render")
	if rc == 0 {
		return 0, false
	}
	return ReadFloat32(r, rc+renderRitualSacrificePctOff), true
}
