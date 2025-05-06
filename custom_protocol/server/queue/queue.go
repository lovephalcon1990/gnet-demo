package queue

import (
	"fmt"
	"sync"

	"github.com/gnet-io/gnet-examples/custom_protocol/pb"
	"github.com/gnet-io/gnet-examples/custom_protocol/serializer"
	"github.com/panjf2000/gnet/v2"
	"github.com/panjf2000/gnet/v2/pkg/logging"
)

type MessageQueue struct {
	mu       sync.RWMutex
	clients  map[gnet.Conn]struct{}
	msgChan  chan *Message
	stopChan chan struct{}
}

type Message struct {
	CmdId uint32
	Body  interface{}
}

func NewMessageQueue() *MessageQueue {
	return &MessageQueue{
		clients:  make(map[gnet.Conn]struct{}),
		msgChan:  make(chan *Message, 1000),
		stopChan: make(chan struct{}),
	}
}

func (q *MessageQueue) Start() {
	go q.processMessages()
}

func (q *MessageQueue) Stop() {
	close(q.stopChan)
}

func (q *MessageQueue) AddClient(conn gnet.Conn) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.clients[conn] = struct{}{}
}

func (q *MessageQueue) RemoveClient(conn gnet.Conn) {
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.clients, conn)
}

func (q *MessageQueue) Broadcast(msg *Message) {
	select {
	case q.msgChan <- msg:
	default:
		logging.Warnf("message queue is full, message dropped")
	}
}

func (q *MessageQueue) processMessages() {
	for {
		select {
		case <-q.stopChan:
			return
		case msg := <-q.msgChan:
			q.mu.RLock()
			for conn := range q.clients {
				// 根据消息类型处理
				switch msg.CmdId {
				case 1: // Login Response
					if loginResp, ok := msg.Body.(*pb.LoginRequest); ok {
						// 发送登录响应
						// TODO: 实现消息序列化和发送
						logging.Infof("Sending login response to fd:%d client: %v", conn.Fd(), loginResp)
					}
				case 2: // Chat Message
					if chatMsg, ok := msg.Body.(*pb.ChatMessage); ok {
						// 发送聊天消息
						// TODO: 实现消息序列化和发送
						logging.Infof("Sending chat message to fd:%d client: %v", conn.Fd(), chatMsg)
						msg := fmt.Sprintf("hi fd:%d", conn.Fd())
						bytes, _ := serializer.EncodeMessage(2, &pb.ChatMessage{Content: msg})
						conn.Write(bytes)
					}
				}
			}
			q.mu.RUnlock()
		}
	}
}
