package server

import "time"

var command = make(map[string]func(*Client, *Data))

func cmd_exists(cmd string) bool {
	_, exists := command[cmd]
	return exists
}

func cmd_add(cmd string, cfunc func(*Client, *Data)) {
	command[cmd] = cfunc
}

// cmd_exec mirrors the Python server's parse(): a message without a
// "type" field, or with an unknown type, is silently ignored (only
// logged), never an error and never a reason to close the connection.
func cmd_exec(c *Client, db *Data) {
	cmd := db.Type
	if cmd == "" {
		Log(LOG_DEBUG, "received message without type, ignoring", "id", c.GetID())
		return
	}
	if !cmd_exists(cmd) {
		Log(LOG_DEBUG, "unknown command", "command", cmd, "id", c.GetID())
		return
	}
	command[cmd](c, db)
}

// sendError encodes an error response and sends it to a client.
func sendError(c *Client, errType string) {
	enc, encerr := Encode(Data{
		Type:  "error",
		Error: errType,
	})
	if encerr != nil {
		Log(LOG_DEBUG, "JSON encoding error", "id", c.GetID(), "error", encerr)
		return
	}
	c.Send(enc)
}

// sendJSON encodes arbitrary data and sends it to a client.
func sendJSON(c *Client, data Data) bool {
	enc, encerr := Encode(data)
	if encerr != nil {
		Log(LOG_DEBUG, "JSON encoding error", "id", c.GetID(), "error", encerr)
		return false
	}
	c.Send(enc)
	return true
}

func init() {
	cmd_add("join", func(c *Client, db *Data) {
		if c.GetChannel() != nil {
			sendError(c, "already_joined")
			return
		}
		var password string
		var locked bool
		db.Channel, password, locked = getChannelParams(db.Channel)
		if db.Channel == "" {
			sendError(c, "invalid_parameters")
			// The Python server sets canClose=True after sending the
			// invalid_parameters error, closing the connection.
			c.Close()
			return
		}

		c.SetConnectionType(db.ConnectionType)
		cc := FindChannel(db.Channel)
		if cc != nil {
			cc.Add(c, password)
			return
		}
		AddChannel(db.Channel, password, locked, c)
	})

	cmd_add("protocol_version", func(c *Client, db *Data) {
		// Python: version = obj.get('version'); if not version: return
		if db.Version <= 0 {
			Log(LOG_DEBUG, "invalid version number", "id", c.GetID(), "version", db.Version)
			return
		}
		c.SetVersion(db.Version)
		Log(LOG_DEBUG, "protocol version set", "id", c.GetID(), "version", db.Version)
	})

	cmd_add("generate_key", func(c *Client, db *Data) {
		key, err := gen_key()
		if err != nil {
			Log_error("unable to generate key", "id", c.GetID(), "error", err)
			c.Close()
			return
		}
		sendJSON(c, Data{
			Type: "generate_key",
			Key:  key,
		})
		Log(LOG_DEBUG, "key generated", "id", c.GetID(), "key", key)
		time.Sleep(time.Second)
		c.Close()
	})
}
