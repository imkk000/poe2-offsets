package gamestate

func readComponentList(r Reader, entity uint64) ([]byte, uint64, bool) {
	begin := ReadU64(r, entity+CompListBeginOff)
	end := ReadU64(r, entity+CompListEndOff)
	if !validDataPtr(begin) || end < begin {
		return nil, 0, false
	}
	count := min((end-begin)/8, MaxComponents)
	if count == 0 {
		return nil, 0, true
	}
	data, err := r.ReadBytes(begin, int(count*8))
	if err != nil || uint64(len(data)) < count*8 {
		return nil, 0, false
	}
	return data, count, true
}

func FindLifeComponent(r Reader, entity uint64) uint64 {
	return ResolveComponentByName(r, entity, "Life")
}

func readU32(r Reader, addr uint64) int {
	return int(ReadU32(r, addr))
}
