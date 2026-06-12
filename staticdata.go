package gamestate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
)

type BaseItemEntry struct {
	Name            string                `json:"name"`
	ItemClass       string                `json:"item_class"`
	DropLevel       int                   `json:"drop_level"`
	InventoryWidth  int                   `json:"inventory_width"`
	InventoryHeight int                   `json:"inventory_height"`
	Properties      *BaseItemProperties   `json:"properties,omitempty"`
	Requirements    *BaseItemRequirements `json:"requirements,omitempty"`
}

type BaseItemProperties struct {
	Armour       *MinMax `json:"armour,omitempty"`
	Evasion      *MinMax `json:"evasion,omitempty"`
	EnergyShield *MinMax `json:"energy_shield,omitempty"`
	StackSize    *int    `json:"stack_size,omitempty"`
	ChargesMax   *int    `json:"charges_max,omitempty"`
}

type BaseItemRequirements struct {
	Strength     int `json:"strength,omitempty"`
	Dexterity    int `json:"dexterity,omitempty"`
	Intelligence int `json:"intelligence,omitempty"`
	Level        int `json:"level,omitempty"`
}

type MinMax struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

type MonsterEntry struct {
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
}

type BuffEntry struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Invisible   bool     `json:"invisible"`
	Removable   bool     `json:"removable"`
	Stats       []string `json:"stats,omitempty"`
}

type SkillEntry struct {
	DisplayName string   `json:"display_name"`
	Description string   `json:"description"`
	Types       []string `json:"types,omitempty"`
}

type StatDescription struct {
	StatNames []string         `json:"stat_names"`
	Formats   []StatDescFormat `json:"formats"`
}

type StatDescFormat struct {
	Ranges []string `json:"ranges"`
	Text   string   `json:"text"`
}

type ModInfo struct {
	Name           string `json:"name"`
	GenerationType string `json:"generation_type"`
	RequiredLevel  int    `json:"required_level"`
}

var (
	baseItemsLoaded    atomic.Pointer[map[string]BaseItemEntry]
	monstersLoaded     atomic.Pointer[map[string]MonsterEntry]
	buffsLoaded        atomic.Pointer[map[string]BuffEntry]
	skillsLoaded       atomic.Pointer[map[string]SkillEntry]
	statsLoaded        atomic.Pointer[[]string]
	statDescsLoaded    atomic.Pointer[map[string]StatDescription]
	modsInfoLoaded     atomic.Pointer[map[string]ModInfo]
	passiveNodesLoaded atomic.Pointer[map[int]PassiveNode]
)

func ModInfoByID(id string) (ModInfo, bool) {
	m := modsInfoLoaded.Load()
	if m == nil {
		return ModInfo{}, false
	}
	v, ok := (*m)[id]
	return v, ok
}

func LoadStaticData(dir string) error {
	exe, err := os.Executable()
	if err == nil && !filepath.IsAbs(dir) {
		dir = filepath.Join(filepath.Dir(exe), dir)
	}
	var bi map[string]BaseItemEntry
	if err := loadJSON(filepath.Join(dir, "base_items.json"), &bi); err != nil {
		return fmt.Errorf("base_items: %w", err)
	}
	baseItemsLoaded.Store(&bi)

	var ms map[string]MonsterEntry
	if err := loadJSON(filepath.Join(dir, "monsters.json"), &ms); err == nil {
		monstersLoaded.Store(&ms)
	}
	var bf map[string]BuffEntry
	if err := loadJSON(filepath.Join(dir, "buffs.json"), &bf); err == nil {
		buffsLoaded.Store(&bf)
	}
	var sk map[string]SkillEntry
	if err := loadJSON(filepath.Join(dir, "skills.json"), &sk); err == nil {
		skillsLoaded.Store(&sk)
	}

	var stats []string
	if err := loadJSON(filepath.Join(dir, "stats.json"), &stats); err == nil {
		statsLoaded.Store(&stats)
	}
	var descs map[string]StatDescription
	if err := loadJSON(filepath.Join(dir, "stat_descriptions.json"), &descs); err == nil {
		statDescsLoaded.Store(&descs)
	}
	var mods map[string]ModInfo
	if err := loadJSON(filepath.Join(dir, "mods.json"), &mods); err == nil {
		modsInfoLoaded.Store(&mods)
	}
	var pn map[string]PassiveNode
	if err := loadJSON(filepath.Join(dir, "passive_nodes.json"), &pn); err == nil {
		nodes := make(map[int]PassiveNode, len(pn))
		for k, v := range pn {
			if id, err := strconv.Atoi(k); err == nil {
				nodes[id] = v
			}
		}
		passiveNodesLoaded.Store(&nodes)
	}
	return nil
}

func StatNameByMemId(memId uint32) string {
	stats := statsLoaded.Load()
	if stats == nil {
		return ""
	}
	row := int(memId) - 1
	if row < 0 || row >= len(*stats) {
		return ""
	}
	return (*stats)[row]
}

func StatDescriptionByName(name string) (StatDescription, bool) {
	descs := statDescsLoaded.Load()
	if descs == nil {
		return StatDescription{}, false
	}
	d, ok := (*descs)[name]
	return d, ok
}

func loadJSON(path string, v any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return json.NewDecoder(f).Decode(v)
}

func BaseItemName(metadataPath string) string {
	if e, ok := baseItemEntry(metadataPath); ok {
		return e.Name
	}
	return ""
}

func BaseItemSize(metadataPath string) (int, int) {
	if e, ok := baseItemEntry(metadataPath); ok {
		return e.InventoryWidth, e.InventoryHeight
	}
	return 0, 0
}

func BaseItemClass(metadataPath string) string {
	if e, ok := baseItemEntry(metadataPath); ok {
		return e.ItemClass
	}
	return ""
}

func BaseItemRequirementsFor(metadataPath string) *BaseItemRequirements {
	if e, ok := baseItemEntry(metadataPath); ok {
		return e.Requirements
	}
	return nil
}

func BaseItemPropertiesFor(metadataPath string) *BaseItemProperties {
	if e, ok := baseItemEntry(metadataPath); ok {
		return e.Properties
	}
	return nil
}

func baseItemEntry(metadataPath string) (BaseItemEntry, bool) {
	m := baseItemsLoaded.Load()
	if m == nil {
		return BaseItemEntry{}, false
	}
	e, ok := (*m)[metadataPath]
	return e, ok
}

func MonsterName(metadataPath string) string {
	m := monstersLoaded.Load()
	if m == nil {
		return ""
	}
	if e, ok := (*m)[metadataPath]; ok {
		return e.Name
	}
	return ""
}

func MonsterType(metadataPath string) string {
	m := monstersLoaded.Load()
	if m == nil {
		return ""
	}
	if e, ok := (*m)[metadataPath]; ok {
		return e.Type
	}
	return ""
}

func BuffByID(id string) (BuffEntry, bool) {
	m := buffsLoaded.Load()
	if m == nil {
		return BuffEntry{}, false
	}
	e, ok := (*m)[id]
	return e, ok
}

func SkillByID(id string) (SkillEntry, bool) {
	m := skillsLoaded.Load()
	if m == nil {
		return SkillEntry{}, false
	}
	e, ok := (*m)[id]
	return e, ok
}
