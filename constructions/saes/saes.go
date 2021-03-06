// A reference-style implementation of AES.  It's useful for seeing the ways we can permute AES' internals without
// affecting its output.
package saes

import (
	"github.com/OpenWhiteBox/AES/primitives/matrix"
	"github.com/OpenWhiteBox/AES/primitives/number"
)

// Powers of x mod M(x).
var powx = [16]byte{0x01, 0x02, 0x04, 0x08, 0x10, 0x20, 0x40, 0x80, 0x1b, 0x36, 0x6c, 0xd8, 0xab, 0x4d, 0x9a, 0x2f}

type Construction struct {
	Key []byte
}

func (constr Construction) BlockSize() int { return 16 }

func (constr Construction) Encrypt(dst, src []byte) {
	roundKeys := constr.StretchedKey()
	copy(dst, src)

	constr.AddRoundKey(roundKeys[0], dst)
	for i := 1; i <= 9; i++ {
		constr.SubBytes(dst)
		constr.ShiftRows(dst)
		constr.MixColumns(dst)
		constr.AddRoundKey(roundKeys[i], dst)
	}

	constr.SubBytes(dst)
	constr.ShiftRows(dst)
	constr.AddRoundKey(roundKeys[10], dst)
}

func (constr Construction) Decrypt(dst, src []byte) {
	roundKeys := constr.StretchedKey()
	copy(dst, src)

	constr.AddRoundKey(roundKeys[10], dst)
	constr.UnShiftRows(dst)
	constr.UnSubBytes(dst)

	for i := 9; i > 0; i-- {
		constr.AddRoundKey(roundKeys[i], dst)
		constr.UnMixColumns(dst)
		constr.UnShiftRows(dst)
		constr.UnSubBytes(dst)
	}

	constr.AddRoundKey(roundKeys[0], dst)
}

func rotw(w uint32) uint32 { return w<<8 | w>>24 }

func (constr *Construction) StretchedKey() [11][]byte {
	var (
		i         int            = 0
		temp      uint32         = 0
		stretched [4 * 11]uint32 // Stretched key
		split     [11][]byte     // Each round key is combined and its uint32s are turned into 4 bytes
	)

	for ; i < 4; i++ { // First key-length of stretched is the raw key.
		stretched[i] = (uint32(constr.Key[4*i]) << 24) |
			(uint32(constr.Key[4*i+1]) << 16) |
			(uint32(constr.Key[4*i+2]) << 8) |
			uint32(constr.Key[4*i+3])
	}

	for ; i < (4 * 11); i++ {
		temp = stretched[i-1]

		if (i % 4) == 0 {
			temp = constr.SubWord(rotw(temp)) ^ (uint32(powx[i/4-1]) << 24)
		}

		stretched[i] = stretched[i-4] ^ temp
	}

	for j := 0; j < 11; j++ {
		split[j] = make([]byte, 16)

		for k := 0; k < 4; k++ {
			word := stretched[4*j+k]

			split[j][4*k] = byte(word >> 24)
			split[j][4*k+1] = byte(word >> 16)
			split[j][4*k+2] = byte(word >> 8)
			split[j][4*k+3] = byte(word)
		}
	}

	return split
}

func (constr *Construction) AddRoundKey(roundKey, block []byte) {
	for i, _ := range block {
		block[i] = roundKey[i] ^ block[i]
	}
}

func (constr *Construction) SubBytes(block []byte) {
	for i, _ := range block {
		block[i] = constr.SubByte(block[i])
	}
}

func (constr *Construction) UnSubBytes(block []byte) {
	for i, _ := range block {
		block[i] = constr.UnSubByte(block[i])
	}
}

func (constr *Construction) SubWord(w uint32) uint32 {
	return (uint32(constr.SubByte(byte(w>>24))) << 24) |
		(uint32(constr.SubByte(byte(w>>16))) << 16) |
		(uint32(constr.SubByte(byte(w>>8))) << 8) |
		uint32(constr.SubByte(byte(w)))
}

func (constr *Construction) SubByte(e byte) byte {
	// AES S-Box
	m := matrix.Matrix{ // Linear component.
		matrix.Row{0xF1}, // 0b11110001
		matrix.Row{0xE3}, // 0b11100011
		matrix.Row{0xC7}, // 0b11000111
		matrix.Row{0x8F}, // 0b10001111
		matrix.Row{0x1F}, // 0b00011111
		matrix.Row{0x3E}, // 0b00111110
		matrix.Row{0x7C}, // 0b01111100
		matrix.Row{0xF8}, // 0b11111000
	}
	a := byte(0x63) // 0b01100011 - Affine component.

	return m.Mul(matrix.Row{byte(number.ByteFieldElem(e).Invert())})[0] ^ a
}

func (constr *Construction) UnSubByte(e byte) byte {
	// AES Inverse S-Box
	m := matrix.Matrix{
		matrix.Row{0xA4},
		matrix.Row{0x49},
		matrix.Row{0x92},
		matrix.Row{0x25},
		matrix.Row{0x4a},
		matrix.Row{0x94},
		matrix.Row{0x29},
		matrix.Row{0x52},
	}
	a := byte(0x63)

	invVal := m.Mul(matrix.Row{e ^ a})[0]
	return byte(number.ByteFieldElem(invVal).Invert())
}

func (constr *Construction) ShiftRows(block []byte) {
	copy(block, []byte{
		block[0], block[5], block[10], block[15], block[4], block[9], block[14], block[3], block[8], block[13], block[2],
		block[7], block[12], block[1], block[6], block[11],
	})
}

func (constr *Construction) UnShiftRows(block []byte) {
	copy(block, []byte{
		block[0], block[13], block[10], block[7], block[4], block[1], block[14], block[11], block[8], block[5], block[2],
		block[15], block[12], block[9], block[6], block[3],
	})
}

func (constr *Construction) MixColumns(block []byte) {
	for i := 0; i < 16; i += 4 {
		constr.MixColumn(block[i : i+4])
	}
}

func (constr *Construction) UnMixColumns(block []byte) {
	for i := 0; i < 16; i += 4 {
		constr.UnMixColumn(block[i : i+4])
	}
}

func (constr *Construction) MixColumn(slice []byte) {
	column := number.ArrayFieldElem{
		number.ByteFieldElem(slice[0]), number.ByteFieldElem(slice[1]),
		number.ByteFieldElem(slice[2]), number.ByteFieldElem(slice[3]),
	}.Mul(number.ArrayFieldElem{
		number.ByteFieldElem(0x02), number.ByteFieldElem(0x01),
		number.ByteFieldElem(0x01), number.ByteFieldElem(0x03),
	})

	for i := 0; i < 4; i++ {
		if len(column) > i {
			slice[i] = byte(column[i])
		} else {
			slice[i] = 0x00
		}
	}
}

func (constr *Construction) UnMixColumn(slice []byte) {
	column := number.ArrayFieldElem{
		number.ByteFieldElem(slice[0]), number.ByteFieldElem(slice[1]),
		number.ByteFieldElem(slice[2]), number.ByteFieldElem(slice[3]),
	}.Mul(number.ArrayFieldElem{
		number.ByteFieldElem(0x0e), number.ByteFieldElem(0x09),
		number.ByteFieldElem(0x0d), number.ByteFieldElem(0x0b),
	})

	for i := 0; i < 4; i++ {
		if len(column) > i {
			slice[i] = byte(column[i])
		} else {
			slice[i] = 0x00
		}
	}
}
