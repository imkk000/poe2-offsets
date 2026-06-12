package gamestate

func ReadPathfinderSpeed(r Reader, entity uint64) (float32, bool) {
	comp := ResolveComponentByName(r, entity, "Pathfinding")
	if comp == 0 {
		return 0, false
	}
	if cached := ReadFloat32(r, comp+0x554); cached >= 0 {
		return cached, true
	}
	return ReadFloat32(r, comp+0x550) * ReadFloat32(r, comp+0x55C), true
}
