// Package server implements the NVDA Remote relay server: it accepts
// TLS and plain TCP connections from NVDA Remote clients, groups them
// into named channels, and relays protocol messages between the
// clients of each channel. The wire protocol, default ports, and
// configuration file format are compatible with the original Python
// NVDARemoteServer.
package server

import (
	"runtime"
	"sync"
	"sync/atomic"
)

var (
	EndMessage byte = '\n'
	lastID     atomic.Int32
	clients    = make(map[*Client]struct{})
	channels   = make(map[string]*ClientChannel)
)

// mu protects the global clients and channels maps. All public functions
// that read or write these maps must hold mu. The previous design also
// used a separate msl ("main server lock") to gate access to
// Mctx.Err() checks and to serialize accept/shutdown paths. This was
// removed because context.Context is already safe for concurrent use
// and logging has its own mutex (ll in logger.go). Removing msl
// eliminates the triple-lock nesting (msl → server → client) that was
// a deadlock risk.
var mu sync.RWMutex

func AddClient(c *Client) {
	mu.Lock()
	defer mu.Unlock()
	id := int(lastID.Add(1))
	c.SetID(id)
	clients[c] = struct{}{}
	Log(LogConnection, "client connected", "id", id, "ip", c.IP())
}

func FindClient(c *Client) bool {
	mu.RLock()
	defer mu.RUnlock()
	_, exists := clients[c]
	return exists
}

// RemoveClient removes a client from the global client map and its
// channel. The existence check and deletion happen under a single lock
// acquisition to avoid TOCTOU (time-of-check/time-of-use) races.
func RemoveClient(c *Client) {
	cc := c.Channel()
	if cc != nil {
		cc.Remove(c)
	}
	mu.Lock()
	defer mu.Unlock()
	id := c.ID()
	if _, exists := clients[c]; !exists {
		Log(LogDebug, "client already disconnected", "id", id)
		return
	}
	Log(LogConnection, "client disconnected", "id", id)
	delete(clients, c)
	if len(clients) == 0 {
		Log(LogDebug, "no clients connected to server")
	}
}

func AddChannel(name, password string, locked bool, c *Client) {
	mu.Lock()
	defer mu.Unlock()
	if locked {
		Log(LogChannel, "channel created (locked)", "name", name, "has_password", password != "")
	} else {
		Log(LogChannel, "channel created", "name", name)
	}
	cc := NewClientChannel(name, password, locked, c)
	channels[name] = cc
}

func FindChannel(name string) *ClientChannel {
	mu.RLock()
	defer mu.RUnlock()
	c, exists := channels[name]
	if !exists {
		return nil
	}
	return c
}

func RemoveChannel(name string) {
	mu.Lock()
	defer mu.Unlock()
	if _, exists := channels[name]; !exists {
		return
	}
	delete(channels, name)
	Log(LogChannel, "channel removed", "name", name)
	if len(channels) == 0 {
		Log(LogDebug, "no channels on server")
	}
}

func MessageReceived(c *Client, pmsg []byte) {
	id := c.ID()
	if !FindClient(c) {
		LogError("client not found in connection map", "id", id)
		runtime.Goexit()
	}
	cc := c.Channel()
	if cc != nil {
		// The origin field is always added, mirroring the Python
		// server, which has no option to disable it.
		pmsg, err := JsonAddOrigin(pmsg, id)
		if err != nil {
			Log(LogDebug, "error adding origin to message", "id", id, "error", err)
			// Non-JSON data (raw NVDA remote protocol messages)
			// are relayed to every client in the channel, mirroring
			// the Python server's send_data_to_others behavior.
			cc.SendAll(pmsg, c)
			return
		}
		cc.SendOthers(pmsg, c)
		return
	}
	// Not in a channel yet: parse and dispatch. Mirrors Python's
	// parse(): data that doesn't parse as JSON, messages without a
	// "type", and unknown commands are all ignored without closing
	// the connection (the Python server's send_data_to_others just
	// finds nobody with the same password and returns).
	decode, err := Decode(pmsg)
	if err != nil {
		Log(LogDebug, "unable to parse message from client, ignoring", "id", id, "error", err)
		return
	}
	cmd_exec(c, &decode)
}
