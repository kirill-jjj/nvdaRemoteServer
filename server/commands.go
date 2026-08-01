package server

import (
	"strconv"
	"time"
)

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
// The Python server doesn't know the do_<cmd> methods either; it just
// does nothing, so new NVDA Remote client commands won't kill
// established connections.
func cmd_exec(c *Client, db *Data) {
	cmd := db.Type
	if cmd == "" {
		Log(LOG_DEBUG, "Received message without a type from client "+strconv.Itoa(c.GetID())+", ignoring")
		return
	}
	if !cmd_exists(cmd) {
		Log(LOG_DEBUG, "Unknown command "+cmd+" from client "+strconv.Itoa(c.GetID())+", ignoring")
		return
	}
	command[cmd](c, db)
}

func init() {
	cmd_add("join", func(c *Client, db *Data) {
		if c.GetChannel() != nil {
			enc, encerr := Encode(Data{
				Type:  "error",
				Error: "already_joined",
			})
			if encerr == nil {
				c.Send(enc)
				return
			} else {
				Log(LOG_DEBUG, "JSON encoding error for client "+strconv.Itoa(c.GetID())+"\r\n"+encerr.Error())
				return
			}
		}
		var password string
		var locked bool
		db.Channel, password, locked = getChannelParams(db.Channel)
		if db.Channel == "" {
			enc, encerr := Encode(Data{
				Type:  "error",
				Error: "invalid_parameters",
			})
			if encerr == nil {
				c.Send(enc)
			} else {
				Log(LOG_DEBUG, "JSON encoding error for client "+strconv.Itoa(c.GetID())+"\r\n"+encerr.Error())
			}
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
			Log(LOG_DEBUG, "Client "+strconv.Itoa(c.GetID())+" has tried to register an invalid version number, ignoring")
			return
		}
		c.SetVersion(db.Version)
		Log(LOG_DEBUG, "Client "+strconv.Itoa(c.GetID())+" has set protocol version "+strconv.Itoa(db.Version)+".")
	})

	cmd_add("generate_key", func(c *Client, db *Data) {
		key, err := gen_key()
		if err != nil {
			Log_error("Unable to generate a key for client " + strconv.Itoa(c.GetID()) + "\r\n" + err.Error() + "\r\nClosing connection.")
			c.Close()
			return
		}
		enc, encerr := Encode(Data{
			Type: "generate_key",
			Key:  key,
		})
		if encerr != nil {
			Log(LOG_DEBUG, "JSON encoding error for client "+strconv.Itoa(c.GetID()))
			return
		}
		c.Send(enc)
		Log(LOG_DEBUG, "Client "+strconv.Itoa(c.GetID())+" has generated a key: "+key)
		time.Sleep(time.Second)
		c.Close()
	})
}
