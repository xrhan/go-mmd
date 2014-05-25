package mmd

import (
	"net"
)

type FrameReader interface {
	ReadFrame() ([]byte, error)
}
type FrameWriter interface {
	WriteFrame([]byte) error
}
type FramedSocket struct {
	Connection net.Conn
}

func (fs *FramedSocket) ReadFrame() {
}
