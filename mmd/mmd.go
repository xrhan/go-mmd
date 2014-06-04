package mmd

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	logpkg "log"
	"net"
	"os"
	"reflect"
	"sync"
	"time"
)

var log = logpkg.New(os.Stdout, "[mmd] ", logpkg.LstdFlags|logpkg.Lmicroseconds)
var EOC = errors.New("End Of Channel")

type config struct {
	url     string
	readSz  int
	writeSz int
	appName string
}

type MMDConn struct {
	socket      *net.TCPConn
	writeChan   chan []byte
	dispatch    map[ChannelId]chan ChannelMsg
	dlock       sync.RWMutex
	callTimeout time.Duration
}
type MMDChan struct {
	ch  chan ChannelMsg
	con *MMDConn
	Id  ChannelId
}

func (c *MMDChan) NextMessage() (ChannelMsg, error) {
	a, ok := <-c.ch
	if !ok {
		return ChannelMsg{}, EOC
	}
	return a, nil
}
func (c *MMDChan) Close(body interface{}) error {
	cm := ChannelMsg{Channel: c.Id, Body: body, IsClose: true}
	buff := NewBuffer(1024)
	err := Encode(buff, cm)
	if err != nil {
		return err
	}
	c.con.Send(buff.Flip())
	return nil
}
func (c *MMDChan) Send(body interface{}) error {
	cm := ChannelMsg{Channel: c.Id, Body: body}
	buff := NewBuffer(1024)
	err := Encode(buff, cm)
	if err != nil {
		return err
	}
	c.con.Send(buff.Flip())
	return nil
}

func (c *MMDConn) SetDefaultCallTimeout(dur time.Duration) {
	c.callTimeout = dur
}
func (c *MMDConn) GetDefaultCallTimeout() time.Duration {
	return c.callTimeout
}
func newConfig(url string) *config {
	return &config{
		url:     url,
		readSz:  64 * 1024,
		writeSz: 64 * 1024,
		appName: fmt.Sprintf("Go:%s", os.Args[0]),
	}
}

func LocalConnect() (*MMDConn, error) {
	return Connect(newConfig("localhost:9999"))
}
func ConnectTo(host string, port int) (*MMDConn, error) {
	return Connect(newConfig(fmt.Sprintf("%s:%d", host, port)))
}
func Connect(cfg *config) (*MMDConn, error) {
	addr, err := net.ResolveTCPAddr("tcp", cfg.url)
	if err != nil {
		return nil, err
	}
	// log.Printf("Connecting to: %s / %s\n", cfg.url, addr)
	conn, err := net.DialTCP("tcp", nil, addr)
	if err != nil {
		return nil, err
	}
	conn.SetWriteBuffer(cfg.writeSz)
	conn.SetReadBuffer(cfg.readSz)
	mmdc := &MMDConn{socket: conn,
		writeChan:   make(chan []byte),
		dispatch:    make(map[ChannelId]chan ChannelMsg, 1024),
		callTimeout: time.Second * 5,
	}
	go writer(mmdc)
	go reader(mmdc)
	handshake := []byte{1, 1}
	handshake = append(handshake, cfg.appName...)
	mmdc.WriteFrame(handshake)
	// log.Println("Connected:", mmdc)
	return mmdc, nil
}

func (c *MMDConn) Subscribe(service string, body interface{}) (*MMDChan, error) {
	buff := NewBuffer(1024)
	cc := NewChannelCreate(Call, service, body)
	cc.Type = Subscribe
	err := Encode(buff, cc)
	if err != nil {
		return nil, err
	}
	ch := make(chan ChannelMsg, 1)
	c.registerChannel(cc.ChannelId, ch)
	defer c.unregisterChannel(cc.ChannelId)
	c.Send(buff.Flip())
	return &MMDChan{ch: ch, con: c, Id: cc.ChannelId}, nil
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
	defer c.unregisterChannel(cc.ChannelId)
	c.Send(buff.Flip())
	select {
	case ret := <-ch:
		return ret.Body, nil
	case <-time.After(c.callTimeout):
		return nil, fmt.Errorf("Timeout waiting for: %s", service)
	}
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
	buff := make([]byte, 256)
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
		if len(buff) < fsz {
			buff = make([]byte, fsz)
		}

		reads := 0
		offset := 0
		for offset < fsz {
			sz, err := c.socket.Read(buff[offset:fsz])
			if err != nil {
				log.Panic("Error reading message:", err)
				return
			}
			reads++
			offset += sz
		}
		log.Println("Decoding:", fsz, "bytes")
		m, err := Decode(Wrap(buff[:fsz]))
		if err != nil {
			log.Panic("Error decoding buffer:", err)
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
				log.Panic("Unknown message type:", reflect.TypeOf(msg), msg)
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
