package gamestate

const (
	itemViewContainerBacklinkOff = 0x758
	itemViewItemIDOff            = 0x760
	itemViewDimOff               = 0x63F // game stash-search sets this to 1 on non-matching items

	// StashTabContainer (RE 2026-06-07 via Ghidra OpenStashTab FUN_140ac6990 +
	// tab-select setter FUN_1426ef0c0). It is an Element ancestor of the open tab's
	// item-views, so we resolve it by walking the Element.Parent chain upward.
	stcStashesBeginOff = 0x358 // std::vector<StashTabContainerInventory> begin
	stcStashesEndOff   = 0x360 // end
	stcVisibleIdxOff   = 0x370 // int32: the open tab within this container
	stcEntryStride     = 0x90  // StashTabContainerInventory size
	stcEntryNameOff    = 0x00  // std::wstring tab name
	stcEntryInvOff     = 0x80  // content-panel Element (ancestor of that tab's item-views)
	stcEntryButtonOff  = 0x88  // tab-header button Element

	stashParentWalkDepth = 24 // item-view -> ... -> StashTabContainer (gem/flask nest deep)
)

type StashItem struct {
	ID                                 uint32
	GridX, GridY, GW, GH               int
	Name, Path, Rarity                 string
	ModTexts                           []string // resolved mod display strings (backpack only)
	ScreenX, ScreenY, ScreenW, ScreenH float32
	Dimmed                             bool // game search dimmed it (non-match)
	Matched                            bool // game search is active and this item matched
}

type stashView struct {
	rect [4]float32
	dim  bool
}

// validateStashTabContainer reports the Stashes-vector begin + VisibleStashIndex if
// stc has the StashTabContainer shape. Structural (no vtable) so it survives patches.
func validateStashTabContainer(r Reader, stc uint64) (begin uint64, visibleIdx int, ok bool) {
	if stc < HeapLo || stc >= HeapHi {
		return 0, 0, false
	}
	begin = ReadU64(r, stc+stcStashesBeginOff)
	end := ReadU64(r, stc+stcStashesEndOff)
	if begin < HeapLo || begin >= HeapHi || end < begin {
		return 0, 0, false
	}
	span := end - begin
	if span == 0 || span%stcEntryStride != 0 {
		return 0, 0, false
	}
	count := int(span / stcEntryStride)
	if count < 1 || count > 400 {
		return 0, 0, false
	}
	vi := int(int32(ReadU32(r, stc+stcVisibleIdxOff)))
	if vi < 0 || vi >= count {
		return 0, 0, false
	}
	return begin, vi, true
}

// walkUp invokes fn on each Element.Parent ancestor of start (up to the stash depth)
// until fn returns true or the chain leaves the heap.
func walkUp(r Reader, start uint64, fn func(anc uint64) bool) {
	anc := start
	for range stashParentWalkDepth {
		anc = ReadU64(r, anc+ElementParentOff)
		if anc < HeapLo || anc >= HeapHi {
			return
		}
		if fn(anc) {
			return
		}
	}
}

// ReadStashTabName reads the tab name (std::wstring) of Stashes[idx] of stc.
func ReadStashTabName(r Reader, stcBegin uint64, idx int) string {
	return ReadStdWString(r, stcBegin+uint64(idx)*stcEntryStride+stcEntryNameOff)
}

// FindStashContainer returns the rendered (open) stash tab's item container plus
// each item id's on-screen rect and the game's search dim flag.
//
// Deterministic: every InventoryItemView is grouped by its backing container; the
// item-view's Element.Parent chain is walked up to the nearest StashTabContainer
// (Stashes vec @ +0x358, VisibleStashIndex @ +0x370). The OPEN container is the one
// whose views descend from Stashes[VisibleStashIndex] (its content panel @ +0x80) of
// that container — which works for every tab type (normal, quad, currency, gem,
// flask, waystone sub-grids) with no grid-size assumption. Falls back to the old
// most-on-screen heuristic if the chain can't be resolved.
type itemViewTab struct {
	ids        map[uint32]stashView
	views      []uint64 // item-view addresses (cached for cheap re-read, no full rescan)
	onScr      int
	sample     uint64 // a representative item-view
	stc, begin uint64
	visibleIdx int
}

// scanItemViews does ONE InventoryItemView pass over the heap, grouping views by
// their backing container (id -> rect+dim, on-screen count, resolved StashTabContainer).
// Both the stash picker and the inventory-rect collector consume this, so the radar
// can poll a single heap scan per cycle.
func scanItemViews(r Reader, regions []HeapRegion) map[uint64]*itemViewTab {
	tabs := make(map[uint64]*itemViewTab)
	for _, v := range scanQwordHits(r, regions, InventoryItemViewVtable) {
		if ReadU64(r, v+ElementSelfOff) != v {
			continue
		}
		c := ReadU64(r, v+itemViewContainerBacklinkOff)
		if c < HeapLo || c >= HeapHi {
			continue
		}
		t := tabs[c]
		if t == nil {
			t = &itemViewTab{ids: make(map[uint32]stashView), sample: v}
			if stc, begin, vi, found := resolveStc(r, v); found {
				t.stc, t.begin, t.visibleIdx = stc, begin, vi
			}
			tabs[c] = t
		}
		sv := stashView{dim: ReadByte(r, v+itemViewDimOff) != 0}
		if x, y, posOK := ElementAbsPos(r, v); posOK {
			ew, eh := ElementSize(r, v)
			sv.rect = [4]float32{x, y, ew, eh}
			if x > 1 && x < 2560 && y > 1 && y < 1600 {
				t.onScr++
			}
		}
		t.ids[ReadU32(r, v+itemViewItemIDOff)] = sv
		t.views = append(t.views, v)
	}
	return tabs
}

// pickOpenStash selects the rendered (open) tab from a scanned tab set: the container
// whose views reach its own Stashes[VisibleStashIndex] content panel (tie-break most
// on-screen, for sub-tabs); falls back to the real-grid most-on-screen heuristic.
func pickOpenStash(r Reader, tabs map[uint64]*itemViewTab) (container uint64, byID map[uint32]stashView, panel uint64, ok bool) {
	bestScr := -1
	for c, t := range tabs {
		if t.stc == 0 {
			continue
		}
		openPanel := ReadU64(r, t.begin+uint64(t.visibleIdx)*stcEntryStride+stcEntryInvOff)
		if openPanel < HeapLo || openPanel >= HeapHi {
			continue
		}
		// The stash UI keeps its item-view + panel elements allocated when closed
		// (just hidden), so existence/on-screen-rect alone reports a false "open".
		// Require the panel to be hierarchically visible.
		if !ElementVisibleHierarchical(r, openPanel) {
			continue
		}
		reaches := false
		walkUp(r, t.sample, func(anc uint64) bool {
			if anc == openPanel {
				reaches = true
				return true
			}
			return false
		})
		if reaches && t.onScr > bestScr {
			container, byID, panel, bestScr = c, t.ids, openPanel, t.onScr
		}
	}
	if container != 0 {
		return container, byID, panel, true
	}

	// fallback: pre-RE heuristic — real grid + most on-screen views (no panel rect).
	bestScr = -1
	for c, t := range tabs {
		if !realStashGrid(int(ReadU32(r, c+backpackGridWidthOff)), int(ReadU32(r, c+backpackGridHeightOff))) {
			continue
		}
		if t.onScr > bestScr {
			container, byID, bestScr = c, t.ids, t.onScr
		}
	}
	if container == 0 {
		return 0, nil, 0, false
	}
	return container, byID, 0, true
}

func FindStashContainer(r Reader, regions []HeapRegion, preferred uint64) (uint64, map[uint32]stashView, uint64, bool) {
	return pickOpenStash(r, scanItemViews(r, regions))
}

// resolveStc walks an item-view's parent chain to the nearest StashTabContainer.
func resolveStc(r Reader, view uint64) (stc, begin uint64, visibleIdx int, ok bool) {
	walkUp(r, view, func(anc uint64) bool {
		if b, vi, good := validateStashTabContainer(r, anc); good {
			stc, begin, visibleIdx, ok = anc, b, vi, true
			return true
		}
		return false
	})
	return stc, begin, visibleIdx, ok
}

// realStashGrid reports a normal (12x12) or quad (24x24) grid — used only by the
// fallback path; the deterministic chain needs no grid-size assumption.
func realStashGrid(w, h int) bool { return (w == 12 && h == 12) || (w == 24 && h == 24) }

// buildStashItems turns a picked open tab (container + per-id rects/dim + content
// panel) into StashItems, enriching with grid/name details from ReadBackpack when the
// container is a readable grid (special tabs whose grid>32 fail it still yield items
// with rect+dim). Returns the grid dims and the fixed content-panel rect (design space).
func buildStashItems(r Reader, container uint64, byID map[uint32]stashView) ([]StashItem, int, int) {
	searchActive := false
	for _, sv := range byID {
		if sv.dim {
			searchActive = true
			break
		}
	}

	bp, bpOK := ReadBackpack(r, container)
	details := make(map[uint32]BackpackItem)
	if bpOK {
		for _, it := range bp.Items {
			details[it.ID] = it
		}
	}

	out := make([]StashItem, 0, len(byID))
	for id, sv := range byID {
		si := StashItem{ID: id}
		si.ScreenX, si.ScreenY, si.ScreenW, si.ScreenH = sv.rect[0], sv.rect[1], sv.rect[2], sv.rect[3]
		si.Dimmed = sv.dim
		si.Matched = searchActive && !sv.dim
		if it, has := details[id]; has {
			si.GridX, si.GridY, si.GW, si.GH = it.GridX, it.GridY, it.Width, it.Height
			si.Name, si.Path, si.Rarity = it.Name, it.Path, it.Rarity
		}
		out = append(out, si)
	}
	return out, bp.Width, bp.Height
}

func ReadVisibleStash(r Reader, regions []HeapRegion, preferred uint64) ([]StashItem, uint64, int, int, [4]float32, bool) {
	s := ScanStash(r, regions, 0)
	if !s.Open {
		return nil, 0, 0, 0, [4]float32{}, false
	}
	return s.Items, s.Container, s.GridW, s.GridH, s.PanelRect, true
}

// StashScan is a cacheable read of the open stash tab + the backpack item rects. The
// STC/STCBegin/VisibleIdx/OpenViews/InvViews fields let the radar re-read everything
// cheaply (RereadStash) instead of re-scanning the whole 8GB heap each poll.
type StashScan struct {
	Open         bool
	Container    uint64
	GridW, GridH int
	PanelRect    [4]float32
	Items        []StashItem

	STC, STCBegin uint64
	VisibleIdx    int
	OpenViews     []uint64

	InvContainer uint64
	InvViews     []uint64
	InvRects     map[uint32][4]float32
}

// ScanStash does the full (expensive) InventoryItemView heap pass and returns a
// cacheable snapshot of the open stash tab + the backpack rects.
func ScanStash(r Reader, regions []HeapRegion, backpackContainer uint64) StashScan {
	tabs := scanItemViews(r, regions)

	var s StashScan
	s.InvContainer = backpackContainer
	s.InvRects = make(map[uint32][4]float32)
	if backpackContainer != 0 {
		if t := tabs[backpackContainer]; t != nil {
			s.InvViews = t.views
			for id, sv := range t.ids {
				if sv.rect[2] > 0 || sv.rect[3] > 0 {
					s.InvRects[id] = sv.rect
				}
			}
		}
	}

	container, byID, panel, ok := pickOpenStash(r, tabs)
	if !ok {
		return s
	}
	s.Open = true
	s.Container = container
	s.PanelRect = elementRect(r, panel)
	s.Items, s.GridW, s.GridH = buildStashItems(r, container, byID)
	if t := tabs[container]; t != nil {
		s.STC, s.STCBegin, s.VisibleIdx, s.OpenViews = t.stc, t.begin, t.visibleIdx, t.views
	}
	return s
}

// RereadStash cheaply refreshes a prior ScanStash WITHOUT a heap scan: it reads the
// cached StashTabContainer's VisibleStashIndex + content-panel rect and re-reads the
// cached item-view addresses directly. Returns ok=false when the cache is stale (stash
// closed, or the open tab changed) so the caller falls back to a full ScanStash.
func RereadStash(r Reader, prev StashScan) (StashScan, bool) {
	if prev.STC == 0 || prev.Container == 0 {
		return StashScan{}, false
	}
	// VisibleStashIndex + Stashes vector still valid? (stash closed -> garbage)
	begin, vidx, ok := validateStashTabContainer(r, prev.STC)
	if !ok || begin != prev.STCBegin || vidx != prev.VisibleIdx {
		return StashScan{}, false // closed or tab switched -> full rescan
	}
	s := prev
	panel := ReadU64(r, begin+uint64(vidx)*stcEntryStride+stcEntryInvOff)
	if panel < HeapLo || panel >= HeapHi || !ElementVisibleHierarchical(r, panel) {
		return StashScan{}, false // UI closed: elements persist but are hidden
	}
	s.PanelRect = elementRect(r, panel)

	byID, ok := rereadViews(r, prev.Container, prev.OpenViews)
	if !ok {
		return StashScan{}, false
	}
	s.Items, s.GridW, s.GridH = buildStashItems(r, prev.Container, byID)

	s.InvRects = make(map[uint32][4]float32)
	if inv, ok := rereadViews(r, prev.InvContainer, prev.InvViews); ok {
		for id, sv := range inv {
			if sv.rect[2] > 0 || sv.rect[3] > 0 {
				s.InvRects[id] = sv.rect
			}
		}
	}
	return s, true
}

// rereadViews re-reads cached item-view addresses (rect + dim) for a known container,
// skipping any that are no longer valid views of it. ok=false if none remain valid
// (container gone / addresses reused) so the caller forces a full rescan.
func rereadViews(r Reader, container uint64, views []uint64) (map[uint32]stashView, bool) {
	if container == 0 {
		return nil, true // no inventory cached; not an error
	}
	out := make(map[uint32]stashView, len(views))
	valid := 0
	for _, v := range views {
		if ReadU64(r, v+ElementSelfOff) != v || ReadU64(r, v+itemViewContainerBacklinkOff) != container {
			continue
		}
		valid++
		sv := stashView{dim: ReadByte(r, v+itemViewDimOff) != 0}
		if x, y, posOK := ElementAbsPos(r, v); posOK {
			ew, eh := ElementSize(r, v)
			sv.rect = [4]float32{x, y, ew, eh}
		}
		out[ReadU32(r, v+itemViewItemIDOff)] = sv
	}
	if len(views) > 0 && valid == 0 {
		return nil, false
	}
	return out, true
}

func elementRect(r Reader, e uint64) [4]float32 {
	if e == 0 {
		return [4]float32{}
	}
	if x, y, ok := ElementAbsPos(r, e); ok {
		w, h := ElementSize(r, e)
		return [4]float32{x, y, w, h}
	}
	return [4]float32{}
}
