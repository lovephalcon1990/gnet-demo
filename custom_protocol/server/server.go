package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"math"
	"sync/atomic"
	"time"

	"github.com/gnet-io/gnet-examples/custom_protocol/pb"
	"github.com/gnet-io/gnet-examples/custom_protocol/serializer"
	"github.com/gnet-io/gnet-examples/custom_protocol/server/handler"
	"github.com/gnet-io/gnet-examples/custom_protocol/server/queue"
	"github.com/panjf2000/gnet/v2"
	"github.com/panjf2000/gnet/v2/pkg/logging"
)

type customServer struct {
	gnet.BuiltinEventEngine
	eng       gnet.Engine
	network   string
	addr      string
	multicore bool
	connected int32
	batchRead int
	queue     *queue.MessageQueue
	handler   *handler.MessageHandler
}

func (s *customServer) OnBoot(eng gnet.Engine) (action gnet.Action) {
	logging.Infof("running server on %s with multi-core=%t",
		fmt.Sprintf("%s://%s", s.network, s.addr), s.multicore)
	s.eng = eng
	s.queue.Start()
	return
}

func (s *customServer) OnOpen(c gnet.Conn) (out []byte, action gnet.Action) {
	atomic.AddInt32(&s.connected, 1)
	logging.Infof("connection=%s starts...", c.RemoteAddr().String())
	s.queue.AddClient(c)
	return
}

func (s *customServer) OnClose(c gnet.Conn, err error) (action gnet.Action) {
	if err != nil {
		logging.Infof("error occurred on connection=%s, %v\n", c.RemoteAddr().String(), err)
	}
	atomic.AddInt32(&s.connected, -1)
	logging.Infof("connection=%s stops...", c.RemoteAddr().String())
	s.queue.RemoveClient(c)
	return
}

func (s *customServer) OnTick() (delay time.Duration, action gnet.Action) {
	delay = time.Second * 5
	logging.Infof("connected clients: %d", s.connected)
	s.queue.Broadcast(&queue.Message{
		CmdId: 2,
		Body:  &pb.ChatMessage{Content: "hi"},
	})
	return
}

func (s *customServer) OnTraffic(c gnet.Conn) (action gnet.Action) {
	// 读取并处理数据包
	// 1. 读取固定长度包头
	headerBuf := make([]byte, serializer.HeaderSize)
	if _, err := io.ReadFull(c, headerBuf); err != nil {
		if err == io.EOF {
			logging.Infof("connection closed by server")
			return
		}
		logging.Errorf("read header error: %v", err)
		return
	}

	// 2. 解析包头获取包体长度
	bodyLen := binary.BigEndian.Uint32(headerBuf[26:serializer.HeaderSize])
	if bodyLen == 0 || bodyLen > 10*1024*1024 {
		logging.Errorf("invalid body length: %d", bodyLen)
		return
	}

	// 3. 读取包体
	bodyBuf := make([]byte, bodyLen)
	if _, err := io.ReadFull(c, bodyBuf); err != nil {
		logging.Errorf("read body error: %v", err)
		return
	}

	// 4. 解析消息
	cmdId, msg, err := serializer.DecodeMessage(bodyBuf)
	if err != nil {
		logging.Errorf("failed to decode message: %v", err)
		return
	}

	// 处理消息
	s.handler.HandleMessage(c, cmdId, msg)
	return
}

func main() {
	var (
		port      int
		multicore bool
		batchRead int
	)

	// Example command: go run server.go --port 9000 --multicore=true --batchread 10
	flag.IntVar(&port, "port", 9999, "--port 9000")
	flag.BoolVar(&multicore, "multicore", true, "--multicore=true")
	flag.IntVar(&batchRead, "batchread", 100, "--batch-read 100")
	flag.Parse()
	if batchRead <= 0 {
		batchRead = math.MaxInt32 // unlimited batch read
	}

	// 创建消息队列和处理器
	msgQueue := queue.NewMessageQueue()
	msgHandler := handler.NewMessageHandler(msgQueue)

	ss := &customServer{
		network:   "tcp",
		addr:      fmt.Sprintf(":%d", port),
		multicore: multicore,
		batchRead: batchRead,
		queue:     msgQueue,
		handler:   msgHandler,
	}

	err := gnet.Run(ss, ss.network+"://"+ss.addr,
		gnet.WithMulticore(multicore),
		gnet.WithTCPKeepAlive(0),
		gnet.WithTicker(true),
	)
	logging.Infof("server exits with error: %v", err)
}
