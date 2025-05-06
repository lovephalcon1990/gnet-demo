package handler

import (
	"github.com/gnet-io/gnet-examples/custom_protocol/pb"
	"github.com/gnet-io/gnet-examples/custom_protocol/server/queue"
	"github.com/panjf2000/gnet/v2"
	"github.com/panjf2000/gnet/v2/pkg/logging"
)

type MessageHandler struct {
	queue *queue.MessageQueue
}

func NewMessageHandler(queue *queue.MessageQueue) *MessageHandler {
	return &MessageHandler{
		queue: queue,
	}
}

func (h *MessageHandler) HandleLogin(conn gnet.Conn, req *pb.LoginRequest) {
	// 处理登录请求
	logging.Infof("Processing login request from user: %s", req.Username)

	// 创建登录响应
	resp := &pb.LoginRequest{
		Username: req.Username,
		Password: "******", // 实际应用中应该返回处理结果
	}

	// 广播登录响应
	h.queue.Broadcast(&queue.Message{
		CmdId: 1,
		Body:  resp,
	})
}

func (h *MessageHandler) HandleChat(conn gnet.Conn, msg *pb.ChatMessage) {
	// 处理聊天消息
	logging.Infof("Processing chat message: %s", msg.Content)

	// 广播聊天消息
	h.queue.Broadcast(&queue.Message{
		CmdId: 2,
		Body:  msg,
	})
}

func (h *MessageHandler) HandleMessage(conn gnet.Conn, cmdId uint32, msg interface{}) {
	switch cmdId {
	case 1:
		if loginReq, ok := msg.(*pb.LoginRequest); ok {
			h.HandleLogin(conn, loginReq)
		}
	case 2:
		if chatMsg, ok := msg.(*pb.ChatMessage); ok {
			h.HandleChat(conn, chatMsg)
		}
	default:
		logging.Warnf("Unknown command ID: %d", cmdId)
	}
}
