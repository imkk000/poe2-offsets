package gamestate

import (
	"encoding/binary"
	"errors"
	"math"
)

const CameraWorldToScreenOff = 0x1A0

func ReadCameraMatrix(r Reader, gsoSlot uint64) ([16]float32, error) {
	var m [16]float32
	cam, err := ResolveCamera(r, gsoSlot)
	if err != nil {
		return m, err
	}
	buf, err := r.ReadBytes(cam+CameraWorldToScreenOff, 64)
	if err != nil || len(buf) < 64 {
		return m, errors.New("camera matrix short read")
	}
	for i := range 16 {
		m[i] = math.Float32frombits(binary.LittleEndian.Uint32(buf[i*4 : i*4+4]))
	}
	return m, nil
}

func ProjectWorldToScreen(m [16]float32, wx, wy, wz float32, winW, winH int) (float32, float32, bool) {
	cw := wx*m[3] + wy*m[7] + wz*m[11] + m[15]
	if cw <= 0.0001 {
		return 0, 0, false
	}
	cx := wx*m[0] + wy*m[4] + wz*m[8] + m[12]
	cy := wx*m[1] + wy*m[5] + wz*m[9] + m[13]
	sx := (cx/cw/2 + 0.5) * float32(winW)
	sy := (0.5 - cy/cw/2) * float32(winH)
	return sx, sy, true
}

func ReadEntityWorldXYZ(r Reader, entity uint64) (x, y, z float32, ok bool) {
	comp := ResolveComponentByName(r, entity, "Render")
	if comp == 0 {
		return 0, 0, 0, false
	}
	x = ReadFloat32(r, comp+renderPosXOff)
	y = ReadFloat32(r, comp+renderPosYOff)
	z = ReadFloat32(r, comp+renderPosYOff+4)
	if !plausibleWorldCoord(x) || !plausibleWorldCoord(y) {
		return 0, 0, 0, false
	}

	if math.IsNaN(float64(z)) || math.IsInf(float64(z), 0) {
		z = 0
	}
	return x, y, z, true
}
