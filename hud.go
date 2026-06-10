package gamestate

import (
	"errors"
	"math"
)

const (
	hudRootMapParentSlot    uint64 = 0x7C8
	hudRootSizeOff          uint64 = 0x288
	hudRootHeightOff        uint64 = 0x28C
	mapParentMiniMapPtrOff  uint64 = 0x388
	mapParentLargeMapPtrOff uint64 = 0x380
	mapViewWidthOff         uint64 = 0x288
	mapViewHeightOff        uint64 = 0x28C

	mapViewShiftXOff uint64 = 0x368
	mapViewShiftYOff uint64 = 0x36C
	mapViewZoomOff   uint64 = 0x3A8
)

type HUDChain struct {
	HUDRoot   uint64
	MapParent uint64
	MiniMap   uint64
	LargeMap  uint64

	DesignW, DesignH float32

	MiniW, MiniH float32

	MiniShiftX, MiniShiftY float32

	MiniZoom float32
}

func IsUiElementVisible(r Reader, widget uint64) bool {
	flags := ReadU32(r, widget+0x180)
	return flags&(1<<0x0B) != 0
}

const (
	MapShiftXOff        = 0x368
	MapShiftYOff        = 0x36C
	MapDefaultShiftYOff = 0x374
	MapZoomOff          = 0x3A8
)

func ElementVisibleHierarchical(r Reader, elem uint64) bool {
	cur := elem
	for range uiRootMaxParentHops {
		if !IsUiElementVisible(r, cur) {
			return false
		}
		parent := ReadU64(r, cur+ElementParentOff)
		if parent < HeapLo || parent >= HeapHi {
			return true
		}
		cur = parent
	}
	return true
}

func ResolveHUDChainFromIGS(r Reader, gsoSlot uint64) (HUDChain, error) {
	root, err := ResolveUiRoot(r, gsoSlot)
	if err != nil {
		return HUDChain{}, err
	}
	return ResolveHUDChain(r, root)
}

func ResolveHUDChain(r Reader, hudRoot uint64) (HUDChain, error) {
	var c HUDChain
	if hudRoot < HeapLo || hudRoot >= HeapHi {
		return c, errors.New("hudRoot not a heap pointer")
	}
	c.HUDRoot = hudRoot
	mp := ReadU64(r, hudRoot+hudRootMapParentSlot)
	if mp < HeapLo || mp >= HeapHi {
		return c, errors.New("MapParent slot at HUD root +0x7C8 not a heap ptr")
	}
	c.MapParent = mp
	c.LargeMap = ReadU64(r, mp+mapParentLargeMapPtrOff)
	c.MiniMap = ReadU64(r, mp+mapParentMiniMapPtrOff)
	c.DesignW = ReadFloat32(r, hudRoot+hudRootSizeOff)
	c.DesignH = ReadFloat32(r, hudRoot+hudRootHeightOff)
	if c.MiniMap >= HeapLo && c.MiniMap < HeapHi {
		c.MiniW = ReadFloat32(r, c.MiniMap+mapViewWidthOff)
		c.MiniH = ReadFloat32(r, c.MiniMap+mapViewHeightOff)
		c.MiniShiftX = ReadFloat32(r, c.MiniMap+mapViewShiftXOff)
		c.MiniShiftY = ReadFloat32(r, c.MiniMap+mapViewShiftYOff)
		c.MiniZoom = ReadFloat32(r, c.MiniMap+mapViewZoomOff)
	}
	if !plausibleDesign(c.DesignW) || !plausibleDesign(c.DesignH) {
		return c, errors.New("HUD root design canvas size implausible — wrong vtable?")
	}
	return c, nil
}

func plausibleDesign(f float32) bool {
	if math.IsNaN(float64(f)) || math.IsInf(float64(f), 0) {
		return false
	}
	return f >= 256 && f <= 16384
}

func (c HUDChain) MiniMapPixelRect(winW, winH int, marginDesignRight, marginDesignTop float32) (cx, cy, radius int) {
	if c.DesignW == 0 || c.DesignH == 0 || c.MiniW == 0 {
		return winW / 2, winH / 2, 80
	}
	scaleX := float32(winW) / c.DesignW
	scaleY := float32(winH) / c.DesignH
	pxMiniW := c.MiniW * scaleX
	pxMiniH := c.MiniH * scaleY
	rightEdgePx := float32(winW) - marginDesignRight*scaleX
	topEdgePx := marginDesignTop * scaleY
	cx = int(rightEdgePx - pxMiniW/2)
	cy = int(topEdgePx + pxMiniH/2)

	cx += int(c.MiniShiftX * scaleX)
	cy += int(c.MiniShiftY * scaleY)
	radius = int(pxMiniW / 2)
	return
}
