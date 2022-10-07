package tcp

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
)

type Conn struct {
	net.Conn
	context   context.Context
	once      sync.Once
	cancel    func()
	OnData    func([]byte)
	OnError   func(error)
	OnClose   func()
	writeChan chan []byte
	UserData  any
}

type Server struct {
	net.Listener
	context  context.Context
	once     sync.Once
	cancel   func()
	OnAccept func(*Conn)
	OnClose  func()
}

// NewServer 创建服务
func NewServer(ctx context.Context, network string, addr string) (*Server, error) {
	listener, err := net.Listen(network, addr)
	if err != nil {
		return nil, err
	}
	serv := &Server{
		Listener: listener,
	}
	serv.context, serv.cancel = context.WithCancel(ctx)
	go func() {
		defer log.Println("close server")
		for {
			select {
			case <-serv.Done():
				serv.Close()
				return
			default:
				rawConn, err := listener.Accept()
				if err != nil {
					continue
				}
				cli := &Conn{
					Conn:      rawConn,
					writeChan: make(chan []byte, 100),
				}
				cli.context, cli.cancel = context.WithCancel(serv.context)
				if serv.OnAccept != nil {
					cli.start()
					serv.OnAccept(cli)
				}
			}
		}
	}()
	return serv, nil
}

// Close 关闭服务
func (serv *Server) Close() (err error) {
	first := false
	serv.once.Do(func() {
		first = true
		serv.cancel()
		if serv.OnClose != nil {
			serv.OnClose()
		}
		err = serv.Listener.Close()
	})
	if first {
		return err
	}
	return errors.New("close error")
}

func (serv *Server) Done() <-chan struct{} {
	return serv.context.Done()
}

func Dial(ctx context.Context, network string, addr string) (*Conn, error) {
	rawConn, err := net.Dial(network, addr)
	if err != nil {
		return nil, err
	}
	cli := &Conn{
		Conn:      rawConn,
		writeChan: make(chan []byte, 10),
	}
	cli.context, cli.cancel = context.WithCancel(ctx)
	cli.start()
	return cli, nil
}

// readBytes 读取套接字字节
func (cli *Conn) readBytes(length int) ([]byte, error) {
	end := length
	buffer := make([]byte, length)
	temp := buffer[0:end]
	reTry := 0
	nLen := 0
	for {
		select {
		case <-cli.context.Done():
			return nil, errors.New("链接已经退出")
		default:
			reTry++
			if reTry > 100 {
				return nil, errors.New(fmt.Sprintf("Expected to read %d bytes, but only read %d", length, nLen))
			}
			n, err := cli.Read(temp)
			if err != nil {
				return nil, err
			}
			nLen += n
			if n < end {
				temp = buffer[n:end]
				end = end - n
				continue
			}
			return buffer, nil
		}
	}
}

func (cli *Conn) start() {
	//读取数据
	go func() {
		for {
			select {
			case <-cli.Done():
				cli.Close()
				return
			default:
				if cli.OnData != nil {
					data, err := cli.readData()
					if err != nil {
						if cli.OnError != nil {
							cli.OnError(err)
						}
						cli.Close()
						return
					}
					cli.OnData(data)
				}
			}
		}
	}()
	//写入队列
	go func() {
		var err error
		defer func() {
			if err != nil {
				if cli.OnError != nil {
					cli.OnError(err)
				}
				cli.Close()
			}
		}()
		for {
			select {
			case <-cli.Done():
				err = errors.New("connect is closed")
				return
			case data := <-cli.writeChan:
				buffer := new(bytes.Buffer)
				length := uint32(len(data))
				if err = binary.Write(buffer, binary.BigEndian, length); err != nil {
					return
				}
				if _, err = buffer.Write(data); err != nil {
					return
				}
				if _, err = cli.Conn.Write(buffer.Bytes()); err != nil {
					return
				}
			}
		}
	}()
}

// ReadData 读取消息
func (cli *Conn) readData() ([]byte, error) {
	head, err := cli.readBytes(4)
	if err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(head)
	if length == 0 {
		return []byte{}, nil
	}
	data, err := cli.readBytes(int(length))
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (cli *Conn) Done() <-chan struct{} {
	return cli.context.Done()
}

func (cli *Conn) Write(data []byte) {
	cli.writeChan <- data
}

func (cli *Conn) Close() (err error) {
	first := false
	cli.once.Do(func() {
		first = true
		cli.cancel()
		close(cli.writeChan)
		if cli.OnClose != nil {
			cli.OnClose()
		}
		err = cli.Conn.Close()
	})
	if first {
		return err
	}
	return errors.New("close error")
}
