package gamestate

import (
	"encoding/binary"
)

const (
	entityDetailsPtrOff      = 0x08
	detailsLookupPtrOff      = 0x28
	lookupBucketOff          = 0x28
	lookupBucketEntryStride  = 0x10
	lookupBucketMaxScanBytes = 64 * lookupBucketEntryStride
	lookupNameMaxBytes       = 96
)

func ResolveComponentByName(r Reader, entity uint64, name string) uint64 {
	if entity < HeapLo || entity >= HeapHi {
		return 0
	}
	details := ReadU64(r, entity+entityDetailsPtrOff)
	if details < HeapLo || details >= HeapHi {
		return 0
	}
	lookup := ReadU64(r, details+detailsLookupPtrOff)
	if lookup < HeapLo || lookup >= HeapHi {
		return 0
	}
	bucketBegin := ReadU64(r, lookup+lookupBucketOff)
	bucketEnd := ReadU64(r, lookup+lookupBucketOff+8)
	if !validDataPtr(bucketBegin) || bucketEnd <= bucketBegin {
		return 0
	}
	size := min(bucketEnd-bucketBegin, lookupBucketMaxScanBytes)
	if size%lookupBucketEntryStride != 0 {
		return 0
	}
	data, err := r.ReadBytes(bucketBegin, int(size))
	if err != nil || uint64(len(data)) < size {
		return 0
	}
	clBegin := ReadU64(r, entity+CompListBeginOff)
	clEnd := ReadU64(r, entity+CompListEndOff)
	if !validDataPtr(clBegin) || clEnd <= clBegin {
		return 0
	}
	clCount := (clEnd - clBegin) / 8
	for off := 0; off+lookupBucketEntryStride <= len(data); off += lookupBucketEntryStride {
		namePtr := binary.LittleEndian.Uint64(data[off : off+8])
		idx := binary.LittleEndian.Uint32(data[off+8 : off+12])
		if uint64(idx) >= clCount {
			continue
		}
		if !asciiEquals(r, namePtr, name) {
			continue
		}
		return ReadU64(r, clBegin+uint64(idx)*8)
	}
	return 0
}

func HasMinimapIcon(r Reader, entity uint64) bool {
	return ResolveComponentByName(r, entity, "MinimapIcon") != 0
}

func ListComponentNames(r Reader, entity uint64) []string {
	if entity < HeapLo || entity >= HeapHi {
		return nil
	}
	details := ReadU64(r, entity+entityDetailsPtrOff)
	if details < HeapLo || details >= HeapHi {
		return nil
	}
	lookup := ReadU64(r, details+detailsLookupPtrOff)
	if lookup < HeapLo || lookup >= HeapHi {
		return nil
	}
	bucketBegin := ReadU64(r, lookup+lookupBucketOff)
	bucketEnd := ReadU64(r, lookup+lookupBucketOff+8)
	if !validDataPtr(bucketBegin) || bucketEnd <= bucketBegin {
		return nil
	}
	size := min(bucketEnd-bucketBegin, lookupBucketMaxScanBytes)
	if size%lookupBucketEntryStride != 0 {
		return nil
	}
	data, err := r.ReadBytes(bucketBegin, int(size))
	if err != nil || uint64(len(data)) < size {
		return nil
	}
	var names []string
	for off := 0; off+lookupBucketEntryStride <= len(data); off += lookupBucketEntryStride {
		namePtr := binary.LittleEndian.Uint64(data[off : off+8])
		if name := readAsciiName(r, namePtr); name != "" {
			names = append(names, name)
		}
	}
	return names
}

func readAsciiName(r Reader, addr uint64) string {
	if addr < 0x140000000 || addr >= 0x150000000 {
		if addr < HeapLo || addr >= HeapHi {
			return ""
		}
	}
	buf, err := r.ReadBytes(addr, lookupNameMaxBytes)
	if err != nil || len(buf) == 0 {
		return ""
	}
	for i, b := range buf {
		if b == 0 {
			return string(buf[:i])
		}
		if b < 0x20 || b > 0x7E {
			return ""
		}
	}
	return ""
}

func asciiEquals(r Reader, addr uint64, want string) bool {
	if addr < 0x140000000 || addr >= 0x150000000 {

		if addr < HeapLo || addr >= HeapHi {
			return false
		}
	}
	buf, err := r.ReadBytes(addr, len(want)+1)
	if err != nil || len(buf) < len(want)+1 {
		return false
	}
	for i := range want {
		if buf[i] != want[i] {
			return false
		}
	}
	return buf[len(want)] == 0
}
