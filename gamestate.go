package gamestate

import "errors"

const (
	GameStateInGameStateOff    = 0x90
	InGameStateAreaInstanceOff = 0x290
	InGameStateCameraOff       = 0x368
	InGameStateUiRootOff       = 0x2F0
	CameraZoomOff              = 0x528

	ElementParentOff = 0xB8

	AreaInstanceEntityListOff = 0x6E0
	EntityListAwakeHeadOff    = 0x10
	EntityListAwakeSizeOff    = 0x18
	EntityListSleepHeadOff    = 0x20
	EntityListSleepSizeOff    = 0x28

	AreaInstancePlayerInfoOff  = 0x5B0
	AreaInstanceLocalPlayerOff = 0x5D0
)

const uiRootMaxParentHops = 64

func ResolveGSO(r Reader, gsoSlot uint64) (uint64, error) {
	gso := ReadU64(r, gsoSlot)
	if gso == 0 {
		return 0, errors.New("gso null (game not in-game yet?)")
	}
	return gso, nil
}

// GsoSlotResolves reports whether a candidate slot dereferences into a live
// InGameState. Only meaningful once the game is past the login screen, so a
// false result is inconclusive rather than a rejection.
func GsoSlotResolves(r Reader, gsoSlot uint64) bool {
	gso := ReadU64(r, gsoSlot)
	if gso < HeapLo || gso >= HeapHi {
		return false
	}
	igs := ReadU64(r, gso+GameStateInGameStateOff)
	return igs >= HeapLo && igs < HeapHi
}

func ResolveInGameState(r Reader, gsoSlot uint64) (uint64, error) {
	gso, err := ResolveGSO(r, gsoSlot)
	if err != nil {
		return 0, err
	}
	igs := ReadU64(r, gso+GameStateInGameStateOff)
	if igs == 0 {
		return 0, errors.New("InGameState null")
	}
	return igs, nil
}

func ResolveAreaInstance(r Reader, gsoSlot uint64) (uint64, error) {
	igs, err := ResolveInGameState(r, gsoSlot)
	if err != nil {
		return 0, err
	}
	area := ReadU64(r, igs+InGameStateAreaInstanceOff)
	if area == 0 {
		return 0, errors.New("AreaInstance null")
	}
	return area, nil
}

func ResolveCamera(r Reader, gsoSlot uint64) (uint64, error) {
	igs, err := ResolveInGameState(r, gsoSlot)
	if err != nil {
		return 0, err
	}
	cam := ReadU64(r, igs+InGameStateCameraOff)
	if cam == 0 {
		return 0, errors.New("camera null")
	}
	return cam, nil
}

func ResolveLocalPlayer(r Reader, gsoSlot uint64) (uint64, error) {
	area, err := ResolveAreaInstance(r, gsoSlot)
	if err != nil {
		return 0, err
	}
	p := ReadU64(r, area+AreaInstanceLocalPlayerOff)
	if p == 0 {
		return 0, errors.New("LocalPlayer null")
	}
	return p, nil
}

func ResolveUiRoot(r Reader, gsoSlot uint64) (uint64, error) {
	igs, err := ResolveInGameState(r, gsoSlot)
	if err != nil {
		return 0, err
	}
	if root := ReadU64(r, igs+InGameStateUiRootOff); root >= HeapLo && root < HeapHi {
		return root, nil
	}
	// The slot holding the root is not stable across restarts (it has been seen
	// at several offsets within InGameState), so fall back to identifying the
	// root by its design canvas, which is a fixed 2560x1600 for this client.
	if root := scanUiRoot(r, igs); root != 0 {
		return root, nil
	}
	return 0, errors.New("UiRoot not found in InGameState")
}

const (
	uiRootScanSpan   = 0x800
	uiRootCanvasOff  = 0x270
	uiRootDesignW    = 2560
	uiRootDesignH    = 1600
	uiRootCandidates = 0x400
)

// scanUiRoot finds the root UI element by its design-canvas dimensions.
func scanUiRoot(r Reader, igs uint64) uint64 {
	buf, err := r.ReadBytes(igs, uiRootScanSpan)
	if err != nil {
		return 0
	}
	for off := 0; off+8 <= len(buf); off += 8 {
		cand := ReadU64(r, igs+uint64(off))
		if cand < HeapLo || cand >= HeapHi {
			continue
		}
		w := ReadFloat32(r, cand+uiRootCanvasOff)
		h := ReadFloat32(r, cand+uiRootCanvasOff+4)
		if w == uiRootDesignW && h == uiRootDesignH {
			return cand
		}
	}
	return 0
}

func ResolveTrueUiRoot(r Reader, gsoSlot uint64) (uint64, error) {
	cur, err := ResolveUiRoot(r, gsoSlot)
	if err != nil {
		return 0, err
	}
	for range uiRootMaxParentHops {
		parent := ReadU64(r, cur+ElementParentOff)
		if parent < HeapLo || parent >= HeapHi {
			return cur, nil
		}
		cur = parent
	}
	return 0, errors.New("UiRoot Parent walk exceeded hop limit (struct drift?)")
}
