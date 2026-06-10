package gamestate

// Actor combat state. Offsets found 2026-06-10 by observing the local player's Actor
// during live manual combat (a live probe): Actor+0x900 held a pointer that
// tracked the current target across every monster the player switched to. Resolve the
// component by name so it survives vtable drift.

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
