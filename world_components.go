package gamestate

// World-entity component readers. Offsets verified live 2026-06-10. All resolve their
// component by name, so they survive vtable drift.

const (
	// Positioned.Reaction byte: 0 = hostile/enemy, 1 = ally/friendly (player, player
	// summons, allied NPCs). Verified live 2026-06-10 on a minion build — summoned
	// skeletons read 1, hostile monsters read 0.
	positionedReactionOff = 0x1E0

	// Targetable flag block (same +0x18 drift as Life).
	targetableIsTargetableOff = 0x69
	targetableIsHighlightOff  = 0x6A
	targetableIsTargetedOff   = 0x6B
	targetableHiddenOff       = 0x71

	transitionableStateOff = 0x120 // Transitionable: i16 CurrentStateEnum (benches read 2)
)

// ReadReaction reads the Positioned.Reaction byte: 0 = hostile/enemy, 1 = ally
// (player, summons, allied NPCs). Verified live on a minion build.
func ReadReaction(r Reader, entity uint64) (byte, bool) {
	pos := ResolveComponentByName(r, entity, "Positioned")
	if pos == 0 {
		return 0, false
	}
	return ReadByte(r, pos+positionedReactionOff), true
}

// IsHostile reports whether an entity's Positioned.Reaction marks it an enemy
// (reaction 0). Entities without a Positioned component default to false.
func IsHostile(r Reader, entity uint64) bool {
	rc, ok := ReadReaction(r, entity)
	return ok && rc == 0
}

type TargetableState struct {
	IsTargetable    bool
	IsHighlightable bool
	IsTargeted      bool // currently targeted by the player
	Hidden          bool // hidden from player
}

// ReadTargetable reads the Targetable component flags. Verified live 2026-06-10:
// clickable town objects (stash, benches) read IsTargetable=1, Hidden=0.
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

// ReadTransitionableState reads Transitionable.CurrentStateEnum (door/lever/bench
// state). Offset confirmed live (crafting benches read 2); per-object value
// semantics (open vs closed) need an interactable door to enumerate.
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

const (
	stateMachineValuesBeginOff = 0x160
	stateMachineValuesEndOff   = 0x168
	stateMachineValueStride    = 8
)

// ReadStateMachineState reads a state value from the entity's StateMachine component by
// slot index, the same store GGG's GetState getter reads (FUN_141f5ed10): values vector
// [+0x160,+0x168) stride 8 with a u32 state per slot, selected by GetState via a
// name->index map @+0x158. Encounter/altar/interactable state lives here (e.g. the ritual
// altar state at index 0: 0=available, 2=active, 3=done). ok is false outside the vector.
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
