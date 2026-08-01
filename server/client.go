package server

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"strconv"
	"sync"
	"time"
)

var ping_msg = []byte(`{"type":"ping"}`)

const write_sec int = 8

type Client struct {
	sync.RWMutex
	conn              net.Conn
	messageTerminator byte
	connectionType    string
	id                int
	version           int
	ip                string
	c                 *ClientChannel
	auth              bool
	ctx               context.Context
	Close             context.CancelFunc
	t                 *time.Ticker
	s                 *Server
	closed            bool
	sd                chan []byte
}

func (c *Client) ClearChannel() {
	defer c.Unlock()
	c.Lock()
	c.c = nil
}

func (c *Client) SetChannel(clientChannel *ClientChannel) {
	defer c.Unlock()
	c.Lock()
	c.c = clientChannel
}

func (c *Client) GetChannel() *ClientChannel {
	defer c.RUnlock()
	c.RLock()
	return c.c
}

func (c *Client) GetID() int {
	defer c.RUnlock()
	c.RLock()
	return c.id
}

func (c *Client) SetID(id int) {
	defer c.Unlock()
	c.Lock()
	c.id = id
}

func (c *Client) GetAuthorized() bool {
	defer c.RUnlock()
	c.RLock()
	return c.auth
}

func (c *Client) GetIP() string {
	defer c.RUnlock()
	c.RLock()
	return c.ip
}

func (c *Client) GetConnectionType() string {
	defer c.RUnlock()
	c.RLock()
	return c.connectionType
}

func (c *Client) SetAuthorized(auth bool) {
	defer c.Unlock()
	c.Lock()
	c.auth = auth
}

func (c *Client) SetConnectionType(ctype string) {
	defer c.Unlock()
	c.Lock()
	c.connectionType = ctype
}

func (c *Client) GetVersion() int {
	defer c.RUnlock()
	c.RLock()
	return c.version
}

func (c *Client) SetVersion(version int) {
	defer c.Unlock()
	c.Lock()
	c.version = version
}

// Handle client data.
func (c *Client) listen() {
	idstr := strconv.Itoa(c.id)
	c.Lock()
	c.t = time.NewTicker(time.Duration(pingTime) * time.Second)
	reader := bufio.NewReader(c.conn)
	EndMessage := c.messageTerminator
	c.Unlock()
	// Send data to client. Messages are written through a buffered
	// writer: multiple queued messages are batched into fewer syscalls,
	// which matters a lot for the NVDA remote protocol with its many
	// small relayed events. The buffer is flushed immediately when the
	// queue is empty (low latency) or by a ticker while a continuous
	// stream of messages is flowing (batching). The buffered writer
	// also copies the message bytes, so the same message slice can be
	// safely handed to many clients (SendAll), which the previous
	// append(b, EndMessage) approach did not guarantee.
	c.sd = make(chan []byte, 100)
	go func() {
		bw := bufio.NewWriterSize(c.conn, 16*1024)
		flushTicker := time.NewTicker(5 * time.Millisecond)
		defer flushTicker.Stop()
		for {
			select {
			case b, ok := <-c.sd:
				if !ok {
					// Channel closed: the connection is being torn
					// down, nothing left to flush.
					return
				}
				if len(b) == 0 {
					c.Close()
					return
				}
				Log(LOG_PROTOCOL, "Data sent to client "+idstr+"\r\n"+string(b))
				_ = c.conn.SetWriteDeadline(time.Now().Add(time.Duration(write_sec) * time.Second))
				if _, err := bw.Write(b); err != nil {
					Log(LOG_DEBUG, "Error sending message to client "+idstr+".\r\n"+err.Error()+"\r\nClosing connection.")
					c.Close()
					return
				}
				_ = bw.WriteByte(EndMessage)
				if len(c.sd) == 0 {
					if err := bw.Flush(); err != nil {
						Log(LOG_DEBUG, "Error sending data to client "+idstr+".\r\n"+err.Error()+"\r\nClosing connection.")
						c.Close()
						return
					}
				}
				c.t.Reset(time.Duration(pingTime) * time.Second)
			case <-flushTicker.C:
				if err := bw.Flush(); err != nil {
					Log(LOG_DEBUG, "Error flushing data to client "+idstr+".\r\n"+err.Error()+"\r\nClosing connection.")
					c.Close()
					return
				}
			}
		}
	}()
	// Stopping and pinging our client
	go func() {
		for {
			select {
			case <-c.ctx.Done():
				msl.Lock()
				c.s.Lock()
				c.t.Stop()
				c.Lock()
				c.conn.Close()
				c.closed = true
				close(c.sd)
				c.Unlock()
				c.s.Unlock()
				msl.Unlock()
				return
			case <-c.t.C:
				c.Send(ping_msg)
			}
		}
	}()
	defer c.s.Done()
	defer RemoveClient(c)
	defer c.Close()
	// Ported from the Python server: a client must negotiate the TLS
	// connection within the configured timeout, otherwise the connection
	// is dropped. A timeout of 0 or less disables the limit.
	if timeoutSecs > 0 {
		_ = c.conn.SetDeadline(time.Now().Add(time.Duration(timeoutSecs * float64(time.Second))))
	}
	if tc, ok := c.conn.(*tls.Conn); ok {
		if err := tc.Handshake(); err != nil {
			Log(LOG_DEBUG, "TLS handshake failed for client "+idstr+".\r\n"+err.Error()+"\r\nClosing connection.")
			return
		}
	}
	_ = c.conn.SetDeadline(time.Time{})
	for {
		message, err := reader.ReadBytes(EndMessage)
		if err != nil {
			msl.Lock()
			if !stoppingServers {
				c.Lock()
				if !c.closed {
					if !errors.Is(err, io.EOF) {
						Log(LOG_DEBUG, "Error receiving message from client "+idstr+".\r\n"+err.Error()+"\r\nClosing connection.")
					}
				}
				c.Unlock()
			}
			msl.Unlock()
			return
		}
		if len(message) == 1 {
			Log(LOG_DEBUG, "Received empty message from client "+idstr)
			continue
		}
		if maxMsgLen > 0 && len(message)-1 > maxMsgLen {
			Log(LOG_DEBUG, "Received too much data from client "+idstr+", disconnecting")
			c.Close()
			return
		}
		message = bytes.TrimRight(message, string(EndMessage))
		Log(LOG_PROTOCOL, "Data received from client "+idstr+"\r\n"+string(message))
		MessageReceived(c, message)
	}
}

// Send bytes to client.
func (c *Client) Send(b []byte) {
	defer func() {
		if r := recover(); r != nil {
			c.Close()
		}
	}()
	c.RLock()
	if c.closed {
		c.RUnlock()
		return
	}
	c.RUnlock()
	if len(b) == 0 {
		return
	}
	c.sd <- b
}
