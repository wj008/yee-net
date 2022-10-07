package tcp

import (
	"context"
	"log"
	"testing"
	"time"
)

func TestNewServer(t *testing.T) {
ReStart:

	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	serv, err := NewServer(ctx, "tcp", ":9000")
	if err != nil {
		log.Println(err)
		return
	}
	log.Println("服务启动成功...")
	serv.OnAccept = func(conn *Conn) {
		log.Println("有链接进入", conn.RemoteAddr())
		conn.OnData = func(bytes []byte) {
			log.Println("读取数据：", string(bytes))
		}
		conn.OnClose = func() {
			log.Println("链接已经关闭")
		}
		conn.OnError = func(err error) {
			log.Println("数据读取错误", err)
		}
	}
	select {
	case <-serv.Done():
		serv.Close()
		time.Sleep(2 * time.Second)
		log.Println("重启服务器")
		goto ReStart
	}
}

func TestDial(t *testing.T) {
ReStart:
	conn, err := Dial(context.Background(), "tcp", "127.0.0.1:9000")
	if err != nil {
		log.Println("服务错误")
		time.Sleep(2 * time.Second)
		log.Println("重启客户端")
		goto ReStart
	}
	conn.OnClose = func() {
		log.Println("链接已经关闭")
	}
	conn.OnError = func(err error) {
		log.Println("数据读取错误", err)
	}
	conn.OnData = func(bytes []byte) {
		log.Println(string(bytes))
	}

	go func() {
		for {
			select {
			case <-conn.Done():
				return
			case <-time.After(time.Second):
				log.Println("写入数据")
				conn.Write([]byte("测试写入数据"))
			}
		}
	}()

	select {
	case <-conn.Done():
		conn.Close()
		time.Sleep(2 * time.Second)
		log.Println("重启客户端")
		goto ReStart
	}
}
