package main

import (
	"flag"
	"fmt"
	"math"
	"sync/atomic"

	"github.com/panjf2000/gnet/v2"
	"github.com/panjf2000/gnet/v2/pkg/logging"

	"github.com/gnet-io/gnet-examples/simple_protocol/protocol"
)

type simpleServer struct {
	gnet.BuiltinEventEngine
	eng          gnet.Engine
	network      string
	addr         string
	multicore    bool
	connected    int32
	disconnected int32
	batchRead    int // maximum number of packet to read per event-loop iteration
}

func (s *simpleServer) OnBoot(eng gnet.Engine) (action gnet.Action) {
	logging.Infof("running server on %s with multi-core=%t",
		fmt.Sprintf("%s://%s", s.network, s.addr), s.multicore)
	s.eng = eng
	return
}

func (s *simpleServer) OnOpen(c gnet.Conn) (out []byte, action gnet.Action) {
	c.SetContext(new(protocol.SimpleCodec))
	atomic.AddInt32(&s.connected, 1)
	out = []byte("sweetness\r\n")
	return
}

func (s *simpleServer) OnClose(c gnet.Conn, err error) (action gnet.Action) {
	if err != nil {
		logging.Infof("error occurred on connection=%s, %v\n", c.RemoteAddr().String(), err)
	}
	disconnected := atomic.AddInt32(&s.disconnected, 1)
	connected := atomic.AddInt32(&s.connected, -1)
	if connected == 0 {
		logging.Infof("all %d connections are closed, shut it down", disconnected)
		action = gnet.None
	}
	return
}

func (s *simpleServer) OnTraffic(c gnet.Conn) (action gnet.Action) {
	codec := c.Context().(*protocol.SimpleCodec)
	var packets [][]byte
	for i := 0; i < s.batchRead; i++ {
		data, err := codec.Decode(c)
		if err == protocol.ErrIncompletePacket {
			break
		}
		if err != nil {
			logging.Errorf("invalid packet: %v", err)
			return gnet.Close
		}
		packet, _ := codec.Encode(data)
		packets = append(packets, packet)
	}
	if n := len(packets); n > 1 {
		_, _ = c.Writev(packets)
	} else if n == 1 {
		_, _ = c.Write(packets[0])
	}
	if len(packets) == s.batchRead && c.InboundBuffered() > 0 {
		if err := c.Wake(nil); err != nil { // wake up the connection manually to avoid missing the leftover data
			logging.Errorf("failed to wake up the connection, %v", err)
			return gnet.Close
		}
	}
	return
}

func main() {
	var (
		port      int
		multicore bool
		batchRead int
	)

	// Example command: go run server.go --port 9000 --multicore=true --batchread 10
	flag.IntVar(&port, "port", 9000, "--port 9000")
	flag.BoolVar(&multicore, "multicore", false, "--multicore=true")
	flag.IntVar(&batchRead, "batchread", 100, "--batch-read 100")
	flag.Parse()
	if batchRead <= 0 {
		batchRead = math.MaxInt32 // unlimited batch read
	}
	ss := &simpleServer{
		network:   "tcp",
		addr:      fmt.Sprintf(":%d", port),
		multicore: multicore,
		batchRead: batchRead,
	}
	err := gnet.Run(ss, ss.network+"://"+ss.addr, gnet.WithMulticore(multicore))
	logging.Infof("server exits with error: %v", err)
}
