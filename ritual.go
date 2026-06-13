package gamestate

import (
	"encoding/binary"
	"math"
	"strings"
)

const ritualWindowVtable uint64 = 0x142f37a08

const (
	ritualWinDataSourceOff = 0x3D0
	ritualRtConfigOff      = 0x1A0
	ritualRtBoardOff       = 0x1A8
	ritualQStatStoreOff    = 0x130
	ritualTributeStateOff  = 0x2880
	ritualTributePoolOff   = 0x18
	ritualRewardKeyOff     = 0x760
	ritualEntryStride      = 12
	ritualHiddenCost       = 0xFFFF
	ritualStatOfferedCost  = 21576
	ritualStatDeferCost    = 16575
)

type RitualReward struct {
	Item         InventoryItem `json:"item"`
	RewardKey    uint32        `json:"rewardKey"`
	Cost         int           `json:"cost,omitempty"`
	OriginalCost int           `json:"originalCost,omitempty"`
	Deferred     bool          `json:"deferred,omitempty"`
	Hidden       bool          `json:"hidden,omitempty"`
}

type ritualCostEntry struct {
	remaining, original uint16
	deferred            bool
}

func ritualStatValue(r Reader, store uint64, key int32) int32 {
	begin := ReadU64(r, store+8)
	end := ReadU64(r, store+0x10)
	if begin < HeapLo || end <= begin || end-begin > 0x8000 {
		return 0
	}
	buf, err := r.ReadBytes(begin, int(end-begin))
	if err != nil {
		return 0
	}
	for i := 0; i+8 <= len(buf); i += 8 {
		if int32(binary.LittleEndian.Uint32(buf[i:])) == key {
			return int32(binary.LittleEndian.Uint32(buf[i+4:]))
		}
	}
	return 0
}

type ritualTribute struct {
	costs map[uint32]ritualCostEntry
	mult  float64
	pool  int
}

// ritualTributeVec resolves a ritual window to its tribute-state object and the raw cost
// vector bytes. ok is false for stale/empty windows.
func ritualTributeVec(r Reader, win uint64) (rt, trib uint64, buf []byte, ok bool) {
	rt = ReadU64(r, win+ritualWinDataSourceOff)
	if rt < HeapLo || rt >= HeapHi {
		return 0, 0, nil, false
	}
	trib = ReadU64(r, ReadU64(r, rt+ritualRtBoardOff)+ritualTributeStateOff)
	if trib < HeapLo || trib >= HeapHi {
		return 0, 0, nil, false
	}
	begin := ReadU64(r, trib)
	end := ReadU64(r, trib+8)
	if begin < HeapLo || end <= begin || end-begin > 0x4000 {
		return 0, 0, nil, false
	}
	buf, err := r.ReadBytes(begin, int(end-begin))
	if err != nil {
		return 0, 0, nil, false
	}
	return rt, trib, buf, true
}

func parseRitualCostVec(buf []byte) map[uint32]ritualCostEntry {
	costs := make(map[uint32]ritualCostEntry)
	for i := 0; i+ritualEntryStride <= len(buf); i += ritualEntryStride {
		rem := binary.LittleEndian.Uint16(buf[i+4:])
		orig := binary.LittleEndian.Uint16(buf[i+6:])
		costs[binary.LittleEndian.Uint32(buf[i:])] = ritualCostEntry{
			remaining: rem, original: orig,
			deferred: buf[i+8] == 2 || (rem != ritualHiddenCost && rem < orig),
		}
	}
	return costs
}

func readRitualTributeTable(r Reader, regions []HeapRegion) (ritualTribute, bool) {
	for _, win := range scanQwordHits(r, regions, ritualWindowVtable) {
		rt, trib, buf, ok := ritualTributeVec(r, win)
		if !ok {
			continue
		}
		cfg := ReadU64(r, rt+ritualRtConfigOff)
		s1 := ritualStatValue(r, cfg+ritualQStatStoreOff, ritualStatOfferedCost)
		s2 := ritualStatValue(r, cfg+ritualQStatStoreOff, ritualStatDeferCost)
		return ritualTribute{
			costs: parseRitualCostVec(buf),
			mult:  1 + float64(s1+s2)/100,
			pool:  int(ReadU32(r, trib+ritualTributePoolOff)),
		}, true
	}
	return ritualTribute{mult: 1}, false
}

func (t ritualTribute) apply(rr *RitualReward) {
	e, found := t.costs[rr.RewardKey]
	if !found {
		return
	}
	if e.remaining == ritualHiddenCost || e.original == ritualHiddenCost {
		rr.Hidden = true
		return
	}
	rr.Cost = int(math.Round(float64(e.remaining) * t.mult))
	rr.OriginalCost = int(math.Round(float64(e.original) * t.mult))
	rr.Deferred = e.deferred
}

// Ritual rewards include currency/runes/gems with no Mods component, so don't require
// Mods: parse it when present (rares/uniques), else fall back to the metadata path.
func ritualRewardItem(r Reader, item uint64) InventoryItem {
	var it InventoryItem
	if mods := ResolveComponentByName(r, item, "Mods"); mods != 0 {
		if parsed, ok := ReadItemFromRarityComp(r, mods); ok {
			it = parsed
		}
	}
	if it.Path == "" && it.FullName == "" && it.BaseName == "" {
		it.Path = ReadEntityMetadata(r, item)
	}
	return it
}

// ReadRitualRewardsWithCost returns each ritual reward with its per-slot tribute cost.
// Cost is the displayed (remaining) tribute; OriginalCost the pre-deferral base; Hidden=unrevealed.
func ReadRitualRewardsWithCost(r Reader, regions []HeapRegion) []RitualReward {
	rewards, _, _ := ReadRitualState(r, regions)
	return rewards
}

// ReadRitualState returns the open ritual reward window's rewards (with per-slot tribute cost)
// and the available tribute pool. open is false when no ritual reward window is open.
func ReadRitualState(r Reader, regions []HeapRegion) (rewards []RitualReward, pool int, open bool) {
	cont, ok := FindRitualContainer(r, regions)
	if !ok {
		return nil, 0, false
	}
	trib, _ := readRitualTributeTable(r, regions)
	seen := make(map[uint64]bool)
	var out []RitualReward
	for _, v := range scanQwordHits(r, regions, InventoryItemViewVtable) {
		if ReadU64(r, v+inventoryItemViewContainerOff) != cont {
			continue
		}
		item := ReadU64(r, v+hoverViewItemEntityOff)
		if item < HeapLo || item >= HeapHi || seen[item] {
			continue
		}
		seen[item] = true
		rr := RitualReward{Item: ritualRewardItem(r, item), RewardKey: ReadU32(r, v+ritualRewardKeyOff)}
		trib.apply(&rr)
		out = append(out, rr)
	}
	return out, trib.pool, true
}

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
