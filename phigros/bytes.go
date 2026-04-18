package phigros

import "encoding/binary"

type Bytes struct {
	Data []byte
	ptr  int
	bit  int
}

func NewBytesReader(b []byte) *Bytes {
	return &Bytes{Data: b}
}

func (b *Bytes) Alignment() {
	if b.bit > 0 {
		b.bit = 0
		b.ptr++
	}
}

// VarShort is a 7-bit little-endian integer encoded in one or two bytes.
func (b *Bytes) ReadVarShort() uint16 {
	b.Alignment()
	num := uint16(b.Data[b.ptr])
	if num < 128 {
		b.ptr++
		return num
	}
	num = uint16(b.Data[b.ptr]&0x7F) | uint16(b.Data[b.ptr+1])<<7
	b.ptr += 2
	return num
}

func (b *Bytes) ReadShort() int16 {
	b.Alignment()
	value := int16(binary.LittleEndian.Uint16(b.Data[b.ptr:]))
	b.ptr += 2
	return value
}

func (b *Bytes) ReadByte1() byte {
	b.Alignment()
	value := b.Data[b.ptr]
	b.ptr++
	return value
}

func (b *Bytes) ReadBool() bool {
	if b.bit >= 8 {
		b.bit = 0
		b.ptr++
	}
	value := GetBool(b.Data[b.ptr], b.bit)
	b.bit++
	return value
}

func (b *Bytes) ReadString() string {
	b.Alignment()
	length := int(b.ReadVarShort())
	start := b.ptr
	b.ptr += length
	return BytesToString(b.Data[start:b.ptr])
}

func (b *Bytes) ReadInt32() int32 {
	b.Alignment()
	value := int32(binary.LittleEndian.Uint32(b.Data[b.ptr:]))
	b.ptr += 4
	return value
}

func (b *Bytes) ReadFloat32() float32 {
	b.Alignment()
	value := ByteToFloat32(b.Data[b.ptr : b.ptr+4])
	b.ptr += 4
	return value
}

func GetBool(num byte, index int) bool {
	return (num>>index)&1 == 1
}
