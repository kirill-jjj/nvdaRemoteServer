package server

import (
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
)

var (
	sl         sync.RWMutex
	EndMessage byte = '\n'
	lastID     atomic.Int32
	clients    map[*Client]struct{}
	channels   map[string]*ClientChannel
)

func AddClient(c *Client) {
	sl.Lock()
	defer sl.Unlock()
	id := int(lastID.Add(1))
	c.SetID(id)
	if clients == nil {
		clients = make(map[*Client]struct{})
	}
	clients[c] = struct{}{}
	Log(LOG_CONNECTION, "Client "+strconv.Itoa(id)+" has connected from "+c.GetIP())
}

func FindClient(c *Client) bool {
	sl.RLock()
	defer sl.RUnlock()
	if clients == nil {
		return false
	}
	_, exists := clients[c]
	return exists
}

func RemoveClient(c *Client) {
	if !FindClient(c) {
		Log(LOG_DEBUG, "Client "+strconv.Itoa(c.GetID())+" is already disconnected.")
		return
	}
	cc := c.GetChannel()
	if cc != nil {
		cc.Remove(c)
	}
	sl.Lock()
	defer sl.Unlock()
	Log(LOG_CONNECTION, "Client "+strconv.Itoa(c.GetID())+" has disconnected.")
	delete(clients, c)
	if len(clients) == 0 {
		clients = nil
		Log(LOG_DEBUG, "There are no clients connected to the server.")
	}
}

func AddChannel(name, password string, locked bool, c *Client) {
	sl.Lock()
	defer sl.Unlock()
	if channels == nil {
		channels = make(map[string]*ClientChannel)
	}
	logstr := "Channel " + name + " has been created."
	if locked {
		logstr += " This is a locked channel. "
		if password != "" {
			logstr += "Clients can control a computer with the password " + password
		} else {
			logstr += "No computers can be controlled on this channel."
		}
	}
	Log(LOG_CHANNEL, logstr)
	cc := NewClientChannel(name, password, locked, c)
	channels[name] = cc
}

func FindChannel(name string) *ClientChannel {
	sl.RLock()
	defer sl.RUnlock()
	if channels == nil {
		return nil
	}
	c, exists := channels[name]
	if !exists {
		return nil
	}
	return c
}

func RemoveChannel(name string) {
	sl.Lock()
	defer sl.Unlock()
	if channels == nil {
		return
	}
	if _, exists := channels[name]; !exists {
		return
	}
	delete(channels, name)
	Log(LOG_CHANNEL, "Channel "+name+" has been removed.")
	if len(channels) == 0 {
		channels = nil
		Log(LOG_DEBUG, "There are no channels on the server.")
	}
}

func MessageReceived(c *Client, pmsg []byte) {
	id := c.GetID()
	if !FindClient(c) {
		Log_error("A client object was not found from the connection receiving a message, number " + strconv.Itoa(id) + ". Unexpected behavior encountered. Closing connection.")
		runtime.Goexit()
	}
	cc := c.GetChannel()
	if cc != nil {
		// The origin field is always added, mirroring the Python
		// server, which has no option to disable it.
		pmsg, err := JsonAdd(pmsg, "origin", id)
		if err != nil {
			Log(LOG_DEBUG, "Error adding origin to message from client "+strconv.Itoa(id)+".\r\n"+err.Error()+"\r\nSending to all clients without origin field.")
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
		Log(LOG_DEBUG, "Unable to parse message from client "+strconv.Itoa(id)+", ignoring.\r\n"+err.Error())
		return
	}
	cmd_exec(c, &decode)
}
