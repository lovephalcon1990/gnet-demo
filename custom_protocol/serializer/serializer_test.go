package serializer

import (
	"testing"

	"github.com/gnet-io/gnet-examples/custom_protocol/pb"
	"google.golang.org/protobuf/proto"
)

func TestEncodeMessage(t *testing.T) {
	// 创建一个登录请求
	loginReq := &pb.LoginRequest{
		Username: "testuser",
		Password: "testpass",
	}

	// 编码消息
	packet, err := EncodeMessage(1, loginReq)
	if err != nil {
		t.Fatalf("encode message: %v", err)
	}

	// 解码消息
	decodedCmdId, decodedMsg, err := DecodeMessage(packet[HeaderSize:]) // 注意这里需要跳过头部
	if err != nil {
		t.Fatalf("decode message: %v", err)
	}

	// 验证解码后的命令ID是否正确
	if decodedCmdId != 1 {
		t.Fatalf("decoded cmdId: %d, expected 1", decodedCmdId)
	}

	// 验证消息类型和内容
	decodedLoginReq, ok := decodedMsg.(*pb.LoginRequest)
	if !ok {
		t.Fatalf("decoded message type: %T, expected *pb.LoginRequest", decodedMsg)
	}

	if decodedLoginReq.Username != "testuser" || decodedLoginReq.Password != "testpass" {
		t.Fatalf("decoded message content mismatch: got {Username: %s, Password: %s}, want {Username: testuser, Password: testpass}",
			decodedLoginReq.Username, decodedLoginReq.Password)
	}
}

func TestEncodeDecodeMultipleMessages(t *testing.T) {
	tests := []struct {
		name    string
		cmdId   uint32
		message proto.Message
	}{
		{
			name:  "Login Request",
			cmdId: 1,
			message: &pb.LoginRequest{
				Username: "testuser",
				Password: "testpass",
			},
		},
		{
			name:  "Chat Message",
			cmdId: 2,
			message: &pb.ChatMessage{
				Content: "Hello!",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 编码消息
			packet, err := EncodeMessage(tt.cmdId, tt.message)
			if err != nil {
				t.Fatalf("encode message failed: %v", err)
			}

			// 解码消息
			decodedCmdId, decodedMsg, err := DecodeMessage(packet[HeaderSize:])
			if err != nil {
				t.Fatalf("decode message failed: %v", err)
			}

			// 验证命令ID
			if decodedCmdId != tt.cmdId {
				t.Errorf("command ID mismatch: got %d, want %d", decodedCmdId, tt.cmdId)
			}

			// 验证消息内容
			switch m := tt.message.(type) {
			case *pb.LoginRequest:
				decoded, ok := decodedMsg.(*pb.LoginRequest)
				if !ok {
					t.Fatalf("wrong message type: got %T, want *pb.LoginRequest", decodedMsg)
				}
				if decoded.Username != m.Username || decoded.Password != m.Password {
					t.Errorf("login request content mismatch")
				}
			case *pb.ChatMessage:
				decoded, ok := decodedMsg.(*pb.ChatMessage)
				if !ok {
					t.Fatalf("wrong message type: got %T, want *pb.ChatMessage", decodedMsg)
				}
				if decoded.Content != m.Content {
					t.Errorf("chat message content mismatch")
				}
			}
		})
	}
}
