package udp

import (
	"context"
	"errors"
	"net"
	"sync"
)

type Package struct {
	addr net.Addr
	data []byte
}

type Conn struct {
	*net.UDPConn
	context   context.Context
	once      sync.Once
	cancel    func()
	OnData    func([]byte, net.Addr)
	OnError   func(error)
	OnClose   func()
	writeChan chan *Package
}

func ListenUDP(ctx context.Context, network string, addr string) (*Conn, error) {
	var lAddr *net.UDPAddr
	if addr != "" {
		var err error
		lAddr, err = net.ResolveUDPAddr("udp", addr)
		if err != nil {
			return nil, err
		}
	}
	rawConn, err := net.ListenUDP(network, lAddr)
	if err != nil {
		return nil, err
	}
	cli := &Conn{
		UDPConn:   rawConn,
		writeChan: make(chan *Package, 10),
	}
	cli.context, cli.cancel = context.WithCancel(ctx)
	cli.start()
	return cli, nil
}

func (cli *Conn) start() {
	//读取数据
	go func() {
		buf := make([]byte, 1500)
		for {
			select {
			case <-cli.Done():
				cli.Close()
				return
			default:
				if cli.OnData != nil {
					n, from, err := cli.ReadFrom(buf)
					if err != nil {
						if cli.OnError != nil {
							cli.OnError(err)
						}
						cli.Close()
						return
					}
					cli.OnData(buf[:n], from)
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
			case w := <-cli.writeChan:
				if _, err = cli.UDPConn.WriteTo(w.data, w.addr); err != nil {
					return
				}
			}
		}
	}()
}

func (cli *Conn) Done() <-chan struct{} {
	return cli.context.Done()
}

func (cli *Conn) WriteTo(data []byte, addr net.Addr) {
	cli.writeChan <- &Package{
		addr: addr, data: data,
	}
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
		err = cli.UDPConn.Close()
	})
	if first {
		return err
	}
	return errors.New("close error")
}
