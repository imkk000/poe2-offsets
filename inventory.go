package gamestate

import (
	"encoding/binary"
	"fmt"
	"strings"
)

const (
	RarityCompVtable = rarityVtable
	StackCompVtable  = stackVtable
	SlotCompVtable   = 0x143038F20

	rarityBackLinkOff   = 0x08
	stackRecordOwnerOff = 0x08
	stackRecordCountOff = 0x18
	slotRecordOwnerOff  = 0x08
	entityDetailsOff    = 0x08
	detailsPathOff      = 0x08
)

type InventoryItem struct {
	Owner          string         `json:"owner"`
	RarityComp     string         `json:"rarity_comp"`
	Path           string         `json:"path"`
	BaseName       string         `json:"base_name,omitempty"`
	FullName       string         `json:"full_name,omitempty"`
	VisualArt      string         `json:"visual_art,omitempty"`
	Category       string         `json:"category,omitempty"`
	ItemClass      string         `json:"item_class,omitempty"`
	Rarity         string         `json:"rarity,omitempty"`
	ItemLvl        int            `json:"ilvl,omitempty"`
	ReqLvl         int            `json:"req_lvl,omitempty"`
	ReqStr         int            `json:"req_str,omitempty"`
	ReqDex         int            `json:"req_dex,omitempty"`
	ReqInt         int            `json:"req_int,omitempty"`
	Tier           int            `json:"tier,omitempty"`
	ModCount       int            `json:"mods,omitempty"`
	Stack          int            `json:"stack,omitempty"`
	Quality        int            `json:"quality,omitempty"`
	Sockets        int            `json:"sockets,omitempty"`
	SocketsMax     int            `json:"sockets_max,omitempty"`
	SocketedItems  []string       `json:"socketed,omitempty"`
	GrantedSkills  []GrantedSkill `json:"granted_skills,omitempty"`
	Armour         int            `json:"armour,omitempty"`
	Evasion        int            `json:"evasion,omitempty"`
	EnerShield     int            `json:"es,omitempty"`
	Width          int            `json:"w,omitempty"`
	Height         int            `json:"h,omitempty"`
	Container      string         `json:"container,omitempty"`
	ModTexts       []string       `json:"mod_texts,omitempty"`
	ModStats       []ModStat      `json:"mod_stats,omitempty"`
	Mods           []ItemModEntry `json:"mods_detail,omitempty"`
	Identified     bool           `json:"identified"`
	Corrupted      bool           `json:"corrupted,omitempty"`
	TwiceCorrupted bool           `json:"twice_corrupted,omitempty"`
	VaalCorrupted  bool           `json:"vaal_corrupted,omitempty"`
	Sanctified     bool           `json:"sanctified,omitempty"`
}

type ItemModEntry struct {
	ID     string  `json:"id"`
	Value  int32   `json:"value"`
	Values []int32 `json:"values,omitempty"`
	Slot   string  `json:"slot"`
}

type ModStat struct {
	Stat  string `json:"stat"`
	Value int32  `json:"value"`
}

func ReadItemModTexts(r Reader, rarityComp uint64, rarity string) []string {
	texts, _ := ReadItemMods(r, rarityComp, rarity)
	return texts
}

func ReadItemMods(r Reader, rarityComp uint64, rarity string) ([]string, []ModStat) {

	var offs []uint64
	switch rarity {
	case "Magic", "Rare", "Unique":
		offs = []uint64{0x148}
	default:
		return nil, nil
	}
	var texts []string
	var stats []ModStat
	seen := make(map[uint32]struct{})
	for _, off := range offs {
		begin := ReadU64(r, rarityComp+off)
		end := ReadU64(r, rarityComp+off+8)
		if begin < HeapLo || begin >= HeapHi || end <= begin {
			continue
		}
		count := int((end - begin) / 8)
		if count <= 0 || count > 64 {
			continue
		}
		data, err := r.ReadBytes(begin, count*8)
		if err != nil || len(data) < count*8 {
			continue
		}
		for i := range count {
			pair := binary.LittleEndian.Uint64(data[i*8 : i*8+8])
			memID := uint32(pair & 0xFFFFFFFF)
			value := int32(pair >> 32)
			if _, dup := seen[memID]; dup {
				continue
			}
			seen[memID] = struct{}{}
			name := StatNameByMemId(memID)
			if name == "" {
				texts = append(texts, fmt.Sprintf("(unknown #%d = %d)", memID, value))
				continue
			}
			stats = append(stats, ModStat{Stat: name, Value: value})
			desc, ok := StatDescriptionByName(name)
			if !ok || len(desc.Formats) == 0 {
				texts = append(texts, fmt.Sprintf("(%s = %d)", name, value))
				continue
			}
			texts = append(texts, applyStatTemplate(desc.Formats[0].Text, value))
		}
	}
	return texts, stats
}

func MapModText(statID uint32, value int32) (string, bool) {
	name := StatNameByMemId(statID)
	if !strings.HasPrefix(name, "map_") {
		return "", false
	}
	if desc, ok := StatDescriptionByName(name); ok && len(desc.Formats) > 0 {
		if t := applyStatTemplate(desc.Formats[0].Text, value); t != "" {

			return strings.ReplaceAll(t, "\\n", "\n"), true
		}
	}
	s := strings.TrimPrefix(name, "map_")
	s = strings.ReplaceAll(s, "_+%_final_from_map", "")
	s = strings.ReplaceAll(s, "_final_from_map", "")
	s = strings.ReplaceAll(s, "_", " ")
	s = strings.TrimSpace(s)

	if !strings.Contains(s, " ") {
		return "", false
	}
	return s, true
}

func applyStatTemplate(template string, value int32) string {
	s := template
	s = substituteFormatPlaceholders(s, value)
	return stripRichText(s)
}

func substituteFormatPlaceholders(s string, value int32) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		if s[i] != '{' {
			b.WriteByte(s[i])
			i++
			continue
		}
		end := strings.IndexByte(s[i:], '}')
		if end < 0 {
			b.WriteByte(s[i])
			i++
			continue
		}
		spec := s[i+1 : i+end]
		if spec == "" {
			b.WriteString("???")
			i += end + 1
			continue
		}
		var fmtSpec string
		if _, after, ok := strings.Cut(spec, ":"); ok {
			fmtSpec = after
		}
		b.WriteString(formatStatValue(value, fmtSpec))
		i += end + 1
	}
	return b.String()
}

func formatStatValue(value int32, spec string) string {
	switch {
	case spec == "" || spec == "d":
		return fmt.Sprintf("%d", value)
	case spec == "+d":
		if value >= 0 {
			return fmt.Sprintf("+%d", value)
		}
		return fmt.Sprintf("%d", value)
	case strings.HasSuffix(spec, "%"):
		return fmt.Sprintf("%d", value)
	}
	return fmt.Sprintf("%d", value)
}

func stripRichText(s string) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		c := s[i]
		if c == '[' {
			end := strings.IndexByte(s[i:], ']')
			if end > 0 {
				inner := s[i+1 : i+end]
				if _, after, ok := strings.Cut(inner, "|"); ok {
					b.WriteString(after)
				} else {
					b.WriteString(inner)
				}
				i += end + 1
				continue
			}
		}
		b.WriteByte(c)
		i++
	}
	return b.String()
}

func ReadItemDetailsByOwner(r Reader, owner uint64, it *InventoryItem) {

	if q := ResolveComponentByName(r, owner, "Quality"); q != 0 {
		it.Quality = readU32(r, q+0x18)
	}
	if s := ResolveComponentByName(r, owner, "Sockets"); s != 0 {

		it.SocketsMax = int(readU32(r, s+0x18))
		begin := ReadU64(r, s+0x60)
		end := ReadU64(r, s+0x68)
		if begin >= HeapLo && begin < HeapHi && end >= begin && end-begin <= 16 {
			it.Sockets = int(end - begin)
		} else {
			it.Sockets = it.SocketsMax
		}

		for i := 0; i < it.Sockets && i < 8; i++ {
			p := ReadU64(r, s+0x30+uint64(i)*8)
			if p < HeapLo || p >= HeapHi {
				continue
			}
			if meta := ReadEntityMetadata(r, p); meta != "" {
				it.SocketedItems = append(it.SocketedItems, meta[strings.LastIndexByte(meta, '/')+1:])
			}
		}
	}
	it.Corrupted, it.TwiceCorrupted = ReadItemCorruption(r, owner)
	it.Sanctified = ReadItemSanctified(r, owner)
	it.GrantedSkills = ReadItemGrantedSkills(r, owner)
	it.VisualArt = ReadItemVisualArt(r, owner)
	req := BaseItemRequirementsFor(it.Path)
	if req != nil {
		it.ReqStr = req.Strength
		it.ReqDex = req.Dexterity
		it.ReqInt = req.Intelligence
		if it.ReqLvl == 0 && req.Level > 0 {
			it.ReqLvl = req.Level
		}
	}
	props := BaseItemPropertiesFor(it.Path)
	if props != nil {
		q := it.Quality
		mul := 100 + q
		if props.Armour != nil && props.Armour.Min > 0 {
			it.Armour = (props.Armour.Min * mul) / 100
		}
		if props.Evasion != nil && props.Evasion.Min > 0 {
			it.Evasion = (props.Evasion.Min * mul) / 100
		}
		if props.EnergyShield != nil && props.EnergyShield.Min > 0 {
			it.EnerShield = (props.EnergyShield.Min * mul) / 100
		}
	}
}

func ReadItemFromStackRecord(r Reader, stackRecord uint64) (InventoryItem, bool) {
	var it InventoryItem
	owner := ReadU64(r, stackRecord+stackRecordOwnerOff)
	if owner < HeapLo || owner >= HeapHi {
		return it, false
	}
	details := ReadU64(r, owner+entityDetailsOff)
	if details < HeapLo || details >= HeapHi {
		return it, false
	}
	pathPtr := ReadU64(r, details+detailsPathOff)
	if pathPtr < HeapLo || pathPtr >= HeapHi {
		return it, false
	}
	path := readPathString(r, pathPtr)
	if path == "" {
		return it, false
	}
	it.Owner = formatHex(owner)
	it.RarityComp = formatHex(stackRecord)
	it.Path = path
	it.BaseName = BaseItemName(path)
	it.ItemClass = BaseItemClass(path)
	it.Width, it.Height = BaseItemSize(path)
	it.Category = deriveCategory(path)
	it.Rarity = "Currency"
	it.Stack = readU32(r, stackRecord+stackRecordCountOff)
	it.Identified = true
	return it, true
}

func ReadItemFromRarityComp(r Reader, rarityComp uint64) (InventoryItem, bool) {
	var it InventoryItem
	owner := ReadU64(r, rarityComp+rarityBackLinkOff)
	if owner < HeapLo || owner >= HeapHi {
		return it, false
	}
	details := ReadU64(r, owner+entityDetailsOff)
	if details < HeapLo || details >= HeapHi {
		return it, false
	}
	pathPtr := ReadU64(r, details+detailsPathOff)
	if pathPtr < HeapLo || pathPtr >= HeapHi {
		return it, false
	}
	path := readPathString(r, pathPtr)
	if path == "" {
		return it, false
	}
	it.Owner = formatHex(owner)
	it.RarityComp = formatHex(rarityComp)
	it.Path = path
	it.BaseName = BaseItemName(path)
	it.ItemClass = BaseItemClass(path)
	it.Width, it.Height = BaseItemSize(path)
	it.Category = deriveCategory(path)
	rar, ilvl, req, tier, mods, ident := readItemRarityAndLevels(r, owner)
	it.Rarity = rar
	it.ItemLvl = ilvl
	it.ReqLvl = req
	it.Tier = tier
	it.ModCount = mods
	it.Identified = ident
	it.Stack = readItemStack(r, owner)
	ReadItemDetailsByOwner(r, owner, &it)
	it.ModTexts, it.ModStats = ReadItemMods(r, rarityComp, it.Rarity)
	it.Mods = ReadItemModEntries(r, rarityComp)
	it.VaalCorrupted = it.Corrupted && IsVaalCorrupted(it.Mods)
	it.FullName = BuildItemDisplayName(it.Rarity, it.BaseName, it.Mods)
	if it.Rarity == "Rare" || it.Rarity == "Unique" {
		if name := ReadItemGeneratedName(r, rarityComp); name != "" {
			if it.Rarity == "Rare" && it.BaseName != "" {
				it.FullName = name + " " + it.BaseName
			} else {
				it.FullName = name
			}
		}
	}
	return it, true
}

func BuildItemDisplayName(rarity, baseName string, mods []ItemModEntry) string {
	if rarity != "Magic" || baseName == "" {
		return baseName
	}
	var prefix, suffix string
	for _, m := range mods {
		info, ok := ModInfoByID(m.ID)
		if !ok || info.Name == "" {
			continue
		}
		switch info.GenerationType {
		case "prefix":
			if prefix == "" {
				prefix = info.Name
			}
		case "suffix":
			if suffix == "" {
				suffix = info.Name
			}
		}
	}
	name := baseName
	if prefix != "" {
		name = prefix + " " + name
	}
	if suffix != "" {
		name = name + " " + suffix
	}
	return name
}

var modSlotNames = [5]string{"implicit", "explicit", "enchant", "hellscape", "crucible"}

func readModValues(r Reader, elems []byte, base int) []int32 {
	begin := binary.LittleEndian.Uint64(elems[base : base+8])
	end := binary.LittleEndian.Uint64(elems[base+8 : base+16])
	if begin < HeapLo || begin >= HeapHi || end <= begin || end-begin > 64 || (end-begin)%4 != 0 {
		return nil
	}
	cnt := int((end - begin) / 4)
	raw, err := r.ReadBytes(begin, cnt*4)
	if err != nil || len(raw) < cnt*4 {
		return nil
	}
	out := make([]int32, cnt)
	for i := range cnt {
		out[i] = int32(binary.LittleEndian.Uint32(raw[i*4:]))
	}
	return out
}

func ReadItemModEntries(r Reader, rarityComp uint64) []ItemModEntry {
	buf, err := r.ReadBytes(rarityComp+allModsOffset, 5*24)
	if err != nil || len(buf) < 5*24 {
		return nil
	}
	var out []ItemModEntry
	for slot := range 5 {
		vbase := slot * 24
		begin := binary.LittleEndian.Uint64(buf[vbase : vbase+8])
		end := binary.LittleEndian.Uint64(buf[vbase+8 : vbase+16])
		if !validDataPtr(begin) || end <= begin {
			continue
		}
		span := end - begin
		if span%modArrayStride != 0 {
			continue
		}
		n := int(span / modArrayStride)

		if n <= 0 || n > 64 {
			continue
		}
		elems, err := r.ReadBytes(begin, n*modArrayStride)
		if err != nil || len(elems) < n*modArrayStride {
			continue
		}
		for i := range n {
			base := i * modArrayStride

			values := readModValues(r, elems, base)
			value := int32(binary.LittleEndian.Uint32(elems[base+modArrayValue0Off : base+modArrayValue0Off+4]))
			if len(values) > 0 {
				value = values[0]
			}
			rowPtr := binary.LittleEndian.Uint64(elems[base+modArrayModsPtrOff : base+modArrayModsPtrOff+8])
			if rowPtr < HeapLo || rowPtr >= HeapHi {
				continue
			}
			strPtr := ReadU64(r, rowPtr)
			if strPtr < HeapLo || strPtr >= HeapHi {
				continue
			}
			id := readUTF16String(r, strPtr, 64)
			if id == "" {
				continue
			}
			slotLabel := modSlotNames[slot]
			if slot == 1 {
				switch ReadByte(r, rowPtr+modGenerationTypeOff) {
				case 1:
					slotLabel = "prefix"
				case 2:
					slotLabel = "suffix"
				}
			}
			out = append(out, ItemModEntry{ID: id, Value: value, Values: values, Slot: slotLabel})
		}
	}
	return out
}

func readPathString(r Reader, addr uint64) string {
	buf, err := r.ReadBytes(addr, 256)
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

func deriveCategory(path string) string {
	p := strings.TrimPrefix(path, "Metadata/Items/")
	if p == path {
		return ""
	}
	parts := strings.SplitN(p, "/", 3)
	if len(parts) >= 2 && (parts[0] == "Armours" || parts[0] == "Weapons") {
		return parts[0] + "/" + parts[1]
	}
	return parts[0]
}
