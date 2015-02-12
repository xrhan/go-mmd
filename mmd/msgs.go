package mmd

import (
	"crypto/rand"
	"encoding/hex"
)

var NO_AUTH_TOKEN = string(make([]byte, 16))

//Probably not useful as a public function
//TODO: switch to raw struct and set defaults upon usage
func NewChannelCreate(chanType ChannelType, service string, body interface{}) ChannelCreate {
	return ChannelCreate{
		ChannelId: ChannelId(newUUID()),
		Type:      chanType,
		Service:   service,
		Timeout:   3,
		AuthToken: AuthToken(NO_AUTH_TOKEN),
		Body:      body,
	}
}

func GenerateUUID() UUID {
	return newUUID()
}

func newUUID() UUID {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return UUID(b)
}

type MMDError struct {
	code int
	msg  interface{}
}

type UUID string
type ChannelId UUID
type AuthToken UUID

// Typesafe enum
type ChannelType int

/*type MMDMessage interface {
	Encode(buffer *bytes.Buffer)
}
*/
func (u UUID) Bytes() []byte {
	return []byte(u)
}
func (u UUID) String() string {
	return hex.EncodeToString([]byte(u))
}

func (c AuthToken) String() string {
	return UUID(c).String()
}
func (c ChannelId) String() string {
	return UUID(c).String()
}

const (
	CallChan ChannelType = iota
	SubChan  ChannelType = iota
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

type ChannelMsg struct {
	IsClose bool
	Channel ChannelId
	Body    interface{}
}
