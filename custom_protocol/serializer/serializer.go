package serializer

import (
	"encoding/binary"
	"fmt"

	"github.com/gnet-io/gnet-examples/custom_protocol/pb"
	"github.com/panjf2000/gnet/v2/pkg/logging"
	"google.golang.org/protobuf/proto"
)

const HeaderSize = 30

type PacketHeader struct {
	MagicID        uint32
	ClientIP       uint32
	ClientPort     uint16
	LastActiveTime int64
	CreateTime     int64
	BodyLen        uint32
}

func EncodeMessage(cmdId uint32, msg proto.Message) ([]byte, error) {
	// 序列化消息体
	body, err := proto.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal message: %w", err)
	}

	common := &pb.CommonBody{
		CmdId:  cmdId,
		PbBody: body,
	}

	commonBytes, err := proto.Marshal(common)
	if err != nil {
		return nil, fmt.Errorf("marshal common body: %w", err)
	}

	// 创建包头
	header := &PacketHeader{
		MagicID:        0x12345678,
		BodyLen:        uint32(len(commonBytes)),
		LastActiveTime: 0,
		CreateTime:     0,
	}

	// 序列化包头
	headerBuf := make([]byte, HeaderSize)
	binary.BigEndian.PutUint32(headerBuf[0:4], header.MagicID)
	binary.BigEndian.PutUint32(headerBuf[4:8], header.ClientIP)
	binary.BigEndian.PutUint16(headerBuf[8:10], header.ClientPort)
	binary.BigEndian.PutUint64(headerBuf[10:18], uint64(header.LastActiveTime))
	binary.BigEndian.PutUint64(headerBuf[18:26], uint64(header.CreateTime))
	binary.BigEndian.PutUint32(headerBuf[26:30], header.BodyLen)
	logging.Infof("headerBuf: %v, bodyLen: %d logLevel: %s", headerBuf, header.BodyLen, logging.LogLevel())
	// 组合包头和消息体
	packet := make([]byte, HeaderSize+len(commonBytes))
	copy(packet[0:HeaderSize], headerBuf)
	copy(packet[HeaderSize:], commonBytes)

	return packet, nil
}

func DecodeMessage(data []byte) (uint32, proto.Message, error) {

	common := &pb.CommonBody{}
	if err := proto.Unmarshal(data, common); err != nil {
		return 0, nil, fmt.Errorf("unmarshal common body: %w", err)
	}

	// 根据命令ID创建对应的消息类型
	var msg proto.Message
	switch common.CmdId {
	case 1:
		msg = &pb.LoginRequest{}
	case 2:
		msg = &pb.ChatMessage{}
	default:
		return common.CmdId, nil, fmt.Errorf("unknown command ID: %d", common.CmdId)
	}

	// 解析具体消息
	if err := proto.Unmarshal(common.PbBody, msg); err != nil {
		return common.CmdId, nil, fmt.Errorf("unmarshal message: %w", err)
	}

	return common.CmdId, msg, nil
}
