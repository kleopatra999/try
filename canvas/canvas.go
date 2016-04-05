package canvas

import "github.com/gopherjs/gopherjs/js"

const (
	fontSize  = 12
	padx      = 8
	pady      = 4
	baseColor = "#ccc"
	linepad   = 3
	font      = "Menlo, Consolas, Monaco, Monospace, \"Times New Roman\", Times"
	retina    = true
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

type Canvas struct {
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
	input          string
	acceptInput    bool
	prompt         string
	cursorIdx      int
	scrollY        int
	cancelScroll   bool
	Input          func(s string)
	Up, Down       func()
}

func New(parent *js.Object) (*Canvas, error) {
	c := &Canvas{
		parent: parent,
		dirty:  true,
	}
	js.Global.Call("addEventListener", "resize", func() {
		c.layout()
	})

	js.Global.Get("window").Call("addEventListener", "mousewheel", func(ev *js.Object) bool {
		deltaY := ev.Get("deltaY").Int()
		c.scroll(deltaY)
		return true
	})

	js.Global.Get("document").Call("addEventListener", "keydown", func(ev *js.Object) bool {
		code := ev.Get("keyCode").Int()
		if code == 8 || code == 46 {
			ev.Call("preventDefault")
			switch code {
			case 8:
				c.delete()
			case 46:

			}
			return false
		}
		switch code {
		case 37:
			c.arrow(-1)
		case 38:
			if c.Up != nil {
				c.Up()
			}
		case 39:
			c.arrow(+1)
		case 40:
			if c.Down != nil {
				c.Down()
			}
		}
		return true
	})
	js.Global.Get("document").Call("addEventListener", "keypress", func(ev *js.Object) bool {
		if !c.acceptInput {
			return false
		}
		c.scrollY = 0
		js.Global.Get("window").Call("scrollTo", 0, 0)
		code := ev.Get("keyCode").Int()
		if code == 13 {
			input := c.input
			c.WriteString(c.prompt + c.input + "\n")
			if input != "" {
				c.input = ""
				c.cursorIdx = 0
				c.acceptInput = false
				if c.Input != nil {
					c.Input(input)
				}
			}
		} else {
			c.input = c.input[:c.cursorIdx] + string(rune(code)) + c.input[c.cursorIdx:]
			c.cursorIdx++
		}
		c.dirty = true
		return true
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

func (c *Canvas) delete() {
	if c.cursorIdx > 0 {
		c.cursorIdx--
		c.input = c.input[:c.cursorIdx] + c.input[c.cursorIdx+1:]
		c.dirty = true
	}
}

func (c *Canvas) arrow(delta int) {
	c.cursorIdx += delta
	if c.cursorIdx < 0 {
		c.cursorIdx = 0
	} else if c.cursorIdx > len(c.input) {
		c.cursorIdx = len(c.input)
	}
	c.dirty = true
}

func (c *Canvas) scroll(deltaY int) {
	d := float64(deltaY) / ((fontSize + linepad) * c.ratio)
	if d >= -1 && d <= 1 {
		if d < 0 {
			d = -1
		} else {
			d = 1
		}
	}
	c.scrollY -= int(d)
	c.dirty = true
}

func (c *Canvas) layout() {
	c.dirty = true
	ratio := js.Global.Get("devicePixelRatio").Float()
	if !retina {
		ratio = 1
	}
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

func (c *Canvas) loop(timestamp Duration) {
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

func (c *Canvas) Prompt(prompt string) {
	c.prompt = prompt
	c.acceptInput = true
	c.dirty = true
}

func (c *Canvas) WriteString(s string) {
	c.buffer += s
	c.dirty = true
}

func (c *Canvas) drawChar(row, col int, ch rune) {
	if row < 0 || row >= c.rows || col < 0 || col >= c.cols {
		return
	}
	x, y := (padx*c.ratio + float64(col)*c.charWidth), (pady+float64(row+1)*(fontSize+linepad)-linepad/2)*c.ratio
	c.ctx.Call("fillText", string(ch), x, y)
}

func (c *Canvas) drawCursor(row, col int) {
	if row < 0 || row >= c.rows || col < 0 || col >= c.cols {
		return
	}
	x, y := (padx*c.ratio + float64(col)*c.charWidth), (pady+float64(row+1)*(fontSize+linepad)-linepad/2)*c.ratio
	c.ctx.Call("fillRect", x, y-((fontSize-1)*c.ratio), c.charWidth, (fontSize+1)*c.ratio)
}

func (c *Canvas) setColor(color string) {
	c.ctx.Call("restore")
	c.ctx.Call("save")
	c.ctx.Set("fillStyle", color)
}

func (c *Canvas) handleESC(esc string) {
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
func (c *Canvas) drawBuffer(s string, x, y int, esc bool, escs string, cursorIdx int, blit bool) (int, int, bool, string) {
	i := 0
	cursor := false
	for _, ch := range s {
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
			if blit {
				c.handleESC(escs)
			}
			escs = ""
		}
		if ch == '\r' {
			x = 0
		} else if ch == '\n' {
			x = 0
			y++
		} else {
			if i == cursorIdx {
				if blit {
					c.drawCursor(y, x)
					c.setColor("black")
					c.drawChar(y, x, ch)
					c.setColor(baseColor)
				}
				cursor = true
			} else {
				if blit {
					c.drawChar(y, x, ch)
				}
			}
			x++
			if x >= c.cols {
				x = 0
				y++
			}
			i++
		}
	}
	if !cursor && cursorIdx != -1 {
		if blit {
			c.drawCursor(y, x)
		}
	}
	return x, y, esc, escs
}

func (c *Canvas) draw() {
	// predraw - do not blit
	x, y, esc, escs := 0, 0, true, ""
	x, y, esc, escs = c.drawBuffer(c.buffer, x, y, esc, escs, -1, false)
	if c.acceptInput {
		x, y, esc, escs = c.drawBuffer(c.prompt, x, y, esc, escs, -1, false)
		x, y, esc, escs = c.drawBuffer(c.input, x, y, esc, escs, c.cursorIdx, false)
	}

	minScrollY := 0
	maxScrollY := y - c.rows + 1
	if maxScrollY < 0 {
		maxScrollY = 0
	}

	c.ctx.Call("save")
	defer c.ctx.Call("restore")

	if y >= c.rows {
		y = (c.rows - y - 1)
	} else {
		y = 0
	}

	if c.scrollY < minScrollY {
		c.scrollY = minScrollY
	} else if c.scrollY > maxScrollY {
		c.scrollY = maxScrollY
	}

	y += c.scrollY

	// real draw
	x, esc, escs = 0, true, ""
	x, y, esc, escs = c.drawBuffer(c.buffer, x, y, esc, escs, -1, true)
	if c.acceptInput {
		x, y, esc, escs = c.drawBuffer(c.prompt, x, y, esc, escs, -1, true)
		x, y, esc, escs = c.drawBuffer(c.input, x, y, esc, escs, c.cursorIdx, true)
	}
}
