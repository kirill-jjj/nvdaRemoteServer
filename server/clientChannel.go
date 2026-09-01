package server

import (
	"slices"
	"strings"
	"sync"
)

const (
	connTypeSlave  string = "slave"
	connTypeMaster string = "master"
)

type ClientChannel struct {
	sync.RWMutex
	name          string
	password      string
	locked        bool
	ClientsAll    map[int]*Client
	ClientsMaster map[int]*Client
	ClientsSlave  map[int]*Client
}

func (c *ClientChannel) Lmotd(ctype, name, password string) string {
	msg := "This is a locked channel. Name: " + name + "\n"
	switch ctype {
	case connTypeSlave:
		msg += "No one will be able to control your computer"
		if c.password != "" {
			msg += " unless they authenticate"
			if password == c.password {
				msg += " (password accepted)"
			} else {
				msg += "."
			}
		} else {
			msg += "."
		}
	case connTypeMaster:
		if (c.password != "" && password != c.password) || (c.password == "") {
			msg += "You won't be able to control any computers connected to this channel."
		}
		if c.password == password && c.password != "" {
			msg += "You are authorized to control any computer connected to this channel."
		}
	}
	if !c.locked {
		return ""
	} else {
		return msg
	}
}

func (c *ClientChannel) Add(client *Client, password string) {
	defer c.Unlock()
	c.Lock()
	auth := false
	client.SetAuthorized(false)
	id := client.GetID()
	connection := client.GetConnectionType()
	if c.locked {
		if password == c.password && c.password != "" {
			client.SetAuthorized(true)
			auth = true
		}
	} else {
		client.SetAuthorized(true)
		auth = true
	}
	clients := c.ClientsAll
	lmotd := c.Lmotd(connection, c.name, password)
	switch connection {
	case connTypeMaster:
		if _, exists := c.ClientsMaster[id]; !exists {
			c.ClientsMaster[id] = client
		}
	case connTypeSlave:
		if _, exists := c.ClientsSlave[id]; !exists {
			c.ClientsSlave[id] = client
		}
	}
	if _, exists := c.ClientsAll[id]; exists {
		return
	}
	c.ClientsAll[id] = client
	client.SetChannel(c)

	// Send client_joined to all other clients in the channel.
	c.sendJSON(client, Data{
		Type:    "client_joined",
		Channel: c.name,
		ID:      id,
		Client: &ClientData{
			ID:             id,
			ConnectionType: connection,
		},
	})

	// Build channel_joined response for the joining client.
	scdb := Data{
		Type:    "channel_joined",
		Channel: c.name,
		Origin:  id,
	}
	if len(clients) > 0 {
		scdb.UserIds = make([]int, 0, len(clients))
		scdb.Clients = make([]ClientData, 0, len(clients))
		var ctype string
		for cid, cc := range clients {
			if cid == id {
				continue
			}
			ctype = cc.GetConnectionType()
			scdb.UserIds = append(scdb.UserIds, cid)
			scdb.Clients = append(scdb.Clients, ClientData{
				ID:             cid,
				ConnectionType: ctype,
			})
		}
		if len(scdb.UserIds) == 0 {
			scdb.UserIds = nil
			scdb.Clients = nil
		} else if len(scdb.UserIds) > 1 {
			slices.Sort(scdb.UserIds)
			slices.SortStableFunc(scdb.Clients,
				func(a, b ClientData) int {
					return a.ID - b.ID
				})
		}
	}
	c.sendToClient(client, scdb)

	// Send MOTD if configured.
	if motd != "" || lmotd != "" {
		mdb := Data{
			Type:              "motd",
			Motd:              motd,
			MotdAlwaysDisplay: motdAlwaysDisplay,
		}
		if lmotd != "" {
			if mdb.Motd == "" {
				mdb.Motd = lmotd
			} else {
				mdb.Motd = lmotd + "\n" + mdb.Motd
			}
			mdb.MotdAlwaysDisplay = true
		}
		c.sendToClient(client, mdb)
	}

	Log(LOG_CHANNEL, "client joined channel",
		"id", id,
		"channel", c.name,
		"connection_type", connection,
		"authorized", auth,
	)
}

func (c *ClientChannel) Remove(client *Client) {
	defer c.Unlock()
	c.Lock()
	id := client.GetID()
	connection := client.GetConnectionType()
	switch connection {
	case connTypeMaster:
		delete(c.ClientsMaster, id)
	case connTypeSlave:
		delete(c.ClientsSlave, id)
	}
	delete(c.ClientsAll, id)
	client.ClearChannel()

	// Send client_left to remaining clients in the channel.
	c.sendAllLocked(c.mustEncode(Data{
		Type:   "client_left",
		ID:     id,
		Origin: id,
		Client: &ClientData{
			ID:             id,
			ConnectionType: connection,
		},
	}), client)

	Log(LOG_CHANNEL, "client left channel",
		"id", id,
		"channel", c.name,
	)

	// Check if channel is empty after removal. Previously this was
	// split into EndIfEmpty() + Quit() with an unlock/relock in
	// between, which created a TOCTOU race. Now the check and
	// cleanup happen under the same lock.
	if len(c.ClientsAll) == 0 {
		c.quitLocked()
	}
}

// quitLocked removes all clients from the channel and deletes it from
// the global map. Must be called with c.Lock() held.
func (c *ClientChannel) quitLocked() {
	if c.ClientsAll == nil {
		RemoveChannel(c.name)
		return
	}
	for id, client := range c.ClientsAll {
		delete(c.ClientsMaster, id)
		delete(c.ClientsSlave, id)
		client.ClearChannel()
	}
	c.ClientsAll = nil
	c.ClientsMaster = nil
	c.ClientsSlave = nil
	RemoveChannel(c.name)
}

// sendAllLocked relays msg to every client in the channel except the
// excluded one. The caller must hold the channel's write lock (Add and
// Remove call it from inside their Lock section).
func (c *ClientChannel) sendAllLocked(msg []byte, client *Client) {
	if len(c.ClientsAll) == 0 {
		return
	}
	for _, sc := range c.ClientsAll {
		if client != nil && client == sc {
			continue
		}
		sc.Send(msg)
	}
}

// SendAll relays msg to every client in the channel except the
// excluded one. Taking the read lock here fixes a data race: the raw
// relay path (MessageReceived with non-JSON data) used to iterate the
// client map without any lock while a concurrent join or disconnect
// mutated it, which can panic with "concurrent map read and map write".
func (c *ClientChannel) SendAll(msg []byte, client *Client) {
	c.RLock()
	defer c.RUnlock()
	c.sendAllLocked(msg, client)
}

// SendOthers relays a message to every other client in the channel,
// mirroring the Python server's parse(): when the sender is the only
// client in the channel they get an "nvda_not_connected" notice,
// otherwise the message goes to everyone except the sender.
//
// The previous implementation acquired RLock twice (once for the len
// check, once for the loop), creating a TOCTOU race where the channel
// size could change between the two lock acquisitions. This version
// holds a single RLock for both the check and the iteration.
func (c *ClientChannel) SendOthers(msg []byte, client *Client) {
	if client == nil {
		return
	}
	c.RLock()
	defer c.RUnlock()
	if len(c.ClientsAll) <= 1 {
		client.Send(not_connected_msg)
		return
	}
	for _, sc := range c.ClientsAll {
		if sc == client {
			continue
		}
		sc.Send(msg)
	}
}

func (c *ClientChannel) Name() string {
	c.RLock()
	defer c.RUnlock()
	return c.name
}

// sendJSON encodes data and sends it to every client except the
// excluded one. This replaces the repeated encode-then-check pattern
// that was duplicated across Add, Remove, and other methods.
func (c *ClientChannel) sendJSON(exclude *Client, data Data) {
	c.sendAllLocked(c.mustEncode(data), exclude)
}

// sendToClient encodes data and sends it to a single client.
func (c *ClientChannel) sendToClient(client *Client, data Data) {
	enc := c.mustEncode(data)
	if enc != nil {
		client.Send(enc)
	}
}

// mustEncode encodes data to JSON, logging and returning nil on error.
// This replaces the repeated pattern:
//
//	enc, encerr := Encode(data)
//	if encerr == nil {
//	    ...
//	} else {
//	    Log(LOG_DEBUG, "Error encoding JSON...")
//	}
func (c *ClientChannel) mustEncode(data Data) []byte {
	enc, err := Encode(data)
	if err != nil {
		Log(LOG_DEBUG, "error encoding JSON", "channel", c.name, "error", err)
		return nil
	}
	return enc
}

func NewClientChannel(name, password string, locked bool, client *Client) *ClientChannel {
	c := &ClientChannel{
		name:          name,
		locked:        locked,
		password:      password,
		ClientsAll:    make(map[int]*Client),
		ClientsMaster: make(map[int]*Client),
		ClientsSlave:  make(map[int]*Client),
	}
	c.Add(client, password)
	return c
}

func getChannelParams(name string) (string, string, bool) {
	password := ""
	locked := false
	fl := "lock_"
	fp := "__password__"
	li := strings.Index(name, fl)
	pi := strings.Index(name, fp)
	if li == -1 && pi == -1 {
		return name, password, locked
	}
	if li == 0 {
		name = name[len(fl):]
		locked = true
		pi = strings.Index(name, fp)
	}
	if pi > 0 {
		password = name[(pi + len(fp)):]
		name = name[:pi]
		locked = true
	}
	return name, password, locked
}
