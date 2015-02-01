package mmd

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"math"
)

type Buffer struct {
	bytes.Buffer
	tmp [8]byte // avoid allocating on int int conversions
}

func NewBuffer(sz int) *Buffer {
	return &Buffer{Buffer: *bytes.NewBuffer(make([]byte, 0, sz))}
}

func (b *Buffer) Clone() *Buffer {
	var ret Buffer
	ret = *b
	return &ret
}

func (b *Buffer) HexDump() string {
	l := b.Len()
	if l > 200 {
		l = 200
	}
	return hex.Dump(b.Bytes()[:l])
}

func (b *Buffer) Reset() {
	fmt.Println("Calling my reset")
	b.Buffer.Reset()
}
func (b *Buffer) RequireNext(n int) ([]byte, error) {
	if b.Len() < n {
		return nil, io.EOF
	}
	return b.Next(n), nil
}

func (b *Buffer) ReadString(n int) (string, error) {
	r, e := b.RequireNext(n)
	if e != nil {
		return "", e
	}
	return string(r), nil
}
func (b *Buffer) ReadVarint() (int, error) {
	i, err := binary.ReadVarint(b)
	return int(i), err
}
func (b *Buffer) ReadVaruint() (uint, error) {
	u, err := binary.ReadUvarint(b)
	return uint(u), err
}

/*
16 bit numbers
*/
func (b *Buffer) ReadUInt16() (uint16, error) {
	if b.Len() < 8 {
		return 0, io.EOF
	}
	return binary.BigEndian.Uint16(b.Next(2)), nil
}
func (b *Buffer) ReadInt16() (int16, error) {
	i, e := b.ReadUInt16()
	return int16(i), e
}
func (b *Buffer) WriteUInt16(i uint16) {
	binary.BigEndian.PutUint16(b.tmp[:2], i)
	b.Write(b.tmp[:2])
}
func (b *Buffer) WriteInt16(i int16) {
	b.WriteUInt16(uint16(i))
}

/*
32 bit numbers
*/
func (b *Buffer) ReadUInt32() (uint32, error) {
	if b.Len() < 8 {
		return 0, io.EOF
	}
	return binary.BigEndian.Uint32(b.Next(4)), nil
}
func (b *Buffer) ReadInt32() (int32, error) {
	i, e := b.ReadUInt32()
	return int32(i), e
}
func (b *Buffer) ReadFloat32() (float32, error) {
	i, e := b.ReadUInt32()
	return math.Float32frombits(i), e
}
func (b *Buffer) WriteUInt32(i uint32) {
	binary.BigEndian.PutUint32(b.tmp[:4], i)
	b.Write(b.tmp[:4])
}
func (b *Buffer) WriteInt32(i int32) {
	b.WriteUInt32(uint32(i))
}
func (b *Buffer) WriteFloat32(f float32) {
	b.WriteUInt32(math.Float32bits(f))
}

/*
64 bit numbers
*/
func (b *Buffer) ReadUInt64() (uint64, error) {
	if b.Len() < 8 {
		return 0, io.EOF
	}
	return binary.BigEndian.Uint64(b.Next(8)), nil
}
func (b *Buffer) ReadInt64() (int64, error) {
	i, e := b.ReadUInt64()
	return int64(i), e
}
func (b *Buffer) ReadFloat64() (float64, error) {
	i, e := b.ReadUInt64()
	return math.Float64frombits(i), e
}
func (b *Buffer) WriteUInt64(i uint64) {
	binary.BigEndian.PutUint64(b.tmp[:8], i)
	b.Write(b.tmp[:8])
}
func (b *Buffer) WriteInt64(i int64) {
	b.WriteUInt64(uint64(i))
}
func (b *Buffer) WriteFloat64(f float64) {
	b.WriteUInt64(math.Float64bits(f))
}
