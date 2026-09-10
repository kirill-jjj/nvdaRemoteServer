package server

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
		Log(LogDebug, "received message without type, ignoring", "id", c.ID())
		return
	}
	if !cmd_exists(cmd) {
		Log(LogDebug, "unknown command", "command", cmd, "id", c.ID())
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
		Log(LogDebug, "JSON encoding error", "id", c.ID(), "error", encerr)
		return
	}
	c.Send(enc)
}

// sendJSON encodes arbitrary data and sends it to a client.
func sendJSON(c *Client, data Data) bool {
	enc, encerr := Encode(data)
	if encerr != nil {
		Log(LogDebug, "JSON encoding error", "id", c.ID(), "error", encerr)
		return false
	}
	c.Send(enc)
	return true
}

func init() {
	cmd_add("join", func(c *Client, db *Data) {
		if c.Channel() != nil {
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
			Log(LogDebug, "invalid version number", "id", c.ID(), "version", db.Version)
			return
		}
		c.SetVersion(db.Version)
		Log(LogDebug, "protocol version set", "id", c.ID(), "version", db.Version)
	})

	cmd_add("generate_key", func(c *Client, db *Data) {
		key, err := gen_key()
		if err != nil {
			LogError("unable to generate key", "id", c.ID(), "error", err)
			c.Close()
			return
		}
		sendJSON(c, Data{
			Type: "generate_key",
			Key:  key,
		})
		Log(LogDebug, "key generated", "id", c.ID(), "key", key)
		// Close after the queued JSON actually goes out on the wire,
		// instead of guessing with a one-second sleep.
		c.CloseGracefully()
	})
}
