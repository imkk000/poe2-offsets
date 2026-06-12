package gamestate

const (
	itemViewContainerBacklinkOff = 0x758
	itemViewItemIDOff            = 0x760
	itemViewDimOff               = 0x63F

	stcStashesBeginOff = 0x358
	stcStashesEndOff   = 0x360
	stcVisibleIdxOff   = 0x370
	stcEntryStride     = 0x90
	stcEntryNameOff    = 0x00
	stcEntryInvOff     = 0x80
	stcEntryButtonOff  = 0x88

	stashParentWalkDepth = 24
)

type StashItem struct {
	ID                                 uint32
	GridX, GridY, GW, GH               int
	Name, Path, Rarity                 string
	ModTexts                           []string
	ScreenX, ScreenY, ScreenW, ScreenH float32
	Dimmed                             bool
	Matched                            bool
}

type stashView struct {
	rect [4]float32
	dim  bool
}

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

func ReadStashTabName(r Reader, stcBegin uint64, idx int) string {
	return ReadStdWString(r, stcBegin+uint64(idx)*stcEntryStride+stcEntryNameOff)
}

type itemViewTab struct {
	ids        map[uint32]stashView
	views      []uint64
	onScr      int
	sample     uint64
	stc, begin uint64
	visibleIdx int
}

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

func realStashGrid(w, h int) bool { return (w == 12 && h == 12) || (w == 24 && h == 24) }

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

func RereadStash(r Reader, prev StashScan) (StashScan, bool) {
	if prev.STC == 0 || prev.Container == 0 {
		return StashScan{}, false
	}

	begin, vidx, ok := validateStashTabContainer(r, prev.STC)
	if !ok || begin != prev.STCBegin || vidx != prev.VisibleIdx {
		return StashScan{}, false
	}
	s := prev
	panel := ReadU64(r, begin+uint64(vidx)*stcEntryStride+stcEntryInvOff)
	if panel < HeapLo || panel >= HeapHi || !ElementVisibleHierarchical(r, panel) {
		return StashScan{}, false
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

func rereadViews(r Reader, container uint64, views []uint64) (map[uint32]stashView, bool) {
	if container == 0 {
		return nil, true
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
