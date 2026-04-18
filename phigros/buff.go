package phigros

import (
	"bytes"
	"encoding/binary"
)

type Buff struct {
	Bytes   bytes.Buffer
	bit     int
	tempbit byte
}

func (b *Buff) Alignment() {
	if b.bit > 0 {
		_ = b.Bytes.WriteByte(b.tempbit)
		b.bit = 0
		b.tempbit = 0
	}
}

func (b *Buff) SaveBool(bt bool) {
	if b.bit >= 8 {
		b.Alignment()
	}
	if bt {
		b.tempbit |= 1 << b.bit
	}
	b.bit++
}

func (b *Buff) SaveByte1(v byte) {
	b.Alignment()
	_ = b.Bytes.WriteByte(v)
}

func (b *Buff) SaveShort(v int16) {
	b.Alignment()
	var buf [2]byte
	binary.LittleEndian.PutUint16(buf[:], uint16(v))
	_, _ = b.Bytes.Write(buf[:])
}

func (b *Buff) SaveVarShort(v uint16) {
	b.Alignment()
	if v < 128 {
		_ = b.Bytes.WriteByte(byte(v))
		return
	}
	_, _ = b.Bytes.Write([]byte{byte(v&0x7F) | 0x80, byte(v >> 7)})
}

func (b *Buff) SaveInt32(v int32) {
	b.Alignment()
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], uint32(v))
	_, _ = b.Bytes.Write(buf[:])
}

func (b *Buff) SaveFloat32(v float32) {
	b.Alignment()
	_, _ = b.Bytes.Write(Float32ToByte(v))
}

func (b *Buff) SaveString(s string) {
	b.Alignment()
	b.SaveVarShort(uint16(len(s)))
	_, _ = b.Bytes.WriteString(s)
}

func (b *Buff) SaveBytes(data []byte) {
	b.Alignment()
	_, _ = b.Bytes.Write(data)
}
