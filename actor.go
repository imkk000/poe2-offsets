package gamestate

// Actor combat state. Offsets found 2026-06-10 by observing the local player's Actor
// during live manual combat: Actor+0x900 held a pointer that tracked the current target
// across every monster the player switched to. Resolve the component by name so it
// survives vtable drift.

const actorCurrentTargetOff = 0x900

// ReadActorTarget returns the entity the actor is currently targeting (the monster the
// player is attacking), or 0 if none. Verified live: tracked retargeting across
// TerracottaGuardian / FaridunLizard / DruidicFallenStag / DesertPhantasm / HooksMonster.
func ReadActorTarget(r Reader, entity uint64) uint64 {
	actor := ResolveComponentByName(r, entity, "Actor")
	if actor == 0 {
		return 0
	}
	tgt := ReadU64(r, actor+actorCurrentTargetOff)
	if tgt < HeapLo || tgt >= HeapHi || ReadEntityMetadata(r, tgt) == "" {
		return 0
	}
	return tgt
}

// Movement state. Offsets from GGG's GameWorld GetMovementDestination getter
// (FUN_141f78900): the entity's Positioned/Movement sub-object at entity+0x98 holds
// move speed, a moving flag, the destination grid coords, and the current world position.
// curX/curY at +0x490/+0x494 cross-checked live against the Render position (exact match).
const (
	entityPositionedOff = 0x98
	moveSpeedOff        = 0x28C
	moveFlagOff         = 0x1E5
	moveDestXOff        = 0x240
	moveDestYOff        = 0x244
)

func resolvePositioned(r Reader, entity uint64) uint64 {
	sub := ReadU64(r, entity+entityPositionedOff)
	if sub < HeapLo || sub >= HeapHi {
		return 0
	}
	return sub
}

func actorMoving(r Reader, sub uint64) bool {
	return ReadFloat32(r, sub+moveSpeedOff) != 0 && ReadU32(r, sub+moveFlagOff)&0x80 == 0
}

// ReadActorMoving reports whether the entity is currently moving, matching the engine's
// own gate (move speed nonzero and the move flag byte non-negative).
func ReadActorMoving(r Reader, entity uint64) bool {
	sub := resolvePositioned(r, entity)
	return sub != 0 && actorMoving(r, sub)
}

// ReadMovementDestination returns the entity's destination grid coordinates while moving;
// ok is false when stationary (the engine leaves a sentinel in the destination otherwise).
func ReadMovementDestination(r Reader, entity uint64) (x, y int32, ok bool) {
	sub := resolvePositioned(r, entity)
	if sub == 0 || !actorMoving(r, sub) {
		return 0, 0, false
	}
	return int32(ReadU32(r, sub+moveDestXOff)), int32(ReadU32(r, sub+moveDestYOff)), true
}

// ReadCurrentMoveSpeed returns the entity's current movement speed (0 when stationary),
// the same field the engine's GetCurrentMoveSpeed reads.
func ReadCurrentMoveSpeed(r Reader, entity uint64) float32 {
	sub := resolvePositioned(r, entity)
	if sub == 0 {
		return 0
	}
	return ReadFloat32(r, sub+moveSpeedOff)
}

// Stance index, from GGG's GetStance getter (FUN_141f70650 -> FUN_141d867f0 on the Actor
// component): Actor+0x2BE is the current stance byte index into a per-skill-graph stance
// table (records stride 0x58, name std::wstring @ rec+0x30/+0x40). The index alone is the
// cheap state signal; resolving the name needs the graph-vector walk (Actor+0x210).
const actorStanceIndexOff = 0x2BE

func ReadActorStanceIndex(r Reader, entity uint64) (byte, bool) {
	actor := ResolveComponentByName(r, entity, "Actor")
	if actor == 0 {
		return 0, false
	}
	return byte(ReadU32(r, actor+actorStanceIndexOff) & 0xFF), true
}
