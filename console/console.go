package console

import (
	"strings"

	"github.com/gopherjs/gopherjs/js"
	"github.com/tile38/play/canvas"
)

type Console struct {
	canvas *canvas.Canvas
	clid   bool
	id     string
}

func New(parent *js.Object) (*Console, error) {
	cv, err := canvas.New(parent)
	if err != nil {
		return nil, err
	}
	c := &Console{
		canvas: cv,
	}
	c.loadServer()
	return c, nil
}

func (c *Console) loadServer() {
	host := js.Global.Get("window").Get("location").Get("host").String()
	scheme := "ws"
	if js.Global.Get("window").Get("location").Get("protocol").String() == "https:" {
		scheme = "wss"
	}
	ws := js.Global.Get("WebSocket").New(scheme + "://" + host + "/server/")
	ws.Call("addEventListener", "close", func(ev *js.Object) {
		println("server closed")
	})
	ws.Call("addEventListener", "open", func() {
		println("server opened")
	})
	ws.Call("addEventListener", "error", func() {
		println("server error")
	})
	ws.Call("addEventListener", "message", func(ev *js.Object) {
		str := ev.Get("data").String()
		switch {
		case strings.HasPrefix(str, "invalidid: "):
			//invalidid := str[11:]
			//c.canvas.WriteString(invalidid + ": invalid session id\r\n")
		case strings.HasPrefix(str, "id: "):
			c.id = str[4:]
			c.canvas.WriteString(c.id + "\n")
		case strings.HasPrefix(str, "stderr: "):
			if !c.clid {
				s := str[8:]
				c.canvas.WriteString(s)
				if strings.Contains(s, "server is now ready to accept connections") {
					c.loadCLI()
					c.clid = true
				}
			}

		case strings.HasPrefix(str, "stdout: "):
			c.canvas.WriteString(str[8:])
		}
	})
}
func (c *Console) loadCLI() {
	host := js.Global.Get("window").Get("location").Get("host").String()
	scheme := "ws"
	if js.Global.Get("window").Get("location").Get("protocol").String() == "https:" {
		scheme = "wss"
	}
	ws := js.Global.Get("WebSocket").New(scheme + "://" + host + "/cli/" + c.id)

	ws.Call("addEventListener", "close", func(ev *js.Object) {
		println("cli closed")
	})
	ws.Call("addEventListener", "open", func() {
		println("cli opened")
		c.canvas.WriteString("\n")
		c.canvas.Prompt("tile38> ")
	})
	ws.Call("addEventListener", "error", func() {
		println("cli error")
	})
	ws.Call("addEventListener", "message", func(ev *js.Object) {
		str := ev.Get("data").String()
		switch {
		case strings.HasPrefix(str, "stderr: "):
			s := str[8:]
			c.canvas.WriteString(s)
		case strings.HasPrefix(str, "stdout: "):
			c.canvas.WriteString(str[8:])
		default:
			return
		}
		c.canvas.Prompt("tile38> ")

	})
	c.canvas.Input = func(s string) {
		ws.Call("send", s)
	}
}
