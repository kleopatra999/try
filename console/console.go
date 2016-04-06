package console

import (
	"strings"
	"time"

	"github.com/gopherjs/gopherjs/js"
	"github.com/tile38/play/terminal"
)

const prompt = "\x1b[1m\x1b[37m%s>\x1b[0m "

type Console struct {
	terminal     *terminal.Terminal
	clid         bool
	id           string
	serverOpened bool
	history      []string
	historyIdx   int
	lastInput    string
	service      string
	prompt       string
}

func New(parent *js.Object, service string) (*Console, error) {
	t, err := terminal.New(parent)
	if err != nil {
		return nil, err
	}
	c := &Console{
		terminal: t,
		service:  service,
		prompt:   strings.Replace(prompt, "%s", service, -1),
	}
	c.showMessage()
	c.loadServer()
	c.loadHistory()
	return c, nil
}

func (c *Console) showMessage() {
	// msg := "\x1b[37m\x1b[1mWelcome to Try Tile38, a demonstration of the Tile38 database!\x1b[0m\n"
	// msg += "\n"
	// msg += "Please type \x1b[37m\x1b[1mHELP\x1b[0m to see a list of supported commands.\n"

	// msg += "\n\n"

	// c.terminal.WriteString(msg)
}

func (c *Console) loadServer() {
	id := js.Global.Get("localStorage").Call("getItem", c.service+":session:id").String()
	if id == "null" {
		id = ""
	}
	host := js.Global.Get("window").Get("location").Get("host").String()
	scheme := "ws"
	if js.Global.Get("window").Get("location").Get("protocol").String() == "https:" {
		scheme = "wss"
	}
	ws := js.Global.Get("WebSocket").New(scheme + "://" + host + "/" + c.service + "-server/" + id)
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
	ws.Call("addEventListener", "message", func(ev *js.Object) {
		str := ev.Get("data").String()
		switch {
		case strings.HasPrefix(str, "invalidid: "):
			//invalidid := str[11:]
			//c.terminal.WriteString(invalidid + ": invalid session id\r\n")
		case strings.HasPrefix(str, "id: "):
			c.id = str[4:]
			js.Global.Get("localStorage").Call("setItem", c.service+":session:id", c.id)
			println(c.id)
		case strings.HasPrefix(str, "stderr: ") || strings.HasPrefix(str, "stdout: "):
			if !c.clid {
				s := str[8:]
				c.terminal.WriteString(s)
				if strings.Contains(s, "server is now ready to accept connections") {
					c.loadCLI()
					c.clid = true
				}
			}
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
	ws := js.Global.Get("WebSocket").New(scheme + "://" + host + "/" + c.service + "-cli/" + c.id)

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
		c.terminal.Prompt(c.prompt)
		c.terminal.Input = func(s string) {
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
			c.terminal.Prompt(c.prompt)
		}
	})
}

var histdel = "\n_HISTDEL_\n"

func (c *Console) loadHistory() {
	history := js.Global.Get("localStorage").Call("getItem", c.service+":history").String()
	if history != "null" {
		c.history = strings.Split(history, histdel)
	}
	c.historyIdx = len(c.history)
}

func (c *Console) storeHistory(line string) {
	defer func() {
		c.historyIdx = len(c.history)
	}()
	if len(c.history) > 0 {
		if c.history[len(c.history)-1] == line {
			return
		}
	}
	c.history = append(c.history, line)
	if len(c.history) > 100 {
		c.history = c.history[len(c.history)-100:]
	}
	history := strings.Join(c.history, histdel)
	js.Global.Get("localStorage").Call("setItem", c.service+":history", history)
}
