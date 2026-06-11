package gamestate

const (
	entityStatStoreOff = 0x28
	actorRefTableOff   = 0x78

	hashEntryStride   = 0x58
	hashEntryValueOff = 0x10
	hashValueTypeOff  = 0x20
	hashValueFloat    = 0x0C

	refValueStride   = 0x28
	refValueValidOff = 0x20

	bossPosSubOff = 0x98
	bossPosXOff   = 0x490

	currentBossHashKey   = 0x9B677236
	flameblastChargeHash = 0xE7C628CE
	bannerStagesHash     = 0xFFB34194
	killOwnerHash        = 0x4082B5B7
	minionOwnerHash      = 0x2442801A
	effectCasterHash     = 0x40F0FC9C
	curseCasterHash      = 0x14406536

	hashValueRefValid = 0x01
)

func hashEntry(r Reader, store uint64, key uint32) (uint64, bool) {
	begin := ReadU64(r, store+0x08)
	end := ReadU64(r, store+0x10)
	if begin < HeapLo || end < begin || (end-begin)%hashEntryStride != 0 {
		return 0, false
	}
	n := (end - begin) / hashEntryStride
	lo, hi := uint64(0), n
	for lo < hi {
		mid := (lo + hi) / 2
		if ReadU32(r, begin+mid*hashEntryStride) < key {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo >= n || ReadU32(r, begin+lo*hashEntryStride) != key {
		return 0, false
	}
	return begin + lo*hashEntryStride, true
}

func ReadEntityHashStat(r Reader, entity uint64, hash uint32) (float32, bool) {
	entry, ok := hashEntry(r, entity+entityStatStoreOff, hash)
	if !ok {
		return 0, false
	}
	vbegin := ReadU64(r, entry+hashEntryValueOff)
	vend := ReadU64(r, entry+hashEntryValueOff+8)
	if vbegin < HeapLo || vbegin >= vend {
		return 0, false
	}
	if byte(ReadU32(r, vbegin+hashValueTypeOff)) != hashValueFloat {
		return 0, false
	}
	return ReadFloat32(r, vbegin), true
}

func ReadFlameblastCharge(r Reader, entity uint64) (float32, bool) {
	return ReadEntityHashStat(r, entity, flameblastChargeHash)
}

func ReadBannerStages(r Reader, entity uint64) (float32, bool) {
	return ReadEntityHashStat(r, entity, bannerStagesHash)
}

func ReadCurrentBossPosition(r Reader, entity uint64) (x, y float32, ok bool) {
	comp := ReadU64(r, entity+actorRefTableOff)
	if comp < HeapLo || comp >= HeapHi {
		return 0, 0, false
	}
	entry, found := hashEntry(r, comp, currentBossHashKey)
	if !found {
		return 0, 0, false
	}
	vbegin := ReadU64(r, entry+hashEntryValueOff)
	vend := ReadU64(r, entry+hashEntryValueOff+8)
	if vbegin < HeapLo || vend < vbegin {
		return 0, 0, false
	}
	for e := vbegin; e+refValueStride <= vend; e += refValueStride {
		if byte(ReadU32(r, e+refValueValidOff)) != 1 {
			continue
		}
		boss := ReadU64(r, e)
		if boss < HeapLo || boss >= HeapHi {
			continue
		}
		sub := ReadU64(r, boss+bossPosSubOff)
		if sub < HeapLo || sub >= HeapHi {
			continue
		}
		return ReadFloat32(r, sub+bossPosXOff), ReadFloat32(r, sub+bossPosXOff+4), true
	}
	return 0, 0, false
}

func readEntityRef(r Reader, entity uint64, key uint32) (uint64, bool) {
	entry, ok := hashEntry(r, entity+entityStatStoreOff, key)
	if !ok {
		return 0, false
	}
	vbegin := ReadU64(r, entry+hashEntryValueOff)
	if vbegin < HeapLo || vbegin >= HeapHi {
		return 0, false
	}
	if byte(ReadU32(r, vbegin+hashValueTypeOff)) != hashValueRefValid {
		return 0, false
	}
	ref := ReadU64(r, vbegin)
	if ref < HeapLo || ref >= HeapHi {
		return 0, false
	}
	return ref, true
}

// ReadKillOwner reads the effect-driven KillOwner attribute, set only when an
// on-kill effect resolves the killer; it is not general kill attribution.
func ReadKillOwner(r Reader, entity uint64) (uint64, bool) {
	return readEntityRef(r, entity, killOwnerHash)
}

func ReadMinionOwner(r Reader, entity uint64) (uint64, bool) {
	return readEntityRef(r, entity, minionOwnerHash)
}

func ReadEffectCaster(r Reader, entity uint64) (uint64, bool) {
	if caster, ok := readEntityRef(r, entity, effectCasterHash); ok {
		return caster, true
	}
	return readEntityRef(r, entity, curseCasterHash)
}

const (
	vaalSoulsStatKey = 0x4105
	rageStatKey      = 0x2B99
)

func ReadPlayerStatKey(r Reader, entity uint64, key uint32) (int32, bool) {
	stats := ResolveComponentByName(r, entity, "Stats")
	if stats == 0 {
		return 0, false
	}
	var val int32
	found := false
	walkStatsVec(r, stats, statsItemsPtrOff, func(k uint32, v int32) {
		if k == key {
			val, found = v, true
		}
	})
	return val, found
}

func ReadVaalSouls(r Reader, entity uint64) (int32, bool) {
	return ReadPlayerStatKey(r, entity, vaalSoulsStatKey)
}

func ReadRage(r Reader, entity uint64) (int32, bool) {
	return ReadPlayerStatKey(r, entity, rageStatKey)
}

const (
	entityEffectStoreOff = 0x58
	effectEntryStride    = 0x10
	effectObjOff         = 0x08

	shieldChargeHash = 0x0B6C8015
	arcticArmourKey  = 0xC301
	bloodyAuraKey    = 0xC698
)

func resolveEntityEffect(r Reader, entity uint64, key uint16) (uint64, bool) {
	begin := ReadU64(r, entity+entityEffectStoreOff)
	end := ReadU64(r, entity+entityEffectStoreOff+8)
	if begin < HeapLo || end < begin || (end-begin)%effectEntryStride != 0 {
		return 0, false
	}
	n := (end - begin) / effectEntryStride
	lo, hi := uint64(0), n
	for lo < hi {
		mid := (lo + hi) / 2
		if uint16(ReadU32(r, begin+mid*effectEntryStride)) < key {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo >= n || uint16(ReadU32(r, begin+lo*effectEntryStride)) != key {
		return 0, false
	}
	obj := ReadU64(r, begin+lo*effectEntryStride+effectObjOff)
	if obj < HeapLo || obj >= HeapHi || ReadU32(r, obj+8)&1 != 0 {
		return 0, false
	}
	return obj, true
}

func ReadShieldChargeIntensity(r Reader, entity uint64) (float32, bool) {
	return ReadEntityHashStat(r, entity, shieldChargeHash)
}

func ReadArcticArmourIntensity(r Reader, entity uint64) (float32, bool) {
	obj, ok := resolveEntityEffect(r, entity, arcticArmourKey)
	if !ok {
		return 0, false
	}
	return ReadFloat32(r, obj+0x78), true
}

func ReadBloodyAuraIntensity(r Reader, entity uint64) (float32, bool) {
	obj, ok := resolveEntityEffect(r, entity, bloodyAuraKey)
	if !ok {
		return 0, false
	}
	return ReadFloat32(r, obj+0x18), true
}
