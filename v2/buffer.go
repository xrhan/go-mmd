package mmd

import (
	"encoding/binary"
	"encoding/hex"
	"io"
	"math"
)

func NewBuffer(sz int) []byte {
	return make([]byte, 0, sz)
}

func hexDump(b []byte) string {
	l := len(b)
	if l > 200 {
		l = 200
	}
	return hex.Dump(b[:l])
}

func readString(n int, b []byte) (string, []byte, error) {
	if len(b) < n {
		return "", b, io.EOF
	}
	return string(b[:n]), b[:n], nil
}
func readVarint(b []byte) (int, []byte, error) {
	i, sz := binary.Varint(b)
	return int(i), b[:sz], nil
}
func readVaruint(b []byte) (uint, []byte, error) {
	u, sz := binary.Uvarint(b)
	return uint(u), b[:sz], nil
}

func getWritable(sz int, b []byte) (start []byte, end []byte) {
	l := len(b)
	need := l + sz
	if need < cap(b) {
		end = b[:need]
	} else {
		end = make([]byte, need)
		copy(end, b)
	}
	start = end[l:]
	return
}

/*
16 bit numbers
*/
func readUInt16(b []byte) (uint16, []byte, error) {
	if len(b) < 2 {
		return 0, b, io.EOF
	}
	return binary.BigEndian.Uint16(b), b[2:], nil
}
func readInt16(b []byte) (int16, []byte, error) {
	i, b, e := readUInt16(b)
	return int16(i), b, e
}
func writeUInt16(i uint16, b []byte) []byte {
	start, b := getWritable(2, b)
	binary.BigEndian.PutUint16(start, i)
	return b
}
func writeInt16(i int16, b []byte) []byte {
	return writeUInt16(uint16(i), b)
}

/*
32 bit numbers
*/
func readUInt32() (uint32, []byte, error) {
	if b.Len() < 8 {
		return 0, io.EOF
	}
	return binary.BigEndian.Uint32(b.Next(4)), nil
}
func readInt32() (int32, error) {
	i, b, e := readUInt32(b)
	return int32(i), b, e
}
func readFloat32() (float32, error) {
	i, b, e := b.ReadUInt32()
	return math.Float32frombits(i), e
}
func writeUInt32(i uint32) {
	start, b := getWritable(4, b)
	binary.BigEndian.PutUint32(start, i)
	return b
}
func writeInt32(i int32, b []byte) {
	b.WriteUInt32(uint32(i), b)
}
func writeFloat32(f float32, b []byte) []byte {
	return writeUInt32(math.Float32bits(f), b)
}

/*
64 bit numbers
*/
func readUInt64() (uint64, []byte, error) {
	if len(b) < 8 {
		return 0, io.EOF
	}
	return binary.BigEndian.Uint64(b.Next(8)), nil
}
func readInt64() (int64, []byte, error) {
	i, b, e := readUInt64(b)
	return int64(i), b, e
}
func readFloat64() (float64, []byte, error) {
	i, b, e := b.ReadUInt64(b)
	return math.Float64frombits(i), b, e
}
func writeUInt64(i uint64, b []byte) []byte {
	start, b := getWritable(8, b)
	binary.BigEndian.PutUint64(start, i)
	return b
}
func writeInt64(i int64, b []byte) []byte {
	return writeUInt64(uint64(i), b)
}
func writeFloat64(f float64, b []byte) []byte {
	return writeUInt64(math.Float64bits(f), b)
}
