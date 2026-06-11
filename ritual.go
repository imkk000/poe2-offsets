package gamestate

import "strings"

// Ritual reward shelf. The ritual reward window reuses the same InventoryItemView
// mechanism as the backpack/stash/equipment: each reward slot is an item-view whose
// item entity is at view+0x4F8 and whose container backlink is at view+0x758. The
// ritual container is a grid (~12x10) holding the reward pool — identified as the
// container whose item-views include ritual-marker items. Verified live 2026-06-10
// (Rise of the Phoenix shield, Omen of Greater/Dextral Exaltation).

// ReadRitualRewards reads the ritual reward shelf's items, or nil if no ritual
// window is open. Reuses the item-view heap scan (regions = readable heap regions,
// same as FindBackpackContainer).
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

// RitualWindowOpen reports whether the ritual reward window is currently open. The
// reward item-views only exist in the heap while the window is displayed, so finding
// the ritual container IS the open/closed signal (no separate flag needed).
func RitualWindowOpen(r Reader, regions []HeapRegion) bool {
	_, ok := FindRitualContainer(r, regions)
	return ok
}

// FindRitualContainer returns the ritual reward container — the container backlinked
// by item-views that hold ritual-marker items (HiddenItem*Ritual / RitualAugment).
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
