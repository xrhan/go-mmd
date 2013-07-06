package mmd

import (
	"encoding/hex"
)

//Probably not useful as a public function
//TODO: switch to raw struct and set defaults upon usage
func NewChannelCreate(chanType ChannelType, service string, body interface{}) ChannelCreate {
	return ChannelCreate{
		ChannelId: ChannelId{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
		Type:      chanType,
		Service:   service,
		Timeout:   3,
		AuthToken: AuthToken{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
		Body:      body,
	}
}

type MMDError struct {
	code int
	msg  interface{}
}

// No built in UUID type, so might as well make our own channel type
type UUID []byte
type ChannelId UUID
type AuthToken UUID

// Typesafe enum
type ChannelType int

/*type MMDMessage interface {
	Encode(buffer *bytes.Buffer)
}
*/
func (uuid UUID) String() string {
	return hex.EncodeToString(uuid)
}

func (c AuthToken) String() string {
	return hex.EncodeToString(c)
}
func (c ChannelId) String() string {
	return hex.EncodeToString(c)
}

const (
	Call      ChannelType = iota
	Subscribe ChannelType = iota
)

// Familiar MMD Message types
type ChannelCreate struct {
	ChannelId ChannelId
	Type      ChannelType
	Service   string
	Timeout   int64
	AuthToken AuthToken
	Body      interface{}
}

type ChannelClose struct {
	ChannelId ChannelId
	Body      interface{}
}

type ChannelMessage struct {
	ChannelId ChannelId
	Body      interface{}
}
