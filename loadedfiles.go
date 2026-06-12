package gamestate

import "strings"

const (
	LoadedFilesRootRVA   = 0x462B098
	AreaChangeCounterRVA = 0x3926FE8

	loadedFilesBucketCount  = 16
	loadedFilesBucketStride = 0x38
	loadedFilesVecFirstOff  = 0x00
	loadedFilesVecLastOff   = 0x08
	loadedFilesBucketCapOff = 0x20
	loadedFilesNodeStride   = 0x18
	loadedFilesFilePtrOff   = 0x08
	loadedFilesNameOff      = 0x08
	loadedFilesAreaCountOff = 0x38
	loadedFilesMaxNodes     = 2_000_000
)

type LoadedFile struct {
	Path      string
	AreaCount uint32
}

func AreaChangeCounter(r Reader, moduleBase uint64) uint32 {
	return ReadU32(r, moduleBase+AreaChangeCounterRVA)
}

func ReadLoadedFiles(r Reader, moduleBase uint64) []LoadedFile {
	root := ReadU64(r, moduleBase+LoadedFilesRootRVA)
	if root < HeapLo || root >= HeapHi {
		return nil
	}
	var out []LoadedFile
	for i := range loadedFilesBucketCount {
		out = append(out, readLoadedFilesBucket(r, root+uint64(i)*loadedFilesBucketStride)...)
	}
	return out
}

func readLoadedFilesBucket(r Reader, bucket uint64) []LoadedFile {
	first := ReadU64(r, bucket+loadedFilesVecFirstOff)
	last := ReadU64(r, bucket+loadedFilesVecLastOff)
	if first < HeapLo || first >= HeapHi || last <= first {
		return nil
	}
	if ReadU32(r, bucket+loadedFilesBucketCapOff) == 0 {
		return nil
	}
	nodes := (last - first) / loadedFilesNodeStride
	if nodes > loadedFilesMaxNodes {
		return nil
	}
	out := make([]LoadedFile, 0, nodes)
	for j := range int(nodes) {
		node := first + uint64(j)*loadedFilesNodeStride
		if f, ok := readLoadedFile(r, ReadU64(r, node+loadedFilesFilePtrOff)); ok {
			out = append(out, f)
		}
	}
	return out
}

func readLoadedFile(r Reader, filePtr uint64) (LoadedFile, bool) {
	if filePtr < HeapLo || filePtr >= HeapHi {
		return LoadedFile{}, false
	}
	path := ReadStdWString(r, filePtr+loadedFilesNameOff)
	if path == "" {
		return LoadedFile{}, false
	}
	if at := strings.IndexByte(path, '@'); at >= 0 {
		path = path[:at]
	}
	return LoadedFile{Path: path, AreaCount: ReadU32(r, filePtr+loadedFilesAreaCountOff)}, true
}

func ReadCurrentAreaFiles(r Reader, moduleBase uint64) []string {
	ctr := AreaChangeCounter(r, moduleBase)
	var out []string
	for _, f := range ReadLoadedFiles(r, moduleBase) {
		if f.AreaCount == ctr {
			out = append(out, f.Path)
		}
	}
	return out
}
