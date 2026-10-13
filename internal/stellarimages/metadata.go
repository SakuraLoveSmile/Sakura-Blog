package stellarimages

import (
	"bytes"
	"encoding/binary"
)

func orientationFromContainers(data []byte) int {
	if bytes.HasPrefix(data, []byte("RIFF")) {
		for p := 12; p+8 <= len(data); {
			n := int(binary.LittleEndian.Uint32(data[p+4:]))
			if n > len(data)-p-8 {
				break
			}
			if string(data[p:p+4]) == "EXIF" {
				exif := data[p+8 : p+8+n]
				exif = bytes.TrimPrefix(exif, []byte("Exif\x00\x00"))
				return tiffOrientation(exif)
			}
			p += 8 + n + (n % 2)
		}
	}
	if len(data) >= 4 && (string(data[:2]) == "II" || string(data[:2]) == "MM") {
		return tiffOrientation(data)
	}
	if len(data) > 2 && data[0] == 0xff && data[1] == 0xd8 {
		for p := 2; p+4 <= len(data); {
			if data[p] != 0xff {
				break
			}
			marker := data[p+1]
			p += 2
			if marker == 0xda || marker == 0xd9 {
				break
			}
			if marker == 0x00 || marker == 0xff || marker >= 0xd0 && marker <= 0xd7 {
				continue
			}
			n := int(binary.BigEndian.Uint16(data[p:]))
			if n < 2 || p+n > len(data) {
				break
			}
			if marker == 0xe1 && n >= 8 && bytes.Equal(data[p+2:p+8], []byte("Exif\x00\x00")) {
				return tiffOrientation(data[p+8 : p+n])
			}
			p += n
		}
	}
	if bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")) {
		for p := 8; p+12 <= len(data); {
			n := int(binary.BigEndian.Uint32(data[p:]))
			if n > len(data)-p-12 {
				break
			}
			if string(data[p+4:p+8]) == "eXIf" {
				return tiffOrientation(data[p+8 : p+8+n])
			}
			p += 12 + n
		}
	}
	return 1
}
func tiffOrientation(data []byte) int {
	if len(data) < 8 {
		return 1
	}
	var order binary.ByteOrder
	switch string(data[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 1
	}
	if order.Uint16(data[2:]) != 42 {
		if order.Uint16(data[2:]) != 43 || len(data) < 16 || order.Uint16(data[4:]) != 8 {
			return 1
		}
		offset := order.Uint64(data[8:])
		if offset > uint64(len(data))-8 {
			return 1
		}
		count := order.Uint64(data[offset:])
		for i := uint64(0); i < count; i++ {
			if i > (uint64(len(data))-offset-8)/20 {
				return 1
			}
			p := offset + 8 + i*20
			if p+20 > uint64(len(data)) {
				return 1
			}
			if order.Uint16(data[p:]) == 0x112 && order.Uint16(data[p+2:]) == 3 && order.Uint64(data[p+4:]) == 1 {
				v := int(order.Uint16(data[p+12:]))
				if v >= 1 && v <= 8 {
					return v
				}
			}
		}
		return 1
	}
	offset := uint64(order.Uint32(data[4:]))
	if offset+2 > uint64(len(data)) {
		return 1
	}
	count := uint64(order.Uint16(data[offset:]))
	for i := uint64(0); i < count; i++ {
		p := offset + 2 + i*12
		if p+12 > uint64(len(data)) {
			return 1
		}
		if order.Uint16(data[p:]) == 0x112 && order.Uint16(data[p+2:]) == 3 && order.Uint32(data[p+4:]) == 1 {
			v := int(order.Uint16(data[p+8:]))
			if v >= 1 && v <= 8 {
				return v
			}
		}
	}
	return 1
}
