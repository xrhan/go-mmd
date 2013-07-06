package mmd

import (
	// "bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net"
)

type config struct {
	url     string
	readSz  int
	writeSz int
}

func NewConfig(url string) *config {
	return &config{url: url, readSz: 1024 * 1024, writeSz: 1024 * 1024}
}

func chkErr(err error) {
	if err != nil {
		panic(err)
	}
}

func LocalConnect() *MMDConn {
	return Connect(NewConfig("localhost:9999"))
}

type MMDConn struct {
	socket    *net.TCPConn
	writeChan chan []byte
	readChan  chan []byte
}

func (c MMDConn) String() string {
	return fmt.Sprintf("MMDConn{remote: %s, local: %s}", c.socket.RemoteAddr(), c.socket.LocalAddr())
}

func (c *MMDConn) Send(buff *Buffer) {
	c.writeChan <- buff.Bytes()
}
func (c *MMDConn) Close() {
	close(c.writeChan)
}

func (c *MMDConn) NextMessage() (interface{}, error) {
	cm := <-c.readChan
	fmt.Println("Decoding")
	fmt.Println(hex.Dump(cm))
	return Decode(Wrap(cm))
}

func (c *MMDConn) Call(service string, body interface{}) (interface{}, error) {
	buff := NewBuffer(1024)
	cc := NewChannelCreate(Call, service, body)
	Encode(buff, cc)
	c.Send(buff.Flip())
	return c.NextMessage()
}

func Connect(cfg *config) *MMDConn {
	addr, err := net.ResolveTCPAddr("tcp", cfg.url)
	chkErr(err)
	fmt.Printf("Connecting to: %s / %s\n", cfg.url, addr)
	conn, err := net.DialTCP("tcp", nil, addr)
	chkErr(err)
	conn.SetWriteBuffer(cfg.writeSz)
	conn.SetReadBuffer(cfg.readSz)
	mmdc := &MMDConn{socket: conn, writeChan: make(chan []byte), readChan: make(chan []byte)}
	go writer(mmdc)
	go reader(mmdc)
	mmdc.WriteFrame([]byte{1, 1})
	fmt.Println("Connected:", mmdc)
	return mmdc
}

func cleanupReader(c *MMDConn) {
	fmt.Println("Cleaning up reader")
	close(c.readChan)
	c.socket.CloseRead()
}

func reader(c *MMDConn) {
	fszb := make([]byte, 4)
	defer cleanupReader(c)
	for true {
		num, err := c.socket.Read(fszb)
		if num != 4 {
			panic(fmt.Sprintf("Short read: %d", num))
		}
		if err == io.EOF {
			fmt.Println("Reader closed")
			return
		}
		chkErr(err)
		fsz := int(binary.BigEndian.Uint32(fszb))
		b := make([]byte, fsz)

		reads := 0
		for fsz > 0 {
			r, e := c.socket.Read(b)
			reads++
			chkErr(e)
			fsz = fsz - r
		}
		fmt.Printf("Read %d bytes in %d reads\n%s", fsz, reads, hex.Dump(b))
		c.readChan <- b
		break
	}
}

func writer(c *MMDConn) {
	fsz := make([]byte, 4)
	for true {
		select {
		case data, ok := <-c.writeChan:
			if ok {
				binary.BigEndian.PutUint32(fsz, uint32(len(data)))
				c.socket.Write(fsz)
				fmt.Printf("Writing %d bytes\n%s\n", len(data), hex.Dump(data))
				c.socket.Write(data)
			} else {
				fmt.Println("Exiting")
				c.socket.CloseWrite()
				return
			}
		}
	}
}

func (c *MMDConn) WriteFrame(data []byte) {
	c.writeChan <- data
}
