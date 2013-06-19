package mmd

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

var decodeTable []func(byte, *bytes.Buffer) (interface{}, error)

func init() { // this init() is here due to compiler bug(?) on init loop
	decodeTable = []func(byte, *bytes.Buffer) (interface{}, error){
		0x00: fastInt,
		0x01: fastInt,
		0x02: fastInt,
		0x04: fastInt,
		0x08: fastInt,
		0x10: fastUInt,
		0x11: fastUInt,
		0x12: fastUInt,
		0x14: fastUInt,
		0x18: fastUInt,
		'S':  varString,
		's':  fastString,
		'X':  decodeClose,
		'M':  decodeMessage,
		'i':  varUInt,
		'I':  varInt,
		'l':  varUInt,
		'L':  varInt,
		'r':  fastMap,
		'm':  varIntMap,
		'a':  fastArray,
		'A':  varIntArray,
	}
}

func fastMap(tag byte, buffer *bytes.Buffer) (interface{}, error) {
	return decodeMap(buffer, fastSz(buffer))
}
func varIntMap(tag byte, buffer *bytes.Buffer) (interface{}, error) {
	return decodeMap(buffer, varSz(buffer))
}
func decodeMap(buffer *bytes.Buffer, num int) (interface{}, error) {
	ret := make(map[interface{}]interface{}, num)
	for i := 0; i < num; i++ {
		key, kerr := Decode(buffer)
		if kerr != nil {
			return nil, errors.New(fmt.Sprintf("Failed to decode key: %s", kerr))
		}
		val, verr := Decode(buffer)
		if verr != nil {
			return nil, errors.New(fmt.Sprintf("Failed to decode value: %s", verr))
		}
		ret[key] = val
	}

	return ret, nil
}

func fastArray(tag byte, buffer *bytes.Buffer) (interface{}, error) {
	return decodeArray(buffer, fastSz(buffer))
}
func varIntArray(tag byte, buffer *bytes.Buffer) (interface{}, error) {
	return decodeArray(buffer, varSz(buffer))
}
func decodeArray(buffer *bytes.Buffer, num int) (interface{}, error) {
	ret := make([]interface{}, num)
	for i := 0; i < num; i++ {
		obj, err := Decode(buffer)
		if err != nil {
			return nil, err
		}
		ret[i] = obj
	}
	return ret, nil
}

func varUInt(tag byte, buffer *bytes.Buffer) (interface{}, error) {
	n, err := binary.ReadUvarint(buffer)
	chkErr(err)
	return uint(n), nil
}
func varInt(tag byte, buffer *bytes.Buffer) (interface{}, error) {
	n, err := binary.ReadVarint(buffer)
	chkErr(err)
	return int(n), nil
}

func varString(tag byte, buffer *bytes.Buffer) (interface{}, error) {
	sz, err := binary.ReadUvarint(buffer)
	if err != nil {
		return nil, err
	}
	return string(buffer.Next(int(sz))), nil
}

func varSz(buffer *bytes.Buffer) int {
	sz, err := binary.ReadUvarint(buffer)
	chkErr(err)
	return int(sz)
}
func fastSz(buffer *bytes.Buffer) int {
	tag, err := buffer.ReadByte()
	chkErr(err)
	return int(fastUIntN(tag&0x0F, buffer))
}

func fastInt(tag byte, buffer *bytes.Buffer) (interface{}, error) {
	b, err := buffer.ReadByte()
	chkErr(err)
	return int(fastUIntN(b&0xFF, buffer)), nil
}

func fastUInt(tag byte, buffer *bytes.Buffer) (interface{}, error) {
	return fastUIntN(tag&0x0F, buffer), nil
}

//TODO: this probably doesn't work for signed conversions as the high bit
func fastUIntN(sz byte, buffer *bytes.Buffer) uint {
	switch sz {
	case 0:
		return uint(0)
	case 1:
		b, err := buffer.ReadByte()
		chkErr(err)
		return uint(b)
	case 2:
		return uint(binary.BigEndian.Uint16(buffer.Next(2)))
	case 4:
		return uint(binary.BigEndian.Uint32(buffer.Next(4)))
	case 8:
		return uint(binary.BigEndian.Uint64(buffer.Next(8)))
	default:
		panic(fmt.Sprintf("Invalid integer size: %d", sz))
	}
}

func badTag(msg string, b byte) string {
	return fmt.Sprintf("%s %c:%d", msg, b, b)
}
func fastString(tag byte, buffer *bytes.Buffer) (interface{}, error) {
	return string(buffer.Next(fastSz(buffer))), nil
}

func decodeClose(tag byte, buffer *bytes.Buffer) (interface{}, error) {
	fmt.Println("Decoding close")
	chanId := buffer.Next(16)
	body, err := Decode(buffer)
	chkErr(err)
	return ChannelClose{
		ChannelId: ChannelId(chanId),
		Body:      body,
	}, nil
}

func decodeMessage(tag byte, buffer *bytes.Buffer) (interface{}, error) {
	fmt.Println("Decoding message")
	chanId := buffer.Next(16)
	body, err := Decode(buffer)
	chkErr(err)
	return ChannelMessage{
		ChannelId: ChannelId(chanId),
		Body:      body,
	}, nil
}

func Decode(buffer *bytes.Buffer) (interface{}, error) {
	tag, err := buffer.ReadByte()
	chkErr(err)
	fn := decodeTable[tag]

	if fn == nil {
		return nil, errors.New(fmt.Sprintf("Unsupported type tag: %c:%d", tag, tag))
	}
	return fn(tag, buffer)
}
