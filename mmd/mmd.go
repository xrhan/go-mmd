package mmd

import (
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	logpkg "log"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"
)

var log = logpkg.New(os.Stdout, "[mmd] ", logpkg.LstdFlags|logpkg.Lmicroseconds)
var mmdUrl = "localhost:9999"

const defaultTimeout = time.Second * 30

func init() {
	flag.StringVar(&mmdUrl, "mmd", mmdUrl, "Sets default MMD Url")
}

// EOC Signals close of MMD channel
var EOC = errors.New("End Of Channel")

// ServiceFunc Handler callback for registered services
type ServiceFunc func(*Conn, *Chan, *ChannelCreate)

type OnConnection func(*Conn) error

type Config struct {
	Url               string
	ReadSz            int
	WriteSz           int
	AppName           string
	AutoRetry         bool
	ReconnectInterval time.Duration
	OnConnect         OnConnection
}

func NewConfig(url string) *Config {
	return &Config{
		Url:               url,
		ReadSz:            64 * 1024,
		WriteSz:           64 * 1024,
		AppName:           fmt.Sprintf("Go:%s", filepath.Base(os.Args[0])),
		AutoRetry:         false,
		ReconnectInterval: defaultTimeout,
	}
}

func (c *Config) Connect() (*Conn, error) {
	return _create_connection(c)
}

// Creates a default URL connection (-mmd to override)
func Connect() (*Conn, error) {
	return ConnectTo(mmdUrl)
}
func LocalConnect() (*Conn, error) {
	return ConnectTo("localhost:9999")
}
func ConnectTo(url string) (*Conn, error) {
	return NewConfig(url).Connect()
}

func ConnectWithRetry(url string, reconnectInterval time.Duration, onConnect OnConnection) (*Conn, error) {
	cfg := NewConfig(url)
	cfg.ReconnectInterval = reconnectInterval
	cfg.AutoRetry = true
	cfg.OnConnect = onConnect
	return cfg.Connect()
}

func _create_connection(cfg *Config) (*Conn, error) {
	mmdc := &Conn{
		dispatch:    make(map[ChannelId]chan ChannelMsg, 1024),
		callTimeout: time.Second * 30,
		services:    make(map[string]ServiceFunc),
		config:      cfg,
	}

	err := mmdc.createSocketConnection()
	if err != nil {
		return nil, err
	}

	return mmdc, err
}

func (c *Conn) reconnect() {
	err := c.closeSocket()
	if err != nil {
		log.Panicln("Failed to close socket: ", err)
	}

	err = c.createSocketConnection()
	if err != nil {
		log.Println("Failed to reconnect: ", err)
		return
	}
	log.Println("Socket reset. Reconnected to mmd")
}

func (c *Conn) createSocketConnection() error {
	addr, err := net.ResolveTCPAddr("tcp", c.config.Url)
	if err != nil {
		return err
	}

	for {
		conn, err := net.DialTCP("tcp", nil, addr)
		if err != nil && c.config.AutoRetry {
			time.Sleep(c.config.ReconnectInterval)
			log.Println("Disconnected. Retrying again")
			continue
		}

		if err == nil {
			conn.SetWriteBuffer(c.config.WriteSz)
			conn.SetReadBuffer(c.config.ReadSz)
			c.socket = conn
			return c.onSocketConnection()
		}

		return err
	}
}

func (c *Conn) onSocketConnection() error {
	c.writeChan = make(chan []byte)
	c.startWriter()
	c.startReader()
	c.handshake()

	if c.config.OnConnect != nil {
		return c.config.OnConnect(c)
	}

	return nil
}

// Conn Connection and channel dispatch map
type Conn struct {
	socket      *net.TCPConn
	writeChan   chan []byte
	dispatch    map[ChannelId]chan ChannelMsg
	dlock       sync.RWMutex
	callTimeout time.Duration
	services    map[string]ServiceFunc
	config      *Config
}

// Chan MMD Channel
type Chan struct {
	Ch  chan ChannelMsg
	con *Conn
	Id  ChannelId
}

func (c *Chan) NextMessage() (ChannelMsg, error) {
	a, ok := <-c.Ch
	if !ok {
		return ChannelMsg{}, EOC
	}
	return a, nil
}
func (c *Chan) Close(body interface{}) error {
	cm := ChannelMsg{Channel: c.Id, Body: body, IsClose: true}
	c.con.unregisterChannel(c.Id)
	buff := NewBuffer(1024)
	err := Encode(buff, cm)
	if err != nil {
		return err
	}
	c.con.Send(buff.Flip())
	return nil
}
func (c *Chan) Send(body interface{}) error {
	cm := ChannelMsg{Channel: c.Id, Body: body}
	buff := NewBuffer(1024)
	err := Encode(buff, cm)
	if err != nil {
		return err
	}
	c.con.Send(buff.Flip())
	return nil
}

func (c *Chan) Errorf(code int, format string, args ...interface{}) error {
	return c.Error(code, fmt.Sprintf(format, args...))
}

func (c *Chan) Error(code int, body interface{}) error {
	return c.Close(&MMDError{code, body})
}

func (c *Chan) ErrorInvalidRequest(body interface{}) error {
	return c.Error(Err_INVALID_REQUEST, body)
}

// Call Calls a service
func Call(service string, body interface{}) (interface{}, error) {
	lc, err := LocalConnect()
	if err != nil {
		return nil, err
	}
	defer lc.Close()
	return lc.Call(service, body)
}

func (c *Conn) SetDefaultCallTimeout(dur time.Duration) {
	c.callTimeout = dur
}
func (c *Conn) GetDefaultCallTimeout() time.Duration {
	return c.callTimeout
}

func (c *Conn) registerServiceUtil(name string, fn ServiceFunc, registryAction string) error {
	c.services[name] = fn
	ok, err := c.Call("serviceregistry", map[string]interface{}{
		"action": registryAction,
		"name":   name,
	})
	if err == nil && ok != "ok" {
		err = fmt.Errorf("Unexpected return: %v", ok)
	}
	if err != nil {
		delete(c.services, name)
	}
	return err
}

func (c *Conn) RegisterLocalService(name string, fn ServiceFunc) error {
	return c.registerServiceUtil(name, fn, "registerLocal")
}

func (c *Conn) RegisterService(name string, fn ServiceFunc) error {
	return c.registerServiceUtil(name, fn, "register")
}

func (c *Conn) Subscribe(service string, body interface{}) (*Chan, error) {
	buff := NewBuffer(1024)
	cc := NewChannelCreate(SubChan, service, body)
	err := Encode(buff, cc)
	if err != nil {
		return nil, err
	}
	ch := make(chan ChannelMsg, 1)
	c.registerChannel(cc.ChannelId, ch)
	c.Send(buff.Flip())
	return &Chan{Ch: ch, con: c, Id: cc.ChannelId}, nil
}

func (c *Conn) Call(service string, body interface{}) (interface{}, error) {
	return c.CallAuthenticated(service, AuthToken(NO_AUTH_TOKEN), body)
}

func (c *Conn) CallAuthenticated(service string, token AuthToken, body interface{}) (interface{}, error) {
	buff := NewBuffer(1024)
	cc := NewChannelCreate(CallChan, service, body)
	cc.AuthToken = token
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
		e, ok := ret.Body.(MMDError)
		if ok {
			return nil, fmt.Errorf("MMD Error: %d: %v", e.code, e.msg)
		}
		return ret.Body, nil
	case <-time.After(c.callTimeout):
		return nil, fmt.Errorf("Timeout waiting for: %s", service)
	}
}

func (c *Conn) registerChannel(cid ChannelId, ch chan ChannelMsg) {
	c.dlock.Lock()
	c.dispatch[cid] = ch
	c.dlock.Unlock()
}
func (c *Conn) unregisterChannel(cid ChannelId) chan ChannelMsg {
	c.dlock.Lock()
	ret, ok := c.dispatch[cid]
	if ok {
		delete(c.dispatch, cid)
	}
	c.dlock.Unlock()
	return ret
}
func (c *Conn) lookupChannel(cid ChannelId) chan ChannelMsg {
	c.dlock.RLock()
	ret := c.dispatch[cid]
	c.dlock.RUnlock()
	return ret
}

func (c Conn) String() string {
	return fmt.Sprintf("Conn{remote: %s, local: %s}", c.socket.RemoteAddr(), c.socket.LocalAddr())
}

func (c *Conn) Send(buff *Buffer) {
	c.writeChan <- buff.Bytes()
}
func (c *Conn) Close() {
	close(c.writeChan)
}

func (c *Conn) onDisconnect() {
	c.cleanupReader()
	c.cleanupWriter()
	if c.config.AutoRetry {
		c.reconnect()
	}
}

func (c *Conn) cleanupReader() {
	log.Println("Cleaning up reader")
	defer c.dlock.Unlock()
	c.socket.CloseRead()
	c.dlock.Lock()
	for k, v := range c.dispatch {
		log.Println("Auto-closing channel", k)
		close(v)
	}
}

func (c *Conn) cleanupWriter() {
	c.Close()
	c.socket.CloseWrite()
}

func (c *Conn) handshake() {
	handshake := []byte{1, 1}
	handshake = append(handshake, c.config.AppName...)

	c.WriteFrame(handshake)
}

func (c *Conn) WriteFrame(data []byte) {
	c.writeChan <- data
}

func (c *Conn) startReader() {
	go reader(c)
}

func (c *Conn) startWriter() {
	go writer(c)
}

func (c *Conn) closeSocket() error {
	if c.socket != nil {
		err := c.socket.Close()
		return err
	}

	log.Println("Cannot close a nil socket")
	return nil
}

func reader(c *Conn) {
	fszb := make([]byte, 4)
	buff := make([]byte, 256)
	defer c.onDisconnect()
	for {
		num, err := c.socket.Read(fszb)
		if err != nil {
			if err == io.EOF {
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
			case ChannelCreate:
				fn, ok := c.services[msg.Service]
				if !ok {
					log.Println("Unknown service:", msg.Service, "cannot process", msg)
				}
				ch := make(chan ChannelMsg, 1)
				c.registerChannel(msg.ChannelId, ch)
				fn(c, &Chan{Ch: ch, con: c, Id: msg.ChannelId}, &msg)
			default:
				log.Panic("Unknown message type:", reflect.TypeOf(msg), msg)
			}
		}
	}
}

func writer(c *Conn) {
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
				c.socket.CloseWrite()
				return
			}
		}
	}
}
