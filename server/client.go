package server

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ping_msg          = []byte(`{"type":"ping"}`)
	not_connected_msg = []byte(`{"type":"nvda_not_connected"}`)
)

const writeSec int = 8

// dropKickInterval is how many dropped messages a client may accumulate
// before the server concludes it is hopelessly behind and disconnects
// it. With a queue capacity of 100, a live but slow client absorbs
// bursts without ever reaching this; a client that has stopped reading
// hits it quickly. 500 total drops ≈ 5 full queue overflows.
const dropKickInterval uint32 = 500

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
	dropped           atomic.Uint32 // messages dropped due to a full queue
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

func (c *Client) Channel() *ClientChannel {
	defer c.RUnlock()
	c.RLock()
	return c.c
}

func (c *Client) ID() int {
	defer c.RUnlock()
	c.RLock()
	return c.id
}

func (c *Client) SetID(id int) {
	defer c.Unlock()
	c.Lock()
	c.id = id
}

func (c *Client) Authorized() bool {
	defer c.RUnlock()
	c.RLock()
	return c.auth
}

func (c *Client) IP() string {
	defer c.RUnlock()
	c.RLock()
	return c.ip
}

func (c *Client) ConnectionType() string {
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

func (c *Client) Version() int {
	defer c.RUnlock()
	c.RLock()
	return c.version
}

func (c *Client) SetVersion(version int) {
	defer c.Unlock()
	c.Lock()
	c.version = version
}

// logClientError logs a client error with structured data.
func (c *Client) logClientError(context string, err error) {
	Log(LogDebug, "client error", "context", context, "id", c.id, "error", err)
}

// Handle client data.
//
//nolint:gocyclo // Main event loop: complexity is inherent to the protocol handling.
func (c *Client) listen() {
	idstr := c.id
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
		// Batching timer, armed only when data has been written to bw
		// but not yet flushed (queue was non-empty). An idle client
		// costs zero timer wakeups — unlike an always-on Ticker, which
		// fires every 5ms per client regardless of activity.
		flushTimer := time.NewTimer(0)
		if !flushTimer.Stop() {
			<-flushTimer.C
		}
		defer flushTimer.Stop()
		for {
			select {
			case b, ok := <-c.sd:
				if !ok {
					// Channel closed: the connection is being torn
					// down, nothing left to flush.
					return
				}
				if len(b) == 0 {
					// Sentinel close marker: everything queued before
					// it has already been written to the buffered
					// writer, so flush it before tearing down.
					_ = bw.Flush()
					c.Close()
					return
				}
				Log(LogProtocol, "data sent to client", "id", idstr, "data", string(b))
				_ = c.conn.SetWriteDeadline(time.Now().Add(time.Duration(writeSec) * time.Second))
				if _, err := bw.Write(b); err != nil {
					c.logClientError("sending message", err)
					c.Close()
					return
				}
				_ = bw.WriteByte(EndMessage)
				if len(c.sd) == 0 {
					// Queue drained: flush now for lowest latency and
					// disarm the batch timer if one was armed.
					if err := bw.Flush(); err != nil {
						c.logClientError("sending data", err)
						c.Close()
						return
					}
					if !flushTimer.Stop() {
						select {
						case <-flushTimer.C:
						default:
						}
					}
				} else if !flushTimer.Stop() {
					// More messages queued: make sure the batch timer is
					// armed (arm only once per burst, not per message).
					// Stop returned false: either it already fired and its
					// value still sits in C (drain it) or it was already
					// stopped/drained (nothing to drain). Either way a
					// fresh Reset is then safe.
					select {
					case <-flushTimer.C:
					default:
					}
					flushTimer.Reset(5 * time.Millisecond)
				}
				c.t.Reset(time.Duration(pingTime) * time.Second)
			case <-flushTimer.C:
				// Batch window elapsed: push the accumulated burst out
				// in one syscall.
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
		if err := tc.HandshakeContext(c.ctx); err != nil {
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
			Log(LogDebug, "received empty message from client", "id", idstr)
			continue
		}
		if maxMsgLen > 0 && len(message)-1 > maxMsgLen {
			Log(LogDebug, "received too much data from client, disconnecting", "id", idstr)
			c.Close()
			return
		}
		// Use a string literal "\n" instead of string(EndMessage)
		// to avoid an allocation per message. The byte-to-string
		// conversion allocates a new string on every call, while a
		// string literal is a compile-time constant.
		message = bytes.TrimRight(message, "\n")
		Log(LogProtocol, "data received from client", "id", idstr, "data", string(message))
		MessageReceived(c, message)
	}
}

// CloseGracefully queues an empty-slice sentinel after any pending
// messages. The writer goroutine flushes the buffered writer and only
// then closes the connection, guaranteeing FIFO delivery of everything
// sent before the close request — unlike a bare Close(), which can
// race with messages still sitting in c.sd or the write buffer.
func (c *Client) CloseGracefully() {
	select {
	case c.sd <- []byte{}:
		// Sentinel queued; the writer flushes and closes in order.
	case <-c.ctx.Done():
		// Already shutting down; the teardown goroutine owns the close.
	default:
		// Queue full (client too slow): nothing more will get out
		// in reasonable time anyway, so close immediately.
		c.Close()
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
//  3. default:      channel full (client too slow), drop the message
//     and count the drop.
//
// The recover() is a safety net for the extremely rare race where
// c.sd is closed between select evaluation and the actual send.
//
// This design prevents a slow client from freezing the entire
// channel: without non-blocking send, Remove → sendAllLocked →
// Send would block on a full channel while holding the channel lock.
//
// Dropping is safe for transient spikes (a momentary burst is absorbed
// by later traffic), but a queue that stays full means the client has
// stopped reading entirely — for a screen-reader relay every dropped
// message is a lost key event, so a client that keeps overflowing is
// disconnected instead of being fed garbage (see dropKickInterval).
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
		// Drop the message rather than blocking the sender,
		// and remember it: repeated overflow kills the client.
		n := c.dropped.Add(1)
		if n == 1 {
			Log(LogDebug, "send queue full, dropping messages", "id", c.ID())
		}
		if n%dropKickInterval == 0 {
			LogError("client queue hopelessly behind, disconnecting",
				"id", c.ID(), "dropped_total", n)
			c.Close()
		}
	}
}
