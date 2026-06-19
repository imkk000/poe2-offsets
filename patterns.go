package gamestate

const (
	GsoPattern  = "48 39 2D ?? ?? ?? ?? 0F 85 16 01 00 00"
	GsoDispAt   = 3
	GsoInstrEnd = 7
	GsoAnchor   = "Unable to get InGameState"

	HPBarPattern     = "0F 48 CB 39 4E 38 7C"
	HPBarPatchOffset = 6
	HPBarOnByte      = 0xEB
	HPBarOffByte     = 0x7C

	MapRevealPattern = "?? 05 0F 28 ?? EB 04 41 ?? ?? ?? F3 0F 11 ?? ?? ?? ?? ?? 45 ?? ?? BA 19 00 00 00 48 8D 0D"
	MapRevealOnByte  = 0x75
	MapRevealOffByte = 0x74

	LightRadiusPattern  = "F3 44 0F 58 C6 F3 44 0F 59 3D"
	LightRadiusMulssOff = 5
	LightRadiusDispOff  = 5
	LightRadiusInstrLen = 9
)
