package gamestate

const (
	positionedReactionOff = 0x1E0

	targetableIsTargetableOff = 0x69
	targetableIsHighlightOff  = 0x6A
	targetableIsTargetedOff   = 0x6B
	targetableHiddenOff       = 0x71

	transitionableStateOff = 0x120

	shrineIsUsedOff      = 0x24
	blockageIsBlockedOff = 0x30
	chestIsOpenedOff     = 0x168
	chestLabelVisibleOff = 0x21
)

func ReadReaction(r Reader, entity uint64) (byte, bool) {
	pos := ResolveComponentByName(r, entity, "Positioned")
	if pos == 0 {
		return 0, false
	}
	return ReadByte(r, pos+positionedReactionOff), true
}

func IsHostile(r Reader, entity uint64) bool {
	rc, ok := ReadReaction(r, entity)
	return ok && rc == 0
}

type TargetableState struct {
	IsTargetable    bool
	IsHighlightable bool
	IsTargeted      bool
	Hidden          bool
}

func ReadTargetable(r Reader, entity uint64) (TargetableState, bool) {
	t := ResolveComponentByName(r, entity, "Targetable")
	if t == 0 {
		return TargetableState{}, false
	}
	return TargetableState{
		IsTargetable:    ReadByte(r, t+targetableIsTargetableOff) != 0,
		IsHighlightable: ReadByte(r, t+targetableIsHighlightOff) != 0,
		IsTargeted:      ReadByte(r, t+targetableIsTargetedOff) != 0,
		Hidden:          ReadByte(r, t+targetableHiddenOff) != 0,
	}, true
}

func ReadTransitionableState(r Reader, entity uint64) (int16, bool) {
	tr := ResolveComponentByName(r, entity, "Transitionable")
	if tr == 0 {
		return 0, false
	}
	b, err := r.ReadBytes(tr+transitionableStateOff, 2)
	if err != nil || len(b) < 2 {
		return 0, false
	}
	return int16(b[0]) | int16(b[1])<<8, true
}

func ReadShrineUsed(r Reader, entity uint64) (bool, bool) {
	c := ResolveComponentByName(r, entity, "Shrine")
	if c == 0 {
		return false, false
	}
	return ReadByte(r, c+shrineIsUsedOff) != 0, true
}

func ReadBlockageBlocked(r Reader, entity uint64) (bool, bool) {
	c := ResolveComponentByName(r, entity, "TriggerableBlockage")
	if c == 0 {
		return false, false
	}
	return ReadByte(r, c+blockageIsBlockedOff) != 0, true
}

type ChestState struct {
	Opened       bool
	LabelVisible bool
}

func ReadChestState(r Reader, entity uint64) (ChestState, bool) {
	c := ResolveComponentByName(r, entity, "Chest")
	if c == 0 {
		return ChestState{}, false
	}
	return ChestState{
		Opened:       ReadByte(r, c+chestIsOpenedOff) != 0,
		LabelVisible: ReadByte(r, c+chestLabelVisibleOff) != 0,
	}, true
}

const (
	stateMachineValuesBeginOff = 0x160
	stateMachineValuesEndOff   = 0x168
	stateMachineValueStride    = 8
)

func ReadStateMachineState(r Reader, entity uint64, index int) (uint32, bool) {
	sm := ResolveComponentByName(r, entity, "StateMachine")
	if sm == 0 {
		return 0, false
	}
	begin := ReadU64(r, sm+stateMachineValuesBeginOff)
	end := ReadU64(r, sm+stateMachineValuesEndOff)
	if begin < HeapLo || end < begin {
		return 0, false
	}
	n := (end - begin) / stateMachineValueStride
	if index < 0 || uint64(index) >= n {
		return 0, false
	}
	return ReadU32(r, begin+uint64(index)*stateMachineValueStride), true
}

const (
	stateMachineTimersBeginOff = 0x178
	stateMachineTimersEndOff   = 0x180
)

func ReadStateMachineTimer(r Reader, entity uint64, index int) (float32, bool) {
	sm := ResolveComponentByName(r, entity, "StateMachine")
	if sm == 0 {
		return 0, false
	}
	begin := ReadU64(r, sm+stateMachineTimersBeginOff)
	end := ReadU64(r, sm+stateMachineTimersEndOff)
	if begin < HeapLo || end < begin {
		return 0, false
	}
	n := (end - begin) / stateMachineValueStride
	if index < 0 || uint64(index) >= n {
		return 0, false
	}
	return ReadFloat32(r, begin+uint64(index)*stateMachineValueStride), true
}
