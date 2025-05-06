package main

import (
	"flag"
	"fmt"
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
	bodyBuf, err := serializer.ReadPacketHeader(c)
	if err != nil {
		logging.Errorf("read packet header error: %v", err)
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
		gnet.WithReuseAddr(true),
		// gnet.WithReusePort(true),
	)
	logging.Infof("server exits with error: %v", err)
}
