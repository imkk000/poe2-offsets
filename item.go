package gamestate

import (
	"encoding/binary"
	"strings"
)

const (
	rarityVtable         = 0x143395038 // Mods component vtable (was 0x142FEC308, drifted 2026-06-07)
	rarityOffset         = 0x94
	identifiedOffset     = 0x90
	sanctifiedOffset     = 0x92 // Mods comp flags byte (verified live: 2 sanctified wands vs 13 others)
	itemLevelOffset      = 0x240
	reqLevelOffset       = 0x244
	itemTierOffset       = 0x248
	modsVecOffset        = 0x3B8
	intrinsicModsOffset  = 0x148
	allModsOffset        = 0xA0
	modArrayStride       = 0x40
	modArrayValue0Off    = 0x18
	modArrayModsPtrOff   = 0x28
	modGenerationTypeOff = 0x6A        // byte in the Mods.dat row: 1=prefix, 2=suffix (3=implicit)
	stackVtable          = 0x1433F4418 // was 0x143041828, drifted 2026-06-07
	stackOffset          = 0x18
	worldItemBaseOff     = 0x28 // "WorldItem" comp -> inner Metadata/Items/ entity
	renderItemArtOff     = 0x28 // RenderItem comp: char* to the 2D art .dds (IVI art column)
	modsNameVecOff       = 0x18 // Mods comp: StdVector<WordsRow*> (rare/unique name words)
	wordsRowTextOff      = 0x30 // Words.dat row: char* to the (wide) word text

	// Base component flags byte (verified live 2026-06-07 over 11 corrupt / 8 clean
	// mixed-type items). bit0 = corrupted, bit6 = twice-corrupted. bit4 is a
	// distinct, still-unidentified flag (set on some once-corrupt items).
	baseFlagsOff      = 0xC7
	corruptedBit      = 0x01
	twiceCorruptedBit = 0x40

	// Mods component (verified live 2026-06-10, cross-checked vs reference layout
	// ModsComponentOffsets field order). Item-granted skills hang here.
	grantedSkillsOff = 0x210 // StdVector<SkillGem entity*>
	skillGemLevelOff = 0x24  // SkillGem comp: gem level

	// Charges component (flasks/charms; verified live 2026-06-10 vs reference layout).
	chargesInternalPtrOff = 0x10 // -> ChargesInternal
	chargesCurrentOff     = 0x18 // current charges
	chargesPerUseOff      = 0x18 // within ChargesInternal: charge cost per use
)

type GrantedSkill struct {
	Name  string `json:"name"`
	Level int    `json:"level,omitempty"`
}

// FlaskCharges is the live charge state of a flask/charm (Charges component).
type FlaskCharges struct {
	Current int `json:"current"`
	PerUse  int `json:"per_use"` // charge cost per sip; Current/PerUse = sips available
}

// ReadItemCharges reads a flask/charm's live charges from its Charges component
// (+0x18 current; +0x10 -> internal -> +0x18 per-use cost). Verified live 2026-06-10
// (mana flask cur 75 / per-use 10). Cross-checked vs reference layout Charges struct.
// Returns false for items with no Charges component (currency, gear).
func ReadItemCharges(r Reader, itemEntity uint64) (FlaskCharges, bool) {
	ch := ResolveComponentByName(r, itemEntity, "Charges")
	if ch == 0 {
		return FlaskCharges{}, false
	}
	out := FlaskCharges{Current: readU32(r, ch+chargesCurrentOff)}
	if internal := ReadU64(r, ch+chargesInternalPtrOff); internal >= HeapLo && internal < HeapHi {
		out.PerUse = readU32(r, internal+chargesPerUseOff)
	}
	return out, true
}

// ReadItemGrantedSkills reads the item-granted skills from an item entity's Mods
// component (+0x210 StdVector of SkillGem entity pointers). E.g. a wand granting
// Mana Drain, a sceptre granting Fulmination. Each entry resolves its gem level.
func ReadItemGrantedSkills(r Reader, owner uint64) []GrantedSkill {
	mods := ResolveComponentByName(r, owner, "Mods")
	if mods == 0 {
		return nil
	}
	begin := ReadU64(r, mods+grantedSkillsOff)
	end := ReadU64(r, mods+grantedSkillsOff+8)
	if begin < HeapLo || begin >= HeapHi || end <= begin || end-begin > 0x200 {
		return nil
	}
	n := int((end - begin) / 8)
	var out []GrantedSkill
	for i := 0; i < n && i < 8; i++ {
		ent := ReadU64(r, begin+uint64(i)*8)
		if ent < HeapLo || ent >= HeapHi {
			continue
		}
		meta := ReadEntityMetadata(r, ent)
		if meta == "" {
			continue
		}
		gs := GrantedSkill{Name: meta[strings.LastIndexByte(meta, '/')+1:]}
		if sg := ResolveComponentByName(r, ent, "SkillGem"); sg != 0 {
			gs.Level = readU32(r, sg+skillGemLevelOff)
		}
		out = append(out, gs)
	}
	return out
}

// ReadItemCorruption reads the corrupted / twice-corrupted flags from the Base
// component of an item entity (owner). twice implies corrupted.
func ReadItemCorruption(r Reader, owner uint64) (corrupted, twice bool) {
	base := ResolveComponentByName(r, owner, "Base")
	if base == 0 {
		return false, false
	}
	b := ReadByte(r, base+baseFlagsOff)
	return b&corruptedBit != 0, b&twiceCorruptedBit != 0
}

// ReadItemSanctified reads the sanctified flag from the Mods component flags byte
// (+0x92, adjacent to identified +0x90 / rarity +0x94) of an item entity (owner).
func ReadItemSanctified(r Reader, owner uint64) bool {
	mods := ResolveComponentByName(r, owner, "Mods")
	if mods == 0 {
		return false
	}
	return ReadByte(r, mods+sanctifiedOffset) != 0
}

// IsVaalCorrupted reports whether corruption altered the item's mods — a
// Corruption* implicit/enchant or a mutated Vaal explicit (e.g. The Vertex's
// UniqueMutatedVaalGlobalSkillGemLevel = +4 to all skills). Distinct from a
// "normal" corruption that adds no mod. Caller must already know the item is
// corrupted.
func IsVaalCorrupted(mods []ItemModEntry) bool {
	for _, m := range mods {
		if strings.HasPrefix(m.ID, "Corruption") || strings.Contains(m.ID, "Vaal") || strings.Contains(m.ID, "Mutated") {
			return true
		}
	}
	return false
}

var rarityNames = []string{"Normal", "Magic", "Rare", "Unique", "Gem", "Currency", "Quest", "Prophecy"}

type ItemDetails struct {
	ItemPath       string
	BaseName       string
	VisualArt      string
	Rarity         string
	ItemKind       string
	ItemLvl        int
	ReqLvl         int
	ReqStr         int
	ReqDex         int
	ReqInt         int
	Armour         int
	Evasion        int
	EnerShield     int
	Tier           int
	ModCount       int
	ModTexts       []string
	ModStats       []ModStat
	Mods           []ItemModEntry
	Stack          int
	Identified     bool
	Corrupted      bool
	TwiceCorrupted bool
	VaalCorrupted  bool
	Sanctified     bool
}

func ReadWorldItemRarity(r Reader, worldItem uint64) string {
	innerItem := walkToInnerItem(r, worldItem)
	if innerItem == 0 {
		return ""
	}
	comp := ResolveComponentByName(r, innerItem, "Mods")
	if comp == 0 {
		return ""
	}
	v := ReadU32(r, comp+rarityOffset)
	if int(v) >= len(rarityNames) {
		return ""
	}
	return rarityNames[v]
}

func ReadWorldItemDetails(r Reader, worldItem uint64) ItemDetails {
	var d ItemDetails
	innerItem := walkToInnerItem(r, worldItem)
	if innerItem == 0 {
		return d
	}
	d.ItemPath = ReadEntityMetadata(r, innerItem)
	d.BaseName = BaseItemName(d.ItemPath)
	d.VisualArt = ReadItemVisualArt(r, innerItem)
	d.ItemKind = deriveItemType(d.ItemPath)
	d.Rarity, d.ItemLvl, d.ReqLvl, d.Tier, d.ModCount, d.Identified = readItemRarityAndLevels(r, innerItem)
	d.Stack = readItemStack(r, innerItem)
	d.Corrupted, d.TwiceCorrupted = ReadItemCorruption(r, innerItem)
	d.Sanctified = ReadItemSanctified(r, innerItem)
	if comp := ResolveComponentByName(r, innerItem, "Mods"); comp != 0 {
		d.ModTexts, d.ModStats = ReadItemMods(r, comp, d.Rarity)
		d.Mods = ReadItemModEntries(r, comp)
	}
	d.VaalCorrupted = d.Corrupted && IsVaalCorrupted(d.Mods)
	// Base requirements + defenses are data-derived (a ground/unidentified item
	// carries empty Armour/AttributeRequirements components — the values only resolve
	// from the base item type). Quality-adjust defenses. Verified live 2026-06-10
	// (Shamanistic Leggings: str44 int44 armour123 es34 matched the on-screen tooltip).
	quality := 0
	if q := ResolveComponentByName(r, innerItem, "Quality"); q != 0 {
		quality = readU32(r, q+0x18)
	}
	if req := BaseItemRequirementsFor(d.ItemPath); req != nil {
		d.ReqStr, d.ReqDex, d.ReqInt = req.Strength, req.Dexterity, req.Intelligence
		if d.ReqLvl == 0 && req.Level > 0 {
			d.ReqLvl = req.Level
		}
	}
	if props := BaseItemPropertiesFor(d.ItemPath); props != nil {
		mul := 100 + quality
		if props.Armour != nil && props.Armour.Min > 0 {
			d.Armour = props.Armour.Min * mul / 100
		}
		if props.Evasion != nil && props.Evasion.Min > 0 {
			d.Evasion = props.Evasion.Min * mul / 100
		}
		if props.EnergyShield != nil && props.EnergyShield.Min > 0 {
			d.EnerShield = props.EnergyShield.Min * mul / 100
		}
	}
	return d
}

func ReadVecCount(r Reader, vecBeginAddr uint64) int {
	begin := ReadU64(r, vecBeginAddr)
	end := ReadU64(r, vecBeginAddr+8)
	if begin < HeapLo || begin >= HeapHi || end < begin {
		return 0
	}
	n := int((end - begin) / 8)
	if n < 0 || n > 64 {
		return 0
	}
	return n
}

// WorldItemInner returns the real Metadata/Items/ entity wrapped by a ground
// (WorldItem) entity, or 0. Use it to price/inspect a hovered ground item.
func WorldItemInner(r Reader, worldItem uint64) uint64 { return walkToInnerItem(r, worldItem) }

func walkToInnerItem(r Reader, worldItem uint64) uint64 {
	// The ground-item wrapper carries a "WorldItem" component whose +0x28 points to
	// the real Metadata/Items/ entity. Resolve by NAME (the old worldItemBaseVT
	// hardcode drifts every patch: 0x14321A9B8 -> 0x143222D90 on 2026-06-07).
	comp := ResolveComponentByName(r, worldItem, "WorldItem")
	if comp == 0 {
		return 0
	}
	inner := ReadU64(r, comp+worldItemBaseOff)
	if inner >= HeapLo && inner < HeapHi {
		return inner
	}
	return 0
}

// ReadItemVisualArt returns the item's 2D art .dds path (the ItemVisualIdentity
// art column) read straight from the RenderItem component. For uniques this is
// the unique-specific art (e.g. .../Uniques/EnfoldingDawn.dds), giving a stable
// data-grounded identity that the base metadata path does not.
func ReadItemVisualArt(r Reader, owner uint64) string {
	ri := ResolveComponentByName(r, owner, "RenderItem")
	if ri == 0 {
		return ""
	}
	p := ReadU64(r, ri+renderItemArtOff)
	if p < HeapLo || p >= HeapHi {
		return ""
	}
	return readWideString(r, p, 512)
}

// ReadItemGeneratedName returns the item's canonical generated name (rare 2-word
// name OR unique name) straight from the Mods component. Mods+0x18 is a
// StdVector<WordsRow*>; each word's text is the wide string at *(row+0x30). The
// words carry their own spacing (suffix words have a leading space), so they are
// concatenated then trimmed. Verified live: "Enfolding Dawn" (unique), "Morbid
// Tether"/"Viper Star"/"Soul Solace" (rares). Empty for Normal/Magic. This is the
// real game data — supersedes the .dds/UniqueArtName heuristic for naming.
func ReadItemGeneratedName(r Reader, modsComp uint64) string {
	begin := ReadU64(r, modsComp+modsNameVecOff)
	end := ReadU64(r, modsComp+modsNameVecOff+8)
	if begin < HeapLo || begin >= HeapHi || end <= begin {
		return ""
	}
	n := (end - begin) / 0x10
	if n == 0 || n > 8 {
		return ""
	}
	var b strings.Builder
	for i := range n {
		row := ReadU64(r, begin+i*0x10)
		if row < HeapLo || row >= HeapHi {
			continue
		}
		b.WriteString(readWideString(r, ReadU64(r, row+wordsRowTextOff), 128))
	}
	return strings.TrimSpace(b.String())
}

// UniqueArtName derives a unique's display name from its visual-art path. Only
// unique art lives under .../Uniques/<Name>.dds; the basename camelCase-splits to
// the in-game name (e.g. EnfoldingDawn -> "Enfolding Dawn"). Returns "" for
// non-unique art. Heuristic: matches the trade name for the vast majority of
// uniques; a few stylized names ("The ...") need the words table to be exact.
func UniqueArtName(visualArt string) string {
	_, base, found := strings.Cut(visualArt, "/Uniques/")
	if !found {
		return ""
	}
	base = strings.TrimSuffix(base, ".dds")
	if j := strings.IndexByte(base, '.'); j >= 0 {
		base = base[:j]
	}
	if base == "" {
		return ""
	}
	var b strings.Builder
	prevLower := false
	for i, c := range base {
		if i > 0 && c >= 'A' && c <= 'Z' && prevLower {
			b.WriteByte(' ')
		}
		b.WriteRune(c)
		prevLower = c >= 'a' && c <= 'z'
	}
	return b.String()
}

func readWideString(r Reader, addr uint64, maxBytes int) string {
	buf, err := r.ReadBytes(addr, maxBytes)
	if err != nil || len(buf) < 2 {
		return ""
	}
	var b strings.Builder
	for i := 0; i+2 <= len(buf); i += 2 {
		c := binary.LittleEndian.Uint16(buf[i : i+2])
		if c == 0 {
			break
		}
		if c < 0x20 || c > 0x7E {
			return ""
		}
		b.WriteByte(byte(c))
	}
	return b.String()
}

func readItemStack(r Reader, innerItem uint64) int {
	// Resolve by NAME — the Stack vtable drifts every patch, which silently zeroed
	// currency/gold stack counts. Verified live 2026-06-10 (ground gold 1595/157/207
	// matched the on-screen piles). Stack count is at the component's +0x18.
	comp := ResolveComponentByName(r, innerItem, "Stack")
	if comp == 0 {
		return 0
	}
	return readU32(r, comp+stackOffset)
}

func readItemRarityAndLevels(r Reader, innerItem uint64) (rarity string, ilvl, reqLvl, tier, modCount int, identified bool) {
	comp := ResolveComponentByName(r, innerItem, "Mods")
	if comp == 0 {
		return
	}
	v := ReadU32(r, comp+rarityOffset)
	if int(v) < len(rarityNames) {
		rarity = rarityNames[v]
	}
	ilvl = readU32(r, comp+itemLevelOffset)
	reqLvl = readU32(r, comp+reqLevelOffset)
	tier = readU32(r, comp+itemTierOffset)
	identified = ReadByte(r, comp+identifiedOffset) != 0
	modCount = ReadVecCount(r, comp+modsVecOffset)
	if modCount == 0 {
		modCount = ReadVecCount(r, comp+intrinsicModsOffset)
	}
	return
}

func deriveItemType(path string) string {
	p := strings.TrimPrefix(path, "Metadata/Items/")
	if p == path {
		return ""
	}
	if i := strings.Index(p, "/"); i > 0 {
		return p[:i]
	}
	return p
}
