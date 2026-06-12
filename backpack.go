package gamestate

import (
	"bytes"
	"encoding/binary"
	"strings"
)

func ItemRarity(r Reader, itemEntity uint64, path string) string {
	mods := ResolveComponentByName(r, itemEntity, "Mods")
	if mods != 0 {
		idx := ReadU32(r, mods+0x94)
		if int(idx) < len(rarityNames) {
			return rarityNames[idx]
		}
	}
	switch {
	case strings.Contains(path, "/Currency/"):
		return "Currency"
	case strings.Contains(path, "/Gems/"):
		return "Gem"
	case strings.Contains(path, "/QuestItems/"):
		return "Quest"
	}
	return "Normal"
}

const InventoryItemViewVtable uint64 = 0x14323B6C0

const (
	inventoryItemViewContainerOff = 0xeb * 8
	backpackGridWidthOff          = 0x150
	backpackGridHeightOff         = 0x154
	backpackMapSentinelOff        = 0x188
	backpackMapSizeOff            = 0x190
)

type HeapRegion struct {
	Start uint64
	Size  uint64
}

type BackpackItem struct {
	ID       uint32   `json:"id"`
	GridX    int      `json:"x"`
	GridY    int      `json:"y"`
	Width    int      `json:"w"`
	Height   int      `json:"h"`
	Path     string   `json:"path"`
	Name     string   `json:"name"`
	Rarity   string   `json:"rarity"`
	ModTexts []string `json:"mod_texts,omitempty"`
}

type Backpack struct {
	ContainerAddr uint64         `json:"container_addr"`
	Width         int            `json:"width"`
	Height        int            `json:"height"`
	Filled        int            `json:"filled"`
	Total         int            `json:"total"`
	Items         []BackpackItem `json:"items"`
	Mask          []uint64       `json:"-"`
	Rows          []string       `json:"rows"`
}

func FindBackpackContainer(r Reader, regions []HeapRegion) (uint64, bool) {
	needle := make([]byte, 8)
	binary.LittleEndian.PutUint64(needle, InventoryItemViewVtable)
	counts := make(map[uint64]int)
	const chunkSize = 1 << 20
	for _, reg := range regions {
		for off := uint64(0); off < reg.Size; off += chunkSize {
			n := min(chunkSize, reg.Size-off)
			data, err := r.ReadBytes(reg.Start+off, int(n))
			if err != nil || len(data) < 8 {
				continue
			}
			idx := 0
			for {
				i := bytes.Index(data[idx:], needle)
				if i < 0 {
					break
				}
				abs := reg.Start + off + uint64(idx+i)
				idx += i + 1
				if abs&7 != 0 {
					continue
				}
				container := ReadU64(r, abs+inventoryItemViewContainerOff)
				if container < HeapLo || container >= HeapHi {
					continue
				}
				counts[container]++
			}
		}
	}
	var best uint64
	bestCount := 0
	for c, n := range counts {
		w := int(ReadU32(r, c+backpackGridWidthOff))
		h := int(ReadU32(r, c+backpackGridHeightOff))

		if w != backpackExactWidth || h != backpackExactHeight {
			continue
		}
		if n > bestCount {
			best = c
			bestCount = n
		}
	}
	if best == 0 {
		return 0, false
	}
	return best, true
}

const (
	backpackExactWidth  = 12
	backpackExactHeight = 5
)

func FindBackpackContainerByShape(r Reader, regions []HeapRegion) (uint64, bool) {
	type candidate struct {
		addr  uint64
		items int
		cells int
	}
	var cands []candidate
	const chunkSize = 1 << 20
	const minShapeOffset = 0x200
	for _, reg := range regions {
		for off := uint64(0); off < reg.Size; off += chunkSize {
			n := min(chunkSize, reg.Size-off)
			data, err := r.ReadBytes(reg.Start+off, int(n))
			if err != nil || len(data) < minShapeOffset {
				continue
			}
			limit := len(data) - minShapeOffset
			for pos := 0; pos <= limit; pos += 8 {
				w := binary.LittleEndian.Uint32(data[pos+0x150:])
				if w != backpackExactWidth {
					continue
				}
				h := binary.LittleEndian.Uint32(data[pos+0x154:])
				if h != backpackExactHeight {
					continue
				}
				cells := int(w) * int(h)
				sentinel := binary.LittleEndian.Uint64(data[pos+0x188:])
				if sentinel < HeapLo || sentinel >= HeapHi {
					continue
				}
				if ReadByte(r, sentinel+0x19) != 1 {
					continue
				}
				addr := reg.Start + off + uint64(pos)
				items := countInventoryItems(r, sentinel)
				if items == 0 {
					continue
				}
				cands = append(cands, candidate{addr: addr, items: items, cells: cells})
			}
		}
	}
	if len(cands) == 0 {
		return 0, false
	}
	best := cands[0]
	for _, c := range cands[1:] {
		if c.items > best.items {
			best = c
		}
	}
	return best.addr, true
}

func countInventoryItems(r Reader, sentinel uint64) int {
	count := 0
	walkInventoryMap(r, sentinel, func(node uint64) {
		valuePtr := ReadU64(r, node+0x28)
		if valuePtr < HeapLo || valuePtr >= HeapHi {
			return
		}
		itemPtr := ReadU64(r, valuePtr)
		if itemPtr < HeapLo || itemPtr >= HeapHi {
			return
		}
		x := int(ReadU32(r, valuePtr+8))
		y := int(ReadU32(r, valuePtr+12))
		xEnd := int(ReadU32(r, valuePtr+16))
		yEnd := int(ReadU32(r, valuePtr+20))
		if xEnd <= x || yEnd <= y {
			return
		}
		if xEnd-x > 32 || yEnd-y > 32 {
			return
		}
		count++
	})
	return count
}

func ReadBackpack(r Reader, container uint64) (Backpack, bool) {
	return readBackpack(r, container, false)
}

func ReadBackpackWithMods(r Reader, container uint64) (Backpack, bool) {
	return readBackpack(r, container, true)
}

func readBackpack(r Reader, container uint64, withMods bool) (Backpack, bool) {
	var bp Backpack
	if container < HeapLo || container >= HeapHi {
		return bp, false
	}
	bp.ContainerAddr = container
	bp.Width = int(ReadU32(r, container+backpackGridWidthOff))
	bp.Height = int(ReadU32(r, container+backpackGridHeightOff))
	if bp.Width <= 0 || bp.Width > 32 || bp.Height <= 0 || bp.Height > 32 {
		return bp, false
	}
	bp.Total = bp.Width * bp.Height
	bp.Mask = make([]uint64, (bp.Total+63)/64)

	sentinel := ReadU64(r, container+backpackMapSentinelOff)
	if sentinel < HeapLo || sentinel >= HeapHi {
		return bp, false
	}

	walkInventoryMap(r, sentinel, func(node uint64) {
		id := ReadU32(r, node+0x20)
		valuePtr := ReadU64(r, node+0x28)
		if valuePtr < HeapLo || valuePtr >= HeapHi {
			return
		}

		itemPtr := ReadU64(r, valuePtr)
		x := int(ReadU32(r, valuePtr+8))
		y := int(ReadU32(r, valuePtr+12))
		xEnd := int(ReadU32(r, valuePtr+16))
		yEnd := int(ReadU32(r, valuePtr+20))
		w := xEnd - x
		h := yEnd - y
		if w <= 0 || h <= 0 || w > bp.Width || h > bp.Height {
			return
		}
		var path, name, rarity string
		var modTexts []string
		if itemPtr >= HeapLo && itemPtr < HeapHi {
			details := ReadU64(r, itemPtr+0x08)
			if details >= HeapLo && details < HeapHi {
				path = readPathString(r, ReadU64(r, details+0x08))
				name = BaseItemName(path)
			}
			rarity = ItemRarity(r, itemPtr, path)
			if withMods {

				if mc := ResolveComponentByName(r, itemPtr, "Mods"); mc != 0 {
					for _, e := range ReadItemModEntries(r, mc) {
						modTexts = append(modTexts, e.ID)
					}
					texts, _ := ReadItemMods(r, mc, rarity)
					modTexts = append(modTexts, texts...)
				}
			}
		}
		bp.Items = append(bp.Items, BackpackItem{
			ID: id, GridX: x, GridY: y, Width: w, Height: h,
			Path: path, Name: name, Rarity: rarity, ModTexts: modTexts,
		})
		bp.Filled += w * h
		for yy := y; yy < yEnd && yy < bp.Height; yy++ {
			for xx := x; xx < xEnd && xx < bp.Width; xx++ {
				bit := uint(yy*bp.Width + xx)
				bp.Mask[bit/64] |= 1 << (bit % 64)
			}
		}
	})
	bp.Rows = bp.RenderBinary()
	return bp, true
}

func (b *Backpack) IsCellOccupied(col, row int) bool {
	if col < 0 || col >= b.Width || row < 0 || row >= b.Height {
		return false
	}
	bit := uint(row*b.Width + col)
	return b.Mask[bit/64]&(1<<(bit%64)) != 0
}

func (b *Backpack) RenderBinary() []string {
	if b.Width == 0 || b.Height == 0 {
		return nil
	}
	out := make([]string, b.Height)
	buf := make([]byte, b.Width)
	for row := range b.Height {
		for col := range b.Width {
			if b.IsCellOccupied(col, row) {
				buf[col] = '1'
			} else {
				buf[col] = '0'
			}
		}
		out[row] = string(buf)
	}
	return out
}

func WalkInventoryMap(r Reader, sentinel uint64, visit func(node uint64)) {
	walkInventoryMap(r, sentinel, visit)
}

func walkInventoryMap(r Reader, sentinel uint64, visit func(node uint64)) {
	root := ReadU64(r, sentinel+8)
	if root < HeapLo || root >= HeapHi {
		return
	}
	isReal := func(n uint64) bool {
		return n >= HeapLo && n < HeapHi && n != sentinel && ReadByte(r, n+0x19) == 0
	}
	leftmost := func(n uint64) uint64 {
		for {
			l := ReadU64(r, n)
			if !isReal(l) {
				return n
			}
			n = l
		}
	}
	node := leftmost(root)
	seen := make(map[uint64]bool)
	for isReal(node) && len(seen) < 2000 {
		if seen[node] {
			return
		}
		seen[node] = true
		visit(node)
		right := ReadU64(r, node+0x10)
		if isReal(right) {
			node = leftmost(right)
			continue
		}
		for {
			parent := ReadU64(r, node+8)
			if parent < HeapLo || parent >= HeapHi || parent == sentinel {
				return
			}
			if ReadU64(r, parent+0x10) == node {
				node = parent
				continue
			}
			node = parent
			break
		}
	}
}
