package console

import (
	"strings"

	"github.com/gopherjs/gopherjs/js"
)

const (
	fontSize  = 12
	padx      = 8
	pady      = 4
	baseColor = "#ccc"
	linepad   = 3
	font      = "Menlo, Consolas, Monaco, Monospace, \"Times New Roman\", Times"
)

const (
	clear   = "[0m"
	bright  = "[1m"
	dim     = "[2m"
	black   = "[30m"
	red     = "[31m"
	green   = "[32m"
	yellow  = "[33m"
	blue    = "[34m"
	magenta = "[35m"
	cyan    = "[36m"
	white   = "[37m"
)

type Duration float64

const Second Duration = 1

func itoa(i int) string {
	return js.Global.Get("String").New(i).String()
}
func ftoa(f float64) string {
	return js.Global.Get("String").New(f).String()
}
func randi() int {
	return int(js.Global.Get("Math").Call("random").Float() * 2147483647.0)
}

type Console struct {
	parent, canvas *js.Object
	ctx            *js.Object
	width, height  float64
	ratio          float64
	timestamp      Duration
	cols, rows     int
	charWidth      float64
	charHeight     float64
	buffer         string
	dirty          bool
}

func New(parent *js.Object) (*Console, error) {
	c := &Console{
		parent: parent,
		dirty:  true,
	}
	js.Global.Call("addEventListener", "resize", func() {
		c.layout()
	})
	var raf string
	for _, s := range []string{"requestAnimationFrame", "webkitRequestAnimationFrame", "mozRequestAnimationFrame"} {
		if js.Global.Get(s) != js.Undefined {
			raf = s
			break
		}
	}
	if raf == "" {
		panic("requestAnimationFrame is not available")
	}
	defer c.layout()
	defer c.loadServer()
	count := 0
	var f func(*js.Object)
	f = func(timestampJS *js.Object) {
		js.Global.Call(raf, f)
		c.loop(Duration(timestampJS.Float() / 1000))
		count++
	}
	js.Global.Call(raf, f)
	return c, nil
}

func (c *Console) loadServer() {
	ws := js.Global.Get("WebSocket").New("ws://localhost:8000/server/")
	ws.Call("addEventListener", "close", func(ev *js.Object) {
		println("closed")
	})
	ws.Call("addEventListener", "open", func() {
		println("opened")
	})
	ws.Call("addEventListener", "error", func() {
		println("error")
	})
	ws.Call("addEventListener", "message", func(ev *js.Object) {
		str := ev.Get("data").String()
		switch {
		case strings.HasPrefix(str, "invalidid: "):
			invalidid := str[11:]
			println("invalid id: " + invalidid)
		case strings.HasPrefix(str, "id: "):
			id := str[11:]
			println("id: " + id)
		case strings.HasPrefix(str, "stderr: "):
			c.appendBuffer(str[8:])
		case strings.HasPrefix(str, "stdout: "):
			c.appendBuffer(str[8:])
		}
		c.dirty = true
	})
}

func (c *Console) layout() {
	c.dirty = true
	ratio := js.Global.Get("devicePixelRatio").Float()
	width := c.parent.Get("offsetWidth").Float() * ratio
	height := c.parent.Get("offsetHeight").Float() * ratio
	if c.canvas != nil && c.width == width && c.height == height && c.ratio == ratio {
		return
	}
	c.width, c.height, c.ratio = width, height, ratio
	if c.canvas != nil {
		c.parent.Call("removeChild", c.canvas)
	}
	c.canvas = js.Global.Get("document").Call("createElement", "canvas")
	c.ctx = c.canvas.Call("getContext", "2d")
	c.canvas.Set("width", c.width)
	c.canvas.Set("height", c.height)
	c.canvas.Get("style").Set("width", ftoa(c.width/c.ratio)+"px")
	c.canvas.Get("style").Set("height", ftoa(c.height/c.ratio)+"px")
	c.canvas.Get("style").Set("position", "absolute")
	c.parent.Call("appendChild", c.canvas)
	c.ctx.Set("font", itoa(int(fontSize*c.ratio))+"px "+font)
	c.charWidth = c.ctx.Call("measureText", "01234567890123456789").Get("width").Float() / 20
	c.rows = int((c.height - (pady * 2 * c.ratio)) / ((fontSize + linepad) * c.ratio))
	c.cols = int((c.width - (padx * 2 * c.ratio)) / c.charWidth)
	c.ctx.Set("fillStyle", baseColor)
	c.loop(c.timestamp)
}

func (c *Console) loop(timestamp Duration) {
	if timestamp == 0 || c.timestamp == 0 {
		c.timestamp = timestamp
		return
	}
	c.timestamp = timestamp
	if !c.dirty {
		return
	}
	defer func() {
		c.dirty = false
	}()
	c.ctx.Call("clearRect", 0, 0, c.width, c.height)
	c.draw()
}

func (c *Console) appendBuffer(str string) {
	c.buffer += str
	c.dirty = true
}

func (c *Console) drawChar(row, col int, ch rune) {
	if row < 0 || row >= c.rows || col < 0 || col >= c.cols {
		return
	}
	c.ctx.Call("fillText", string(ch), (padx*c.ratio + float64(col)*c.charWidth), (pady+float64(row+1)*(fontSize+linepad)-linepad/2)*c.ratio)
}

func (c *Console) setColor(color string) {
	c.ctx.Call("restore")
	c.ctx.Call("save")
	c.ctx.Set("fillStyle", color)
}

func (c *Console) handleESC(esc string) {
	switch esc {
	case clear:
		c.setColor(baseColor)
	case red:
		c.setColor("red")
	case green:
		c.setColor("green")
	case yellow:
		c.setColor("yellow")
	case blue:
		c.setColor("blue")
	case magenta:
		c.setColor("magenta")
	case cyan:
		c.setColor("cyan")
	case white:
		c.setColor("white")
	}
}
func (c *Console) draw() {
	c.ctx.Call("save")
	defer c.ctx.Call("restore")
	esc := true
	escs := ""
	x, y := 0, 0
	for _, ch := range c.buffer {
		if ch == 0x1B {
			esc = true
			escs = ""
			continue
		}
		if esc {
			if escs == "" && ch != '[' {
				esc = false
			} else {
				escs += string(ch)
				if ch == 'm' {
					esc = false
				}
				continue
			}
		}
		if escs != "" {
			c.handleESC(escs)
			escs = ""
		}
		if ch == '\r' {
			x = 0
		} else if ch == '\n' {
			x = 0
			y++
		} else {
			c.drawChar(y, x, ch)
			x++
			if x >= c.cols {
				x = 0
				y++
			}
		}
	}
}
