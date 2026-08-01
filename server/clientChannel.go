package server

import (
	"slices"
	"strconv"
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
				msg += " with the password " + c.password
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
			msg += "You are authorized to control any computer connected to this channel. Authorized with password " + c.password
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
		_, exists := c.ClientsMaster[id]
		if exists {
			break
		}
		c.ClientsMaster[id] = client
	case connTypeSlave:
		_, exists := c.ClientsSlave[id]
		if exists {
			break
		}
		c.ClientsSlave[id] = client
	}
	_, exists := c.ClientsAll[id]
	if exists {
		return
	}
	c.ClientsAll[id] = client
	client.SetChannel(c)
	scdb := Data{
		Type:    "client_joined",
		Channel: c.name,
		ID:      id,
		Client: &ClientData{
			ID:             id,
			ConnectionType: connection,
		},
	}
	enc, encerr := Encode(scdb)
	if encerr == nil {
		c.sendAllLocked(enc, client)
	} else {
		Log(LOG_DEBUG, "Error encoding JSON for client "+strconv.Itoa(id)+" while trying to add them to channel "+c.name+"\r\n"+encerr.Error())
	}

	scdb.Type = "channel_joined"
	scdb.Origin = id
	scdb.ID = 0
	scdb.Client = nil
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
	enc, encerr = Encode(scdb)
	if encerr == nil {
		client.Send(enc)
	}
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
		enc, encerr = Encode(mdb)
		if encerr == nil {
			client.Send(enc)
		}
	}
	logstr := "Client " + strconv.Itoa(id) + " has joined channel " + c.name
	if connection != "" {
		logstr += " as a " + connection + ". "
		if auth {
			logstr += "This client is authorized to control other computers"
		} else {
			logstr += "This client is not authorized to control other computers"
		}
	}
	Log(LOG_CHANNEL, logstr+".")
}

func (c *ClientChannel) Remove(client *Client) {
	defer c.EndIfEmpty()
	defer c.Unlock()
	c.Lock()
	id := client.GetID()
	connection := client.GetConnectionType()
	switch connection {
	case connTypeMaster:
		_, exists := c.ClientsMaster[id]
		if !exists {
			break
		}
		delete(c.ClientsMaster, id)
	case connTypeSlave:
		_, exists := c.ClientsSlave[id]
		if !exists {
			break
		}
		delete(c.ClientsSlave, id)
	}
	_, exists := c.ClientsAll[id]
	if exists {
		delete(c.ClientsAll, id)
	}
	client.ClearChannel()
	scdb := Data{
		Type:   "client_left",
		ID:     id,
		Origin: id,
		Client: &ClientData{
			ID:             id,
			ConnectionType: connection,
		},
	}
	enc, encerr := Encode(scdb)
	if encerr == nil {
		c.sendAllLocked(enc, client)
	}
	Log(LOG_CHANNEL, "Client "+strconv.Itoa(id)+" has left channel "+c.name)
}

func (c *ClientChannel) EndIfEmpty() bool {
	c.Lock()
	if len(c.ClientsAll) > 0 {
		c.Unlock()
		return false
	}
	c.Unlock()
	c.Quit()
	return true
}

func (c *ClientChannel) Quit() {
	defer c.Unlock()
	c.Lock()
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
// Note: the original Go server only relayed master -> slaves and
// slave -> masters and blocked unauthorized masters. The Python server
// has no authorization at all (any client in the channel relays to
// everyone), so this matches Python. Locked channels are still
// protected by their full name, which the client must know.
func (c *ClientChannel) SendOthers(msg []byte, client *Client) {
	if client == nil {
		return
	}
	c.RLock()
	n := len(c.ClientsAll)
	c.RUnlock()
	if n <= 1 {
		client.Send([]byte(`{"type":"nvda_not_connected"}`))
		return
	}
	c.RLock()
	defer c.RUnlock()
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
