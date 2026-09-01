/*
MIT License

Copyright (c) 2016 firstrow@gmail.com

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
*/

package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

// TCP server.
type Server struct {
	sync.Mutex
	sync.WaitGroup
	address           string // Address to open connection: localhost:9999
	config            *tls.Config
	messageTerminator byte
	ctx               context.Context
	Stop              context.CancelFunc
}

var (
	Mctx        context.Context
	StopServers context.CancelFunc
)

// init wires OS signals (Ctrl+C, SIGTERM, SIGQUIT) straight into the
// global context via signal.NotifyContext: when the operator stops the
// server (keyboard interrupt, "kill", or the stop/restart daemon
// commands sending SIGTERM), Mctx is cancelled automatically and every
// server and client goroutine that derived its context from Mctx starts
// shutting down.
func init() {
	Mctx, StopServers = signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
}

// Set message terminator.
func (s *Server) MessageTerminator(terminator byte) {
	s.Lock()
	defer s.Unlock()
	s.messageTerminator = terminator
}

// Listen starts network server.
func (s *Server) Listen() error {
	s.Lock()
	var listener net.Listener
	var err error
	config := s.config
	address := s.address
	s.Unlock()
	if config == nil {
		var lc net.ListenConfig
		listener, err = lc.Listen(s.ctx, "tcp", address)
	} else {
		listener, err = tls.Listen("tcp", address, config)
	}
	if err != nil {
		return fmt.Errorf("listening on %s: %w", address, err)
	}
	s.Lock()
	s.ctx, s.Stop = context.WithCancel(Mctx)
	s.Add(1)
	s.Unlock()
	go s.accept(listener)
	return nil
}

func (s *Server) accept(listener net.Listener) {
	s.Lock()
	address := s.address
	s.Unlock()
	// Stopping our server.
	go func() {
		<-s.ctx.Done()
		// Mctx.Err() is thread-safe (context.Context guarantees safe
		// concurrent access), so no extra mutex is needed here.
		if Mctx.Err() == nil {
			Log(LOG_DEBUG, "server received stop signal", "address", address)
		}
		listener.Close()
		s.Done()
	}()
	for {
		conn, err := listener.Accept()
		if err != nil {
			// Mctx.Err() is thread-safe — no msl needed.
			if Mctx.Err() == nil {
				Log_error("error accepting connections", "address", address, "error", err)
			}
			s.Stop()
			break
		}
		// Ported from the Python server: disable Nagle's algorithm on
		// client sockets to reduce latency for remote control traffic.
		// TCP keepalive: the kernel probes silent connections every 60s
		// and drops the ones whose peer vanished without FIN/RST (laptop
		// closed, network gone). Without it, a half-dead client stays in
		// its channel forever: the reader blocks on ReadBytes and nothing
		// ever writes to the client to trip the write deadline.
		if tc, ok := conn.(*tls.Conn); ok {
			if t, ok2 := tc.NetConn().(*net.TCPConn); ok2 {
				setupTCP(t)
			}
		} else if t, ok := conn.(*net.TCPConn); ok {
			setupTCP(t)
		}
		// Only s.Lock() is needed now — msl was removed because
		// context.Context and logging are both thread-safe.
		s.Lock()
		client := &Client{
			conn:              conn,
			ip:                getIP(conn),
			s:                 s,
			messageTerminator: s.messageTerminator,
			closed:            false,
		}
		client.ctx, client.Close = context.WithCancel(s.ctx)
		s.Add(1)
		AddClient(client)
		s.Unlock()
		go client.listen()
	}
}

// setupTCP applies socket-level options shared by every accepted
// connection: disable Nagle's algorithm (low latency for the remote
// control traffic) and enable TCP keepalive with a 60s probe period.
// Keepalive is what eventually detects half-dead peers; combined with
// the 8s write deadline it guarantees a vanished client is removed
// from its channel even if nobody ever writes to it.
func setupTCP(t *net.TCPConn) {
	_ = t.SetNoDelay(true)
	_ = t.SetKeepAlive(true)
	_ = t.SetKeepAlivePeriod(60 * time.Second)
}

// Creates new tcp server instance.
func New(address string) *Server {
	server := &Server{
		address:           address,
		messageTerminator: '\n',
	}

	return server
}

func NewWithTLSConfig(address string, config *tls.Config) *Server {
	server := New(address)
	server.config = config
	return server
}

func getIP(c net.Conn) string {
	ip, _, err := net.SplitHostPort(c.RemoteAddr().String())
	if err != nil {
		return ""
	}
	return strings.Trim(ip, "[]")
}
