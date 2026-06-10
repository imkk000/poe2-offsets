package gamestate

import (
	"strconv"
	"strings"
)

type Jewel struct {
	Name     string   `json:"name"`
	Rarity   string   `json:"rarity"`
	ItemLvl  int      `json:"ilvl,omitempty"`
	Metadata string   `json:"metadata"`
	Mods     []string `json:"mods,omitempty"`
	Grants   []string `json:"grants,omitempty"`
}

func ReadJewels(r Reader, regions []HeapRegion) []Jewel {
	var out []Jewel
	seen := make(map[string]bool)
	for _, ent := range ScanItemEntities(r, regions) {
		meta := ReadEntityMetadata(r, ent)
		if !strings.Contains(meta, "Metadata/Items/Jewels") {
			continue
		}
		mc := ResolveComponentByName(r, ent, "Mods")
		if mc == 0 {
			continue
		}
		it, ok := ReadItemFromRarityComp(r, mc)
		if !ok {
			continue
		}
		ReadItemDetailsByOwner(r, ent, &it)
		name := it.FullName
		if name == "" {
			name = it.BaseName
		}
		key := meta + "|" + name + "|" + strings.Join(it.ModTexts, ";")
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, Jewel{
			Name:     name,
			Rarity:   it.Rarity,
			ItemLvl:  it.ItemLvl,
			Metadata: meta,
			Mods:     it.ModTexts,
			Grants:   resolveGrants(it.ModTexts),
		})
	}
	return out
}

func resolveGrants(mods []string) []string {
	var grants []string
	for _, m := range mods {
		rest, ok := strings.CutPrefix(m, "Allocates ")
		if !ok {
			continue
		}
		id, err := strconv.Atoi(strings.TrimSpace(rest))
		if err != nil {
			continue
		}
		if node, ok := PassiveNodeByID(id); ok && node.Name != "" {
			grants = append(grants, node.Name)
		}
	}
	return grants
}
