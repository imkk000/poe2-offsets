package gamestate

import (
	"bytes"
	"encoding/binary"
	"runtime"
	"sync"
	"sync/atomic"
)

const HoverTrackerVtable uint64 = 0x142D71818

const (
	hoverTrackerViewOff    = 0x50
	hoverViewItemEntityOff = 0x4F8
	hoverTrackerEntityOff  = 0x18

	uiRootHoverHostOff   = 0x7D8
	worldHoverTrackerOff = 0x630
)

func ResolveWorldHoverTracker(r Reader, gsoSlot uint64) uint64 {
	ui, err := ResolveUiRoot(r, gsoSlot)
	if err != nil || ui == 0 {
		return 0
	}
	host := ReadU64(r, ui+uiRootHoverHostOff)
	if host < HeapLo || host >= HeapHi {
		return 0
	}
	tr := host + worldHoverTrackerOff
	if ReadU64(r, tr) != HoverTrackerVtable {
		return 0
	}
	return tr
}

func FindInventoryHoverTracker(r Reader, regions []HeapRegion) uint64 {
	views := scanQwordHits(r, regions, InventoryItemViewVtable)
	if len(views) == 0 {
		return 0
	}
	viewSet := make(map[uint64]struct{}, len(views))
	for _, v := range views {
		viewSet[v] = struct{}{}
	}
	const chunkSize = 1 << 20
	for _, reg := range regions {
		for off := uint64(0); off < reg.Size; off += chunkSize {
			n := min(uint64(chunkSize), reg.Size-off)
			data, err := r.ReadBytes(reg.Start+off, int(n))
			if err != nil || len(data) < 8 {
				continue
			}
			for i := 0; i+8 <= len(data); i += 8 {
				v := binary.LittleEndian.Uint64(data[i : i+8])
				if _, ok := viewSet[v]; !ok {
					continue
				}
				loc := reg.Start + off + uint64(i)
				if loc < hoverTrackerViewOff {
					continue
				}
				base := loc - hoverTrackerViewOff
				if ReadU64(r, base) == HoverTrackerVtable {
					return base
				}
			}
		}
	}
	return 0
}

func ReadHoveredInventoryItem(r Reader, tracker uint64) uint64 {
	if tracker == 0 {
		return 0
	}
	view := ReadU64(r, tracker+hoverTrackerViewOff)
	if view < HeapLo || view >= HeapHi {
		return 0
	}
	item := ReadU64(r, view+hoverViewItemEntityOff)
	if item < HeapLo || item >= HeapHi {
		return 0
	}
	return item
}

func FindWorldHoverTracker(r Reader, regions []HeapRegion, entitySet map[uint64]struct{}) uint64 {
	if len(entitySet) == 0 {
		return 0
	}
	const chunkSize = 1 << 20
	for _, reg := range regions {
		for off := uint64(0); off < reg.Size; off += chunkSize {
			n := min(uint64(chunkSize), reg.Size-off)
			data, err := r.ReadBytes(reg.Start+off, int(n))
			if err != nil || len(data) < 8 {
				continue
			}
			for i := 0; i+8 <= len(data); i += 8 {
				v := binary.LittleEndian.Uint64(data[i : i+8])
				if _, ok := entitySet[v]; !ok {
					continue
				}
				loc := reg.Start + off + uint64(i)
				if loc < hoverTrackerEntityOff {
					continue
				}
				base := loc - hoverTrackerEntityOff
				if ReadU64(r, base) == HoverTrackerVtable {
					return base
				}
			}
		}
	}
	return 0
}

func ReadHoveredWorldEntity(r Reader, tracker uint64) uint64 {
	if tracker == 0 {
		return 0
	}
	ent := ReadU64(r, tracker+hoverTrackerEntityOff)
	if ent < HeapLo || ent >= HeapHi {
		return 0
	}
	return ent
}

func scanQwordHits(r Reader, regions []HeapRegion, needle uint64) []uint64 {
	pat := make([]byte, 8)
	binary.LittleEndian.PutUint64(pat, needle)
	const chunkSize = 1 << 20

	type job struct {
		start uint64
		size  int
	}
	var jobs []job
	for _, reg := range regions {
		for off := uint64(0); off < reg.Size; off += chunkSize {
			n := chunkSize
			if reg.Size-off < uint64(n) {
				n = int(reg.Size - off)
			}
			jobs = append(jobs, job{reg.Start + off, n})
		}
	}

	workers := min(max(runtime.NumCPU(), 1), 12)
	results := make([][]uint64, workers)
	var next atomic.Int64
	var wg sync.WaitGroup
	for w := range workers {
		wg.Add(1)
		go func(wi int) {
			defer wg.Done()
			var local []uint64
			for {
				ji := int(next.Add(1)) - 1
				if ji >= len(jobs) {
					break
				}
				jb := jobs[ji]
				data, err := r.ReadBytes(jb.start, jb.size)
				if err != nil || len(data) < 8 {
					continue
				}
				idx := 0
				for {
					i := bytes.Index(data[idx:], pat)
					if i < 0 {
						break
					}
					abs := jb.start + uint64(idx+i)
					idx += i + 1
					if abs&7 != 0 {
						continue
					}
					local = append(local, abs)
				}
			}
			results[wi] = local
		}(w)
	}
	wg.Wait()

	var out []uint64
	for _, rr := range results {
		out = append(out, rr...)
	}
	return out
}
