package console

import (
	"strings"
	"time"

	"github.com/gopherjs/gopherjs/js"
	"github.com/tile38/play/terminal"
)

const prompt = "\x1b[1m\x1b[37mtile38>\x1b[0m "

type Console struct {
	terminal     *terminal.Terminal
	clid         bool
	id           string
	serverOpened bool
	history      []string
	historyIdx   int
	lastInput    string
}

func New(parent *js.Object) (*Console, error) {
	t, err := terminal.New(parent)
	if err != nil {
		return nil, err
	}
	c := &Console{
		terminal: t,
	}
	c.loadServer()
	c.loadHistory()
	return c, nil
}

func (c *Console) loadServer() {
	id := js.Global.Get("localStorage").Call("getItem", "tile38:session:id").String()
	host := js.Global.Get("window").Get("location").Get("host").String()
	scheme := "ws"
	if js.Global.Get("window").Get("location").Get("protocol").String() == "https:" {
		scheme = "wss"
	}
	ws := js.Global.Get("WebSocket").New(scheme + "://" + host + "/server/" + id)
	ws.Call("addEventListener", "close", func(ev *js.Object) {
		println("server closed")
		c.terminal.ClearInput()
		c.terminal.WriteString("\x1b[31mServer closed: please refresh the page to start a new session.\x1b[0m\n")
		c.serverOpened = false
	})
	ws.Call("addEventListener", "open", func() {
		println("server opened")
		c.serverOpened = true
	})
	ws.Call("addEventListener", "error", func() {
		println("server error")
	})
	ws.Call("addEventListener", "message", func(ev *js.Object) {
		str := ev.Get("data").String()
		switch {
		case strings.HasPrefix(str, "invalidid: "):
			//invalidid := str[11:]
			//c.terminal.WriteString(invalidid + ": invalid session id\r\n")
		case strings.HasPrefix(str, "id: "):
			c.id = str[4:]
			js.Global.Get("localStorage").Call("setItem", "tile38:session:id", c.id)
			println(c.id)
		case strings.HasPrefix(str, "stderr: "):
			if !c.clid {
				s := str[8:]
				c.terminal.WriteString(s)
				if strings.Contains(s, "server is now ready to accept connections") {
					c.loadCLI()
					c.clid = true
				}
			}

		case strings.HasPrefix(str, "stdout: "):
			c.terminal.WriteString(str[8:])
		}
	})
}

func (c *Console) loadCLI() {
	noMorePrompts := false
	lastLive := false
	host := js.Global.Get("window").Get("location").Get("host").String()
	scheme := "ws"
	if js.Global.Get("window").Get("location").Get("protocol").String() == "https:" {
		scheme = "wss"
	}
	ws := js.Global.Get("WebSocket").New(scheme + "://" + host + "/cli/" + c.id)

	ws.Call("addEventListener", "close", func(ev *js.Object) {
		println("cli closed")
		c.terminal.Input = nil
		c.terminal.Up = nil
		c.terminal.Down = nil
		if c.serverOpened {
			c.terminal.WriteString("\x1b[31mCLI closed: trying again.\x1b[0m\n")
			go func() {
				time.Sleep(time.Second)
				if c.serverOpened {
					c.loadCLI()
				}
			}()
		}
	})
	ws.Call("addEventListener", "open", func() {
		println("cli opened")
		c.terminal.WriteString("\n")
		c.terminal.Prompt(prompt)
		c.terminal.Input = func(s string) {
			if strings.HasPrefix(strings.ToLower(s), "follow ") {
				c.terminal.WriteString("\x1b[31mSorry but FOLLOW is disabled.\x1b[0m\n")
				c.terminal.Prompt(prompt)
				return
			}
			if strings.HasPrefix(strings.ToLower(s), "aof ") {
				lastLive = true
			} else {
				lastLive = false
			}
			ws.Call("send", s)
			c.storeHistory(s)
		}
		c.terminal.Up = func() {
			if c.historyIdx == len(c.history) {
				// at end
				if len(c.history) == 0 {
					// nothing to do
					return
				}
				c.lastInput = c.terminal.GetInput()
			}
			if c.historyIdx == 0 {
				// nowhere to go
				return
			}
			c.historyIdx--
			c.terminal.SetInput(c.history[c.historyIdx])

		}
		c.terminal.Down = func() {
			if c.historyIdx == len(c.history) {
				// at end
				return
			}
			c.historyIdx++
			if c.historyIdx == len(c.history) {
				c.terminal.SetInput(c.lastInput)
				c.lastInput = ""
				return
			}
			c.terminal.SetInput(c.history[c.historyIdx])
		}
	})
	ws.Call("addEventListener", "error", func() {
		println("cli error")
	})
	ws.Call("addEventListener", "message", func(ev *js.Object) {
		str := ev.Get("data").String()
		switch {
		case strings.HasPrefix(str, "stderr: "):
			s := str[8:]
			c.terminal.WriteString(s)
			if strings.TrimSpace(s) == `{"ok":true,"live":true}` {
				c.terminal.WriteString("\x1b[32mYou are in live mode. No more input allowed.\x1b[0m\n")
				noMorePrompts = true
			}
		case strings.HasPrefix(str, "stdout: "):
			s := str[8:]
			c.terminal.WriteString(s)
			if (s == "+OK\r\n" || s == "+OK\n") && lastLive {
				c.terminal.WriteString("\x1b[32mYou are in live mode. No more input allowed.\x1b[0m\n")
				noMorePrompts = true
			}
		default:
			return
		}
		if !noMorePrompts {
			c.terminal.Prompt(prompt)
		}
	})
}

var histdel = "\n_HISTDEL_\n"

func (c *Console) loadHistory() {
	history := js.Global.Get("localStorage").Call("getItem", "tile38:history").String()
	if history != "null" {
		c.history = strings.Split(history, histdel)
	}
	c.historyIdx = len(c.history)
}

func (c *Console) storeHistory(line string) {
	if len(c.history) > 0 {
		if c.history[len(c.history)-1] == line {
			return
		}
	}
	c.history = append(c.history, line)
	if len(c.history) > 100 {
		c.history = c.history[len(c.history)-100:]
	}
	c.historyIdx = len(c.history)
	history := strings.Join(c.history, histdel)
	js.Global.Get("localStorage").Call("setItem", "tile38:history", history)
}
