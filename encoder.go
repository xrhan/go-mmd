package mmd

import (
	"encoding/binary"
	"errors"
	"fmt"
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
		buffer.Write(i.ChannelId)
		switch i.Type {
		case Call:
			buffer.WriteByte('C')
		case Subscribe:
			buffer.WriteByte('S')
		default:
			panic("Unknown type")
		}
		buffer.WriteByte(byte(len(i.Service)))
		buffer.WriteString(i.Service)
		ta := make([]byte, 2)
		binary.BigEndian.PutUint16(ta, uint16(i.Timeout))
		buffer.Write(ta)
		buffer.Write(i.AuthToken)
		Encode(buffer, i.Body)
	default:
		return errors.New(fmt.Sprintf("Don't know how to encode: %#v\n", i))
	}
	return nil
}

func writeSz(buffer *Buffer, sz int) {
	if sz > 256 {
		panic("sz must be <= 256")
	}
	buffer.WriteByte(0x01)
	buffer.WriteByte(byte(sz))
}
