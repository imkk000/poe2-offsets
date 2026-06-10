package gamestate

const (
	minimapIconCompletedOff = 0x10
	minimapIconDatRowOff    = 0x20
)

type MinimapIconInfo struct {
	Present   bool
	Name      string
	Completed bool
}

func ReadMinimapIcon(r Reader, entity uint64) MinimapIconInfo {
	comp := ResolveComponentByName(r, entity, "MinimapIcon")
	if comp == 0 {
		return MinimapIconInfo{}
	}
	info := MinimapIconInfo{
		Present:   true,
		Completed: ReadU32(r, comp+minimapIconCompletedOff) != 0,
	}
	if row := ReadU64(r, comp+minimapIconDatRowOff); validDataPtr(row) {
		if str := ReadU64(r, row); validDataPtr(str) {
			info.Name = readUTF16String(r, str, 64)
		}
	}
	return info
}
