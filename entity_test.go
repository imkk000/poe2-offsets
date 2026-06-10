package gamestate

import "testing"

func TestClassifyEntity(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{

		{"Metadata/Monsters/MonsterMods/Flamewall", ""},
		{"Metadata/Monsters/Anomalies/Firewall", ""},

		{"Metadata/Monsters/NPC/SekhemaAsala", "NPC"},
		{"Metadata/Monsters/LeagueAzmeri/GuidingLight", "AzmeriSpirit"},
		{"Metadata/Monsters/TormentedSpirits/TormentedSpiritoftheBearWild", "TormentedSpirit"},
		{"Metadata/Monsters/LeagueExpeditionNew/RuneEncounterController", "Expedition"},
		{"Metadata/Monsters/VaalMonsters/Living/VaalGuardMortarLiving", "Monster"},

		{"Metadata/MiscellaneousObjects/WorldItem", "WorldItem"},
		{"Metadata/MiscellaneousObjects/Checkpoint", "Checkpoint"},
		{"Metadata/MiscellaneousObjects/Portal_X", "Portal"},
		{"Metadata/MiscellaneousObjects/MultiplexPortal", "Portal"},
		{"Metadata/MiscellaneousObjects/Shrine_X", "Shrine"},
		{"Metadata/Shrines/Shrine", "Shrine"},
		{"Metadata/Shrines/LesserShrine", "Shrine"},
		{"Metadata/MiscellaneousObjects/AreaTransition_Animate_Toggleable", "AreaTransition"},
		{"Metadata/MiscellaneousObjects/Waypoint", "Waypoint"},
		{"Metadata/MiscellaneousObjects/Stash", "Stash"},
		{"Metadata/MiscellaneousObjects/GuildStash", "Stash"},
		{"Metadata/MiscellaneousObjects/HealingWell", "Well"},
		{"Metadata/MiscellaneousObjects/Monolith", "Monolith"},
		{"Metadata/MiscellaneousObjects/Sanctum/SanctumLocker", "Locker"},
		{"Metadata/MiscellaneousObjects/Abyss/AbyssFinalNodeBase", "AbyssNode"},
		{"Metadata/MiscellaneousObjects/Abyss/AbyssCrack", "Abyss"},
		{"Metadata/MiscellaneousObjects/Expedition2/Expedition2Encounter", "Expedition"},
		{"Metadata/MiscellaneousObjects/LeagueIncursionNew/IncursionPedestalEncounter", "VaalBeacon"},
		{"Metadata/MiscellaneousObjects/LeagueIncursionNew/IncursionPedestalCrystal_3", "VaalBeacon"},
		{"Metadata/MiscellaneousObjects/SomethingRandom", "Misc"},

		{"Metadata/Chests/StrongBoxes/MagicStrongBox", "Strongbox"},
		{"Metadata/Chests/Abyss/AbyssChestFinalWeapons", "AbyssChest"},
		{"Metadata/Chests/QuestChests/IslandMap/IslandMapFragment02", "QuestObject"},
		{"Metadata/Chests/PetrosphereCluster01A", "Chest"},

		{"Metadata/NPC/Glyph_X", "Glyph"},
		{"Metadata/NPC/SomeNPC", "NPC"},

		{"Metadata/Characters/Int/IntFour", "Player"},
		{"Metadata/Items/QuestItems/SomeItem", "Item"},
		{"Metadata/Effects/Spells/something", "Effect"},
		{"Metadata/RitualAltar_thing", "Ritual"},

		{"Metadata/QuestObjects/Four_Act2/Gates_Dreadnaught", "QuestObject"},
		{"Metadata/Terrain/Gallows/Act2/2_9_2/Objects/SnakePitDecorativePortal", "Portal"},
		{"Metadata/Terrain/Gallows/Act3/3_3/Objects/Expedition2VenomCryptsInteract", "Expedition"},
		{"Metadata/Terrain/Gallows/Act3/3_3/Objects/InfestedBarrensTransition", "AreaTransition"},
		{"Metadata/Terrain/Gallows/Act2/2_9_2/Objects/SpearWallCentre", "QuestObject"},
		{"Metadata/Terrain/Gallows/Act2/2_9_2/Objects/KinarhaActivator", "QuestObject"},
		{"Metadata/Terrain/Foo/Objects/MachinariumGenerator", "QuestObject"},
		{"Metadata/Terrain/X/Objects/RandomDoodad", "TerrainObject"},
		{"Metadata/Terrain/X", "Terrain"},

		{"", ""},
		{"random/garbage/path", "Other"},
	}
	for _, tc := range tests {
		got := ClassifyEntity(tc.path)
		if got != tc.want {
			t.Errorf("ClassifyEntity(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

func TestIsInteractiveTerrainObject(t *testing.T) {
	yes := []string{
		"Metadata/Terrain/Foo/Objects/SpearWallCentre",
		"Metadata/Terrain/Foo/Objects/LeverChain",
		"Metadata/Terrain/Foo/Objects/GateMain",
		"Metadata/Terrain/Foo/Objects/SwitchA",
		"Metadata/Terrain/Foo/Objects/CrankWheel",
		"Metadata/Terrain/Foo/Objects/PullChain",
		"Metadata/Terrain/Foo/Objects/DoorBig",
		"Metadata/Terrain/Foo/Objects/KinarhaActivator",
		"Metadata/Terrain/Foo/Objects/MachinariumGenerator",
	}
	for _, p := range yes {
		if !isInteractiveTerrainObject(p) {
			t.Errorf("expected interactive: %s", p)
		}
	}
	no := []string{
		"Metadata/Terrain/Foo/Objects/RandomDoodad",
		"Metadata/Terrain/Foo/Objects/DecorativeTorch",
		"",
	}
	for _, p := range no {
		if isInteractiveTerrainObject(p) {
			t.Errorf("expected NOT interactive: %s", p)
		}
	}
}
