package gamestate

// League-mechanic readers cracked from GGG's GEAL getters (no-arg closures reached via
// the validator -> closure-vtable+0x10 -> evaluator chain). The Player component holds a
// large per-player league-state object at Player+0x430; different leagues store their
// counters at fixed offsets inside it (Incursion tokens @ +0x81D8). Global-context
// leagues (Azmeri sacred water) hang off a world-manager singleton instead and need that
// root resolved before they can be wired.

const (
	playerLeagueStateOff   = 0x430
	incursionTokensOff     = 0x81D8
	incursionActiveFlagOff = 0x8220
)

// ReadIncursionTokens reads the player's current Incursion token count from GGG's
// GetCurrentIncursionTokens getter (FUN_141843240): Player component +0x430 league-state
// object, token byte @ +0x81D8, gated by the active flag @ +0x8220. ok is false outside
// an Incursion (flag clear / state object absent).
func ReadIncursionTokens(r Reader, entity uint64) (int, bool) {
	player := ResolveComponentByName(r, entity, "Player")
	if player == 0 {
		return 0, false
	}
	state := ReadU64(r, player+playerLeagueStateOff)
	if state < HeapLo || state >= HeapHi {
		return 0, false
	}
	if ReadU32(r, state+incursionActiveFlagOff)&0xFF == 0 {
		return 0, false
	}
	return int(ReadU32(r, state+incursionTokensOff) & 0xFF), true
}
