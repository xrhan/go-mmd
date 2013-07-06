package mmd

import (
	// "bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"time"
)

var decodeTable []func(byte, *Buffer) (interface{}, error)

func init() { // this init() is here due to compiler bug(?) on init loop
	decodeTable = []func(byte, *Buffer) (interface{}, error){
		0x00: decodeFastInt,
		0x01: decodeFastInt,
		0x02: decodeFastInt,
		0x04: decodeFastInt,
		0x08: decodeFastInt,
		0x10: decodeFastUInt,
		0x11: decodeFastUInt,
		0x12: decodeFastUInt,
		0x14: decodeFastUInt,
		0x18: decodeFastUInt,
		'S':  varString,
		's':  fastString,
		'X':  decodeClose,
		'M':  decodeMessage,
		'i':  decodeVaruint,
		'I':  decodeVarint,
		'l':  decodeVaruint,
		'L':  decodeVarint,
		'r':  fastMap,
		'm':  varIntMap,
		'a':  fastArray,
		'A':  varIntArray,
		'b':  varBytes,
		'q':  fastBytes,
		'e':  fastError,
		'E':  varError,
		// 'a':  ack,
		// 'p':  pong,
		// 'P':  ping,
		'D': decodeDouble,
		'd': decodeFloat,
		'n': decodeNil,
		't': decodeTrue,
		'f': decodeFalse,
		'U': decodeUUID,
		'$': decodeSecID,
		'B': decodeByte,
		'#': decodeVarintTime,
		'z': decodeFastTime,
	}
}

func Decode(buff *Buffer) (ret interface{}, err error) {
	if tag, err := buff.ReadByte(); err == nil {
		fn := decodeTable[tag]
		if fn == nil {
			err = errors.New(fmt.Sprintf("Unsupported type tag: %c:%d", tag, tag))
		} else {
			ret, err = fn(tag, buff)
		}
	}
	return
}

// func SDecode(buff *Buffer) (ret interface{}, err error) {
// 	b, err := buff.ReadByte()
// 	if err == nil {
// 		switch b {
// 		case 0x00:
// 			ret = int(0)
// 		case 0x10:
// 			ret = uint(0)
// 		case 0x01:
// 			b, err := buff.ReadByte()
// 			if err != nil {
// 				ret = int(b)
// 			}
// 		case 0x11:
// 			b, err := buff.ReadByte()
// 			if err != nil {
// 				ret = uint(b)
// 			}
// 		}
// 	}
// 	return
// }

type MMDError struct {
	code int
	msg  interface{}
}

func decodeFastInt(tag byte, buff *Buffer) (interface{}, error) {
	return fastInt(tag&0x0F, buff)
}
func decodeFastUInt(tag byte, buff *Buffer) (interface{}, error) {
	return fastUInt(tag&0x0F, buff)
}

const microToSecond = 1024 * 1024

func decodeVarintTime(tag byte, buff *Buffer) (ret interface{}, err error) {
	if sz, err := buff.ReadByte(); err == nil {
		if t, err := decodeFastInt(sz, buff); err == nil {
			ret = convertTime(t.(int))
		}
	}
	return
}

func convertTime(t int) time.Time {
	sec := t / microToSecond
	nsec := (t % microToSecond) * 1024
	return time.Unix(int64(sec), int64(nsec))

}

func decodeFastTime(tag byte, buff *Buffer) (ret interface{}, err error) {
	t, err := taggedFastInt(buff)
	if err != nil {
		return
	}
	ret = convertTime(t)
	return
}

func decodeByte(tag byte, buff *Buffer) (interface{}, error) {
	return buff.ReadByte()
}
func decodeUUID(tag byte, buff *Buffer) (interface{}, error) {
	return buff.Next(16)
}
func decodeSecID(tag byte, buff *Buffer) (interface{}, error) {
	return buff.Next(16)
}
func decodeNil(tag byte, buff *Buffer) (interface{}, error) {
	return nil, nil
}
func decodeTrue(tag byte, buff *Buffer) (interface{}, error) {
	return true, nil
}
func decodeFalse(tag byte, buff *Buffer) (interface{}, error) {
	return false, nil
}

func decodeFloat(tag byte, buff *Buffer) (ret interface{}, err error) {
	v, err := buff.Next(4)
	if err == nil {
		ret = math.Float32frombits(buff.order.Uint32(v))
	}
	return
}
func decodeDouble(tag byte, buff *Buffer) (ret interface{}, err error) {
	v, err := buff.Next(8)
	if err == nil {
		ret = math.Float64frombits(buff.order.Uint64(v))
	}
	return
}
func fastError(tag byte, buff *Buffer) (ret interface{}, err error) {
	code, err := taggedFastInt(buff)
	if err == nil {
		if body, err := Decode(buff); err == nil {
			ret = MMDError{code, body}
		}
	}
	return
}

func varError(tag byte, buff *Buffer) (ret interface{}, err error) {
	return
}

func fastBytes(tag byte, buff *Buffer) (ret interface{}, err error) {
	if sz, err := fastSz(buff); err != nil {
		return buff.Next(sz)
	}
	return
}
func varBytes(tag byte, buff *Buffer) (ret interface{}, err error) {
	if sz, err := buff.ReadVarint(); err != nil {
		ret, err = buff.Next(sz)
	}
	return
}
func fastMap(tag byte, buff *Buffer) (ret interface{}, err error) {
	if sz, err := fastSz(buff); err == nil {
		ret, err = decodeMap(buff, sz)
	}
	return
}

func varIntMap(tag byte, buff *Buffer) (ret interface{}, err error) {
	if sz, err := buff.ReadVarint(); err == nil {
		ret, err = decodeMap(buff, sz)
	}
	return
}

func decodeMap(buff *Buffer, num int) (interface{}, error) {
	ret := make(map[interface{}]interface{}, num)
	for i := 0; i < num; i++ {
		key, kerr := Decode(buff)
		if kerr != nil {
			return nil, errors.New(fmt.Sprintf("Failed to decode key: %s", kerr))
		}
		val, verr := Decode(buff)
		if verr != nil {
			return nil, errors.New(fmt.Sprintf("Failed to decode value: %s", verr))
		}
		ret[key] = val
	}

	return ret, nil
}

func fastArray(tag byte, buff *Buffer) (ret interface{}, err error) {
	if sz, err := taggedFastInt(buff); err == nil {
		ret, err = decodeArray(buff, sz)
	}
	return
}

func varIntArray(tag byte, buff *Buffer) (ret interface{}, err error) {
	if sz, err := varSz(buff); err == nil {
		ret, err = decodeArray(buff, sz)
	}
	return
}

func decodeArray(buff *Buffer, num int) (interface{}, error) {
	ret := make([]interface{}, num)
	for i := 0; i < num; i++ {
		obj, err := Decode(buff)
		if err != nil {
			return nil, err
		}
		ret[i] = obj
	}
	return ret, nil
}

func decodeVarint(tag byte, buff *Buffer) (interface{}, error) {
	return varInt(buff)
}
func decodeVaruint(tag byte, buff *Buffer) (interface{}, error) {
	return varUInt(buff)
}

func varInt(buff *Buffer) (ret int, err error) {
	if n, err := binary.ReadVarint(buff); err == nil {
		ret = int(n)
	}
	return
}
func varUInt(buff *Buffer) (ret uint, err error) {
	if n, err := binary.ReadUvarint(buff); err == nil {
		ret = uint(n)
	}
	return
}

func taggedFastInt(buff *Buffer) (ret int, err error) {
	if tag, err := buff.ReadByte(); err == nil {
		ret, err = fastInt(tag, buff)
	}
	return
}
func taggedFastUInt(buff *Buffer) (ret uint, err error) {
	if tag, err := buff.ReadByte(); err == nil {
		ret, err = fastUInt(tag, buff)
	}
	return
}
func fastInt(sz byte, buff *Buffer) (ret int, err error) {
	switch sz {
	case 0x00:
		ret = int(0)
	case 0x01:
		v, err := buff.ReadByte()
		ret = int(v)
	case 0x02:
		v, err := buff.Next(2)
		ret = int(buff.order.Uint16(v))
	case 0x04:
		v, err := buff.Next(4)
		ret = int(buff.order.Uint32(v))
	case 0x08:
		v, err := buff.Next(8)
		ret = int(buff.order.Uint64(v))
	}
	return
}

func fastUInt(sz byte, buff *Buffer) (ret uint, err error) {
	switch sz {
	case 0x00:
		ret = uint(0)
	case 0x01:
		v, err := buff.ReadByte()
		ret = uint(v)
	case 0x02:
		v, err := buff.Next(2)
		ret = uint(buff.order.Uint16(v))
	case 0x04:
		v, err := buff.Next(4)
		ret = uint(buff.order.Uint32(v))
	case 0x08:
		v, err := buff.Next(8)
		ret = uint(buff.order.Uint64(v))
	}
	return
}

func varString(tag byte, buff *Buffer) (interface{}, error) {
	sz, err := binary.ReadUvarint(buff)
	if err != nil {
		return nil, err
	}
	return buff.ReadString(int(sz))
}

func varSz(buff *Buffer) (int, error) {
	return varInt(buff)
}

func fastSz(buff *Buffer) (int, error) {
	return taggedFastInt(buff)
}

// func decodeFastInt(tag byte, buff *Buffer) (interface{}, error) {
// 	b, err := buff.ReadByte()
// 	chkErr(err)
// 	return int(fastUIntN(b&0xFF, buffer)), nil
// }

// func fastUInt(tag byte, buff *Buffer) (interface{}, error) {
// 	return fastUIntN(tag&0x0F, buffer), nil
// }

// //TODO: this probably doesn't work for signed conversions as the high bit
// func fastUIntN(sz byte, buff *Buffer) uint {
// 	switch sz {
// 	case 0:
// 		return uint(0)
// 	case 1:
// 		b, err := buff.ReadByte()
// 		chkErr(err)
// 		return uint(b)
// 	case 2:
// 		return uint(buff.NextUInt16())
// 	case 4:
// 		return uint(binary.BigEndian.Uint32(buff.Next(4)))
// 	case 8:
// 		return uint(binary.BigEndian.Uint64(buff.Next(8)))
// 	default:
// 		panic(fmt.Sprintf("Invalid integer size: %d", sz))
// 	}
// }

func badTag(msg string, b byte) string {
	return fmt.Sprintf("%s %c:%d", msg, b, b)
}

func fastString(tag byte, buff *Buffer) (ret interface{}, err error) {
	if sz, err := taggedFastInt(buff); err == nil {
		ret, err = buff.ReadString(sz)
	}
	return
}

func decodeClose(tag byte, buff *Buffer) (ret interface{}, err error) {
	if chanId, err := buff.Next(16); err == nil {
		if body, err := Decode(buff); err == nil {
			ret = ChannelClose{
				ChannelId: ChannelId(chanId),
				Body:      body,
			}
		}
	}
	return
}

func decodeMessage(tag byte, buff *Buffer) (ret interface{}, err error) {
	if chanId, err := buff.Next(16); err == nil {
		if body, err := Decode(buff); err == nil {
			ret = &ChannelMessage{
				ChannelId: ChannelId(chanId),
				Body:      body,
			}
		}
	}
	return
}
