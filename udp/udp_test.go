package udp

import (
	"context"
	"log"
	"net"
	"testing"
	"time"
)

func TestListenUDP(t *testing.T) {
ReStart:
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	conn, err := ListenUDP(ctx, "udp", ":9000")
	if err != nil {
		log.Println(err)
		return
	}
	log.Println("服务启动成功...")
	conn.OnData = func(bytes []byte, from net.Addr) {
		log.Println("读取数据：", from, string(bytes))
	}
	conn.OnClose = func() {
		log.Println("链接已经关闭")
	}
	conn.OnError = func(err error) {
		log.Println("数据读取错误", err)
	}
	select {
	case <-conn.Done():
		conn.Close()
		time.Sleep(2 * time.Second)
		log.Println("重启服务器")
		goto ReStart
	}
}

func TestListenUDP2(t *testing.T) {
ReStart:
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	conn, err := ListenUDP(ctx, "udp", "")
	if err != nil {
		log.Println(err)
		return
	}
	log.Println("服务启动成功...")
	conn.OnData = func(bytes []byte, from net.Addr) {
		log.Println("读取数据：", from, string(bytes))
	}
	conn.OnClose = func() {
		log.Println("链接已经关闭")
	}
	conn.OnError = func(err error) {
		log.Println("数据读取错误", err)
	}
	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:9000")

	go func() {
		for {
			select {
			case <-conn.Done():
				return
			case <-time.After(time.Second):
				log.Println("写入数据")
				conn.WriteTo([]byte("测试写入数据"), addr)
			}
		}
	}()
	select {
	case <-conn.Done():
		conn.Close()
		time.Sleep(2 * time.Second)
		log.Println("重启服务器")
		goto ReStart
	}
}
