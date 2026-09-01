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

var (
	ping_msg          = []byte(`{"type":"ping"}`)
	not_connected_msg = []byte(`{"type":"nvda_not_connected"}`)
)

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

// logClientError logs a client error with the standard format:
// "Error <context> to client <id>.\r\n<err>\r\nClosing connection."
// This replaces the 5 duplicated string concatenation patterns in
// the writer and listener goroutines.
func (c *Client) logClientError(context string, err error) {
	Log(LOG_DEBUG, "Error "+context+" to client "+strconv.Itoa(c.id)+".\r\n"+err.Error()+"\r\nClosing connection.")
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
					c.logClientError("sending message", err)
					c.Close()
					return
				}
				_ = bw.WriteByte(EndMessage)
				if len(c.sd) == 0 {
					if err := bw.Flush(); err != nil {
						c.logClientError("sending data", err)
						c.Close()
						return
					}
				}
				c.t.Reset(time.Duration(pingTime) * time.Second)
			case <-flushTicker.C:
				if err := bw.Flush(); err != nil {
					c.logClientError("flushing data", err)
					c.Close()
					return
				}
			}
		}
	}()
	// Stopping and pinging our client.
	// msl (main server lock) was removed here: context.Context is
	// already thread-safe for Done() checks, and logging has its own
	// mutex. This eliminates the triple-lock nesting
	// (msl → server → client) that was a deadlock risk.
	go func() {
		for {
			select {
			case <-c.ctx.Done():
				c.s.Lock()
				c.t.Stop()
				c.Lock()
				c.conn.Close()
				c.closed = true
				close(c.sd)
				c.Unlock()
				c.s.Unlock()
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
			c.logClientError("TLS handshake", err)
			return
		}
	}
	_ = c.conn.SetDeadline(time.Time{})
	for {
		message, err := reader.ReadBytes(EndMessage)
		if err != nil {
			// Mctx.Err() is thread-safe — no msl needed.
			if Mctx.Err() == nil {
				c.Lock()
				if !c.closed {
					if !errors.Is(err, io.EOF) {
						c.logClientError("receiving message", err)
					}
				}
				c.Unlock()
			}
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
		// Use a string literal "\n" instead of string(EndMessage)
		// to avoid an allocation per message. The byte-to-string
		// conversion allocates a new string on every call, while a
		// string literal is a compile-time constant.
		message = bytes.TrimRight(message, "\n")
		Log(LOG_PROTOCOL, "Data received from client "+idstr+"\r\n"+string(message))
		MessageReceived(c, message)
	}
}

// Send bytes to client.
// Sends are buffered through c.sd (capacity 100). The send is
// non-blocking with three cases:
//
//  1. c.sd <- b:   channel has space, send succeeds.
//  2. c.ctx.Done(): client is shutting down, drop the message.
//     Context is always cancelled BEFORE c.sd is closed (see
//     shutdown goroutine), so this case fires first and avoids
//     the send-on-closed-channel panic in almost all cases.
//  3. default:      channel full (client too slow), drop the message.
//
// The recover() is a safety net for the extremely rare race where
// c.sd is closed between select evaluation and the actual send.
//
// This design prevents a slow client from freezing the entire
// channel: without non-blocking send, Remove → sendAllLocked →
// Send would block on a full channel while holding the channel lock.
func (c *Client) Send(b []byte) {
	if len(b) == 0 {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			c.Close()
		}
	}()
	select {
	case c.sd <- b:
	case <-c.ctx.Done():
		// Client is shutting down — drop rather than risk
		// sending on a channel that will be closed momentarily.
	default:
		// Channel full — client is too slow or disconnected.
		// Drop the message rather than blocking the sender.
	}
}
