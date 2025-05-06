package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gnet-io/gnet-examples/custom_protocol/pb"
	"github.com/gnet-io/gnet-examples/custom_protocol/serializer"
	"github.com/panjf2000/gnet/v2/pkg/logging"
	"google.golang.org/protobuf/proto"
)

const (
	maxRetries    = 5
	retryInterval = 6 * time.Second
)

type Client struct {
	network     string
	addr        string
	conn        net.Conn
	retryCount  int
	stopCh      chan struct{}
	reconnectCh chan struct{}
}

func NewClient(network, addr string) *Client {
	return &Client{
		network:     network,
		addr:        addr,
		stopCh:      make(chan struct{}),
		reconnectCh: make(chan struct{}, 1),
	}
}

func (c *Client) connect() error {
	var err error
	c.conn, err = net.Dial(c.network, c.addr)
	if err != nil {
		return err
	}
	logging.Infof("connection=%s starts...", c.conn.LocalAddr().String())
	return nil
}

func (c *Client) reconnect() {
	for {
		select {
		case <-c.stopCh:
			return
		case <-c.reconnectCh:
			if c.retryCount >= maxRetries {
				logging.Errorf("max retries reached, giving up")
				return
			}
			c.retryCount++
			logging.Infof("attempting to reconnect (attempt %d/%d)...", c.retryCount, maxRetries)
			time.Sleep(retryInterval)
			if err := c.connect(); err != nil {
				logging.Errorf("reconnect failed: %v", err)
				c.reconnectCh <- struct{}{}
				continue
			}
			c.retryCount = 0
			go c.readLoop()
		}
	}
}

func (c *Client) readLoop() {
	defer func() {
		if c.conn != nil {
			c.conn.Close()
		}
		// 确保旧 readLoop 退出后才重新连
		select {
		case c.reconnectCh <- struct{}{}:
		default:
		}
	}()

	for {
		select {
		case <-c.stopCh:
			return
		default:
			bodyBuf, err := serializer.ReadPacketHeader(c.conn)
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

			// 5. 处理不同类型的消息
			switch cmdId {
			case 1: // Login Response
				if loginResp, ok := msg.(*pb.LoginRequest); ok {
					logging.Infof("Received login response for user: %s", loginResp.Username)
				}
			case 2: // Chat Message
				if chatMsg, ok := msg.(*pb.ChatMessage); ok {
					logging.Infof("Received chat message: %s", chatMsg.Content)
				}
			default:
				logging.Infof("Received unknown command ID: %d", cmdId)
			}
		}
	}
}

func (c *Client) Start() error {
	if err := c.connect(); err != nil {
		return err
	}
	go c.readLoop()
	go c.reconnect()
	return nil
}

func (c *Client) Stop() {
	close(c.stopCh)
	if c.conn != nil {
		c.conn.Close()
	}
}

// SendMessage 发送消息
func (c *Client) SendMessage(cmdId uint32, msg proto.Message) error {
	if c.conn == nil {
		return fmt.Errorf("connection is nil")
	}

	// 序列化消息
	data, err := serializer.EncodeMessage(cmdId, msg)
	if err != nil {
		return fmt.Errorf("encode message: %w", err)
	}

	// 发送数据
	_, err = c.conn.Write(data)
	if err != nil {
		return fmt.Errorf("write message: %w", err)
	}

	return nil
}

// SendLoginRequest 发送登录请求
func (c *Client) SendLoginRequest(username, password string) error {
	req := &pb.LoginRequest{
		Username: username,
		Password: password,
	}
	return c.SendMessage(1, req)
}

// SendChatMessage 发送聊天消息
func (c *Client) SendChatMessage(content string) error {
	msg := &pb.ChatMessage{
		Content: content,
	}
	return c.SendMessage(2, msg)
}

func main() {
	var (
		network     string
		addr        string
		concurrency int
		username    string
		password    string
	)

	flag.StringVar(&network, "network", "tcp", "--network tcp")
	flag.StringVar(&addr, "address", "127.0.0.1:9999", "--address 127.0.0.1:9999")
	flag.IntVar(&concurrency, "concurrency", 2, "--concurrency 500")
	flag.StringVar(&username, "username", "test", "--username test")
	flag.StringVar(&password, "password", "test", "--password test")
	flag.Parse()

	logging.Infof("start %d clients...", concurrency)

	// 创建一个切片来存储所有客户端实例
	clients := make([]*Client, concurrency)
	var wg sync.WaitGroup
	wg.Add(concurrency)

	// 创建信号通道用于优雅退出
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	for i := 0; i < concurrency; i++ {
		go func(index int) {
			defer wg.Done()
			client := NewClient(network, addr)
			clients[index] = client // 保存客户端实例

			if err := client.Start(); err != nil {
				logging.Errorf("client start failed: %v", err)
				return
			}

			// // 发送登录请求
			if err := client.SendLoginRequest(username, password); err != nil {
				logging.Errorf("send login request failed: %v", err)
				return
			}

			// 发送测试消息
			if err := client.SendChatMessage("Hello, World!"); err != nil {
				logging.Errorf("send chat message failed: %v", err)
				return
			}
		}(i)
	}

	// 等待所有客户端启动完成
	wg.Wait()
	logging.Infof("all %d clients are started", concurrency)

	// 等待中断信号
	<-sigCh
	logging.Infof("received shutdown signal, closing clients...")

	// 优雅关闭所有客户端
	for _, client := range clients {
		if client != nil {
			client.Stop()
		}
	}

	logging.Infof("all clients have been stopped")
}
