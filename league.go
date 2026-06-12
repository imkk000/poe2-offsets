package gamestate

const (
	playerLeagueStateOff   = 0x430
	incursionTokensOff     = 0x81D8
	incursionActiveFlagOff = 0x8220
)

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
