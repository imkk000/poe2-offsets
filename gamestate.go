package gamestate

import "errors"

const (
	GameStateInGameStateOff    = 0x88
	InGameStateAreaInstanceOff = 0x290
	InGameStateCameraOff       = 0x368
	InGameStateUiRootOff       = 0x2F0
	CameraZoomOff              = 0x528

	ElementParentOff = 0xB8

	AreaInstanceEntityListOff = 0x6C8
	EntityListAwakeHeadOff    = 0x10
	EntityListAwakeSizeOff    = 0x18
	EntityListSleepHeadOff    = 0x20
	EntityListSleepSizeOff    = 0x28

	AreaInstancePlayerInfoOff  = 0x598
	AreaInstanceLocalPlayerOff = 0x5B8
)

const uiRootMaxParentHops = 64

func ResolveGSO(r Reader, gsoSlot uint64) (uint64, error) {
	gso := ReadU64(r, gsoSlot)
	if gso == 0 {
		return 0, errors.New("gso null (game not in-game yet?)")
	}
	return gso, nil
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
	root := ReadU64(r, igs+InGameStateUiRootOff)
	if root == 0 {
		return 0, errors.New("UiRoot null at IGS+0x2F0")
	}
	return root, nil
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
