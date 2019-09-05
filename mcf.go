package main

import (
	"encoding/binary"
	"fmt"
	"math"
)

func floatToHex(f float32) string {
	bits := math.Float32bits(f)
	bytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(bytes, bits)
	return fmt.Sprintf("D%02X%02X%02X%02X", bytes[3], bytes[2], bytes[1], bytes[0])
}
