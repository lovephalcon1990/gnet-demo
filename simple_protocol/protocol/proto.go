package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"

	"github.com/panjf2000/gnet/v2"
)

var ErrIncompletePacket = errors.New("incomplete packet")

const (
	magicNumber     = 1314
	magicNumberSize = 2
	bodySize        = 4
)

var magicNumberBytes []byte

func init() {
	magicNumberBytes = make([]byte, magicNumberSize)
	binary.BigEndian.PutUint16(magicNumberBytes, uint16(magicNumber))
}

// SimpleCodec Protocol format:
//
// * 0           2                       6
// * +-----------+-----------------------+
// * |   magic   |       body len        |
// * +-----------+-----------+-----------+
// * |                                   |
// * +                                   +
// * |           body bytes              |
// * +                                   +
// * |            ... ...                |
// * +-----------------------------------+
type SimpleCodec struct{}

func (codec SimpleCodec) Encode(buf []byte) ([]byte, error) {
	bodyOffset := magicNumberSize + bodySize
	msgLen := bodyOffset + len(buf)

	data := make([]byte, msgLen)
	copy(data, magicNumberBytes)

	binary.BigEndian.PutUint32(data[magicNumberSize:bodyOffset], uint32(len(buf)))
	copy(data[bodyOffset:msgLen], buf)
	return data, nil
}

func (codec SimpleCodec) Decode(c gnet.Conn) ([]byte, error) {
	bodyOffset := magicNumberSize + bodySize
	buf, err := c.Peek(bodyOffset)
	if err != nil {
		if errors.Is(err, io.ErrShortBuffer) {
			err = ErrIncompletePacket
		}
		return nil, err
	}

	if !bytes.Equal(magicNumberBytes, buf[:magicNumberSize]) {
		return nil, errors.New("invalid magic number")
	}

	bodyLen := binary.BigEndian.Uint32(buf[magicNumberSize:bodyOffset])
	msgLen := bodyOffset + int(bodyLen)
	buf, err = c.Peek(msgLen)
	if err != nil {
		if errors.Is(err, io.ErrShortBuffer) {
			err = ErrIncompletePacket
		}
		return nil, err
	}
	body := make([]byte, bodyLen)
	copy(body, buf[bodyOffset:msgLen])
	_, _ = c.Discard(msgLen)

	return body, nil
}

func (codec SimpleCodec) Unpack(buf []byte) ([]byte, error) {
	bodyOffset := magicNumberSize + bodySize
	if len(buf) < bodyOffset {
		return nil, ErrIncompletePacket
	}

	if !bytes.Equal(magicNumberBytes, buf[:magicNumberSize]) {
		return nil, errors.New("invalid magic number")
	}

	bodyLen := binary.BigEndian.Uint32(buf[magicNumberSize:bodyOffset])
	msgLen := bodyOffset + int(bodyLen)
	if len(buf) < msgLen {
		return nil, ErrIncompletePacket
	}

	return buf[bodyOffset:msgLen], nil
}
