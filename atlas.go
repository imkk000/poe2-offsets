package gamestate

import "encoding/binary"

const (
	atlasStateInGameStateOff = 0x350
	atlasVertexOff           = 0x10
	atlasCurrentNodeOff      = 0x18
	atlasPrevNodeOff         = 0x20
)

type AtlasState struct {
	Node     uint32 `json:"node"`
	PrevNode uint32 `json:"prev_node,omitempty"`
	Vertex   uint32 `json:"vertex,omitempty"`
}

func ReadAtlasState(r Reader, inGameState uint64) (AtlasState, bool) {
	var s AtlasState
	if inGameState < HeapLo || inGameState >= HeapHi {
		return s, false
	}
	atlas := ReadU64(r, inGameState+atlasStateInGameStateOff)
	if atlas < HeapLo || atlas >= HeapHi {
		return s, false
	}
	buf, err := r.ReadBytes(atlas+atlasVertexOff, 24)
	if err != nil || len(buf) < 20 {
		return s, false
	}
	s.Vertex = binary.LittleEndian.Uint32(buf[0:4])
	s.Node = binary.LittleEndian.Uint32(buf[atlasCurrentNodeOff-atlasVertexOff : atlasCurrentNodeOff-atlasVertexOff+4])
	if len(buf) >= 20 {
		s.PrevNode = binary.LittleEndian.Uint32(buf[atlasPrevNodeOff-atlasVertexOff : atlasPrevNodeOff-atlasVertexOff+4])
	}
	return s, s.Node != 0
}
