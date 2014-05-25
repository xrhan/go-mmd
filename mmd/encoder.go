package mmd

import (
	"encoding/binary"
	"fmt"
	"math"
	"reflect"
	"time"
)

//<<?CHANNEL_CREATE,Chan:16/binary,Type:1/binary,SvcSize:8/unsigned-integer,Svc:SvcSize/binary,Timeout:16/signed-integer,AT:16/binary,Body/binary>>
func Encode(buffer *Buffer, thing interface{}) error {
	switch i := thing.(type) {
	case nil:
		buffer.WriteByte('N')
	case string:
		buffer.WriteByte('s')
		writeSz(buffer, len(i))
		buffer.WriteString(i)
	case ChannelCreate:
		buffer.WriteByte('c')
		buffer.Write([]byte(i.ChannelId))
		switch i.Type {
		case Call:
			buffer.WriteByte('C')
		case Subscribe:
			buffer.WriteByte('S')
		default:
			return fmt.Errorf("Unknown type: %v", i.Type)
		}
		buffer.WriteByte(byte(len(i.Service)))
		buffer.WriteString(i.Service)
		ta := make([]byte, 2)
		binary.BigEndian.PutUint16(ta, uint16(i.Timeout))
		buffer.Write(ta)
		buffer.Write([]byte(i.AuthToken))
		Encode(buffer, i.Body)
	case float32:
		buffer.WriteByte('d')
		buffer.WriteFloat32(i)
	case float64:
		buffer.WriteByte('D')
		buffer.WriteFloat64(i)
	case int:
		return encodeInt(buffer, int64(i))
	case uint:
		return encodeUint(buffer, uint64(i))
	case time.Time:
		buffer.WriteByte('z')
		buffer.WriteInt64(int64(i.UnixNano() / 1000))
	case []interface{}: // common case, don't reflect
		buffer.WriteByte('a')
		writeSz(buffer, len(i))
		for _, item := range i {
			err := Encode(buffer, item)
			if err != nil {
				return fmt.Errorf("Error encoding: %v - %v", item, err)
			}
		}
	case bool:
		if i {
			buffer.WriteByte('t')
		} else {
			buffer.WriteByte('f')
		}
	default:
		return reflectEncode(thing, buffer)
	}
	return nil
}

func encodeInt(buffer *Buffer, i int64) error {
	if i == 0 {
		buffer.WriteByte(0)
	} else if i >= math.MinInt8 && i <= math.MaxInt8 {
		buffer.WriteByte(0x01)
		buffer.WriteByte(byte(i))
	} else if i >= math.MinInt16 && i <= math.MaxInt16 {
		buffer.WriteByte(0x02)
		buffer.order.PutUint16(buffer.GetWritable(2), uint16(i))
	} else if i >= math.MinInt32 && i <= math.MaxInt32 {
		buffer.WriteByte(0x04)
		buffer.order.PutUint32(buffer.GetWritable(4), uint32(i))
	} else if i >= math.MinInt64 && i <= math.MaxInt64 {
		buffer.WriteByte(0x08)
		buffer.order.PutUint64(buffer.GetWritable(8), uint64(i))
	} else {
		return fmt.Errorf("Don't know how to encode int(%d)", i)
	}
	return nil
}

func encodeUint(buffer *Buffer, i uint64) error {
	if i == 0 {
		buffer.WriteByte(0)
	} else if i <= math.MaxUint8 {
		buffer.WriteByte(0x11)
		buffer.WriteByte(byte(i))
	} else if i <= math.MaxUint16 {
		buffer.WriteByte(0x12)
		buffer.order.PutUint16(buffer.GetWritable(2), uint16(i))
	} else if i <= math.MaxUint32 {
		buffer.WriteByte(0x14)
		buffer.order.PutUint32(buffer.GetWritable(4), uint32(i))
	} else if i <= math.MaxUint64 {
		buffer.WriteByte(0x18)
		buffer.order.PutUint64(buffer.GetWritable(8), i)
	} else {
		return fmt.Errorf("Don't know how to encode int(%d)", i)
	}
	return nil
}

func reflectEncode(thing interface{}, buffer *Buffer) error {
	val := reflect.ValueOf(thing)
	kind := val.Kind()
	switch kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return encodeInt(buffer, val.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return encodeUint(buffer, val.Uint())
	case reflect.Slice:
		buffer.WriteByte('a')
		buffer.WriteByte(0x04)
		buffer.order.PutUint32(buffer.GetWritable(4), uint32(val.Len()))
		for i := 0; i < val.Len(); i++ {
			item := val.Index(i)
			if !item.CanInterface() {
				return fmt.Errorf("Can't Interface() %s", val)
			} else {
				err := Encode(buffer, item.Interface())
				if err != nil {
					return err
				}
			}
		}
		return nil
	case reflect.Map:
		buffer.WriteByte('r') // fast map
		writeSz(buffer, val.Len())
		for _, k := range val.MapKeys() {
			ki := k.Interface()
			vi := val.MapIndex(k).Interface()
			err := Encode(buffer, ki)
			if err != nil {
				return err
			}
			err = Encode(buffer, vi)
			if err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("Don't know how to encode (%s) %v", kind, thing)
	}
}

func writeSz(buffer *Buffer, sz int) {
	if sz > 256 {
		panic("sz must be <= 256")
	}
	buffer.WriteByte(0x01)
	buffer.WriteByte(byte(sz))
}
