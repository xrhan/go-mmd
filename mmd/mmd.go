package mmd

import (
	"encoding/binary"
	"fmt"
	"io"
	logpkg "log"
	"net"
	"os"
	"sync"
)

var log = logpkg.New(os.Stdout, "[mmd] ", logpkg.LstdFlags|logpkg.Lmicroseconds)

type config struct {
	url     string
	readSz  int
	writeSz int
	appName string
}

type MMDConn struct {
	socket    *net.TCPConn
	writeChan chan []byte
	dispatch  map[ChannelId]chan ChannelMsg
	dlock     sync.RWMutex
}

func NewConfig(url string) *config {
	return &config{
		url:     url,
		readSz:  1024 * 1024,
		writeSz: 1024 * 1024,
		appName: fmt.Sprintf("Go:%s", os.Args[0]),
	}
}

func LocalConnect() (*MMDConn, error) {
	return Connect(NewConfig("localhost:9999"))
}
func Connect(cfg *config) (*MMDConn, error) {
	addr, err := net.ResolveTCPAddr("tcp", cfg.url)
	if err != nil {
		return nil, err
	}
	log.Printf("Connecting to: %s / %s\n", cfg.url, addr)
	conn, err := net.DialTCP("tcp", nil, addr)
	if err != nil {
		return nil, err
	}
	conn.SetWriteBuffer(cfg.writeSz)
	conn.SetReadBuffer(cfg.readSz)
	mmdc := &MMDConn{socket: conn,
		writeChan: make(chan []byte),
		dispatch:  make(map[ChannelId]chan ChannelMsg, 1024),
	}
	go writer(mmdc)
	go reader(mmdc)
	handshake := []byte{1, 1}
	handshake = append(handshake, cfg.appName...)
	mmdc.WriteFrame(handshake)
	log.Println("Connected:", mmdc)
	return mmdc, nil
}

func (c *MMDConn) Call(service string, body interface{}) (interface{}, error) {
	buff := NewBuffer(1024)
	cc := NewChannelCreate(Call, service, body)
	err := Encode(buff, cc)
	if err != nil {
		return nil, err
	}
	ch := make(chan ChannelMsg, 1)
	c.registerChannel(cc.ChannelId, ch)
	c.Send(buff.Flip())
	//TODO: timeout here
	ret := <-ch
	return ret.Body, nil
}

func (c *MMDConn) registerChannel(cid ChannelId, ch chan ChannelMsg) {
	c.dlock.Lock()
	c.dispatch[cid] = ch
	c.dlock.Unlock()
}
func (c *MMDConn) unregisterChannel(cid ChannelId) chan ChannelMsg {
	c.dlock.Lock()
	ret := c.dispatch[cid]
	c.dlock.Unlock()
	return ret
}
func (c *MMDConn) lookupChannel(cid ChannelId) chan ChannelMsg {
	c.dlock.RLock()
	ret := c.dispatch[cid]
	c.dlock.RUnlock()
	return ret
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

func cleanupReader(c *MMDConn) {
	log.Println("Cleaning up reader")
	c.socket.CloseRead()
	c.dlock.Lock()
	for k, v := range c.dispatch {
		log.Println("Auto-closing channel", k)
		close(v)
	}
}

func reader(c *MMDConn) {
	fszb := make([]byte, 4)
	defer cleanupReader(c)
	for {
		num, err := c.socket.Read(fszb)
		if err != nil {
			if err == io.EOF {
				fmt.Println("Reader closed")
				return
			}
			log.Println("Error reading frame size:", err)
			return
		}
		if num != 4 {
			log.Println("Short read for size:", num)
			return
		}
		fsz := int(binary.BigEndian.Uint32(fszb))
		b := make([]byte, fsz)

		reads := 0
		for fsz > 0 {
			sz, err := c.socket.Read(b)
			if err != nil {
				log.Println("Error reading message:", err)
				return
			}
			reads++
			fsz -= sz
		}
		m, err := Decode(Wrap(b))
		if err != nil {
			log.Println("Error decoding buffer:", err)
		} else {
			switch msg := m.(type) {
			case ChannelMsg:
				if msg.IsClose {
					ch := c.unregisterChannel(msg.Channel)
					if ch != nil {
						ch <- msg
						close(ch)
					} else {
						log.Println("Unknown channel:", msg.Channel, "discarding message")
					}
				} else {
					ch := c.lookupChannel(msg.Channel)
					if ch != nil {
						ch <- msg
					} else {
						log.Println("Unknown channel:", msg.Channel, "discarding message")
					}
				}
			default:
				log.Println("Unknown message:", m)
			}
		}
	}
}

func writer(c *MMDConn) {
	fsz := make([]byte, 4)
	for {
		select {
		case data, ok := <-c.writeChan:
			if ok {
				binary.BigEndian.PutUint32(fsz, uint32(len(data)))
				_, err := c.socket.Write(fsz)
				if err != nil {
					log.Println("Failed to write header:", fsz, err)
				} else {
					_, err = c.socket.Write(data)
					if err != nil {
						log.Println("Failed to write data", err)
					}
				}
			} else {
				log.Println("Exiting")
				c.socket.CloseWrite()
				return
			}
		}
	}
}

func (c *MMDConn) WriteFrame(data []byte) {
	c.writeChan <- data
}
