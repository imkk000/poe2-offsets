package gamestate

const actorCurrentTargetOff = 0x900

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

const (
	entityPositionedOff = 0x98
	moveSpeedOff        = 0x28C
	moveFlagOff         = 0x1E5
	movePrimaryOff      = 0x240
	moveAltYOff         = 0x23C
	moveSentinelYOff    = 0x244
	moveAxisFlagOff     = 0x22D
	moveYSourceFlagOff  = 0x22E
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

func ReadActorMoving(r Reader, entity uint64) bool {
	sub := resolvePositioned(r, entity)
	return sub != 0 && actorMoving(r, sub)
}

func ReadMovementDestination(r Reader, entity uint64) (x, y int32, ok bool) {
	sub := resolvePositioned(r, entity)
	if sub == 0 || !actorMoving(r, sub) {
		return 0, 0, false
	}
	primary := int32(ReadU32(r, sub+movePrimaryOff))
	ySrc := int32(ReadU32(r, sub+moveAltYOff))
	if ReadByte(r, sub+moveYSourceFlagOff) != 0 {
		ySrc = int32(ReadU32(r, sub+moveSentinelYOff))
	}
	if ReadByte(r, sub+moveAxisFlagOff) != 0 {
		return primary, ySrc, true
	}
	return ySrc, primary, true
}

func ReadCurrentMoveSpeed(r Reader, entity uint64) float32 {
	sub := resolvePositioned(r, entity)
	if sub == 0 {
		return 0
	}
	return ReadFloat32(r, sub+moveSpeedOff)
}

const (
	actorActionFlagsOff    = 0x2A0
	actionFlagDead         = 0x40
	actionFlagUsingAbility = 0x02
)

func ReadActionFlags(r Reader, entity uint64) (byte, bool) {
	actor := ResolveComponentByName(r, entity, "Actor")
	if actor == 0 {
		return 0, false
	}
	return ReadByte(r, actor+actorActionFlagsOff), true
}

func IsUsingAbility(r Reader, entity uint64) bool {
	f, ok := ReadActionFlags(r, entity)
	return ok && f&actionFlagUsingAbility != 0
}

func IsActorDead(r Reader, entity uint64) bool {
	f, ok := ReadActionFlags(r, entity)
	return ok && f&actionFlagDead != 0
}

const (
	animatedObjectOff = 0x350
	animationIDOff    = 0x190
	animationStartOff = 0x1B8
	animationEndOff   = 0x1BC
)

func resolveAnimationController(r Reader, entity uint64) uint64 {
	animComp := ResolveComponentByName(r, entity, "Animated")
	if animComp == 0 {
		return 0
	}
	animObj := ReadU64(r, animComp+animatedObjectOff)
	if animObj < HeapLo || animObj >= HeapHi {
		return 0
	}
	return ResolveComponentByName(r, animObj, "AnimationController")
}

func ReadAnimationID(r Reader, entity uint64) (int32, bool) {
	ac := resolveAnimationController(r, entity)
	if ac == 0 {
		return 0, false
	}
	id := int32(ReadU32(r, ac+animationIDOff))
	return id, id != -1
}

func ReadAnimationLength(r Reader, entity uint64) (float32, bool) {
	ac := resolveAnimationController(r, entity)
	if ac == 0 || int32(ReadU32(r, ac+animationIDOff)) == -1 {
		return 0, false
	}
	return ReadFloat32(r, ac+animationEndOff) - ReadFloat32(r, ac+animationStartOff), true
}

const actorStanceIndexOff = 0x2BE

func ReadActorStanceIndex(r Reader, entity uint64) (byte, bool) {
	actor := ResolveComponentByName(r, entity, "Actor")
	if actor == 0 {
		return 0, false
	}
	return byte(ReadU32(r, actor+actorStanceIndexOff) & 0xFF), true
}
