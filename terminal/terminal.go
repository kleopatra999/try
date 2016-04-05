package terminal

import "github.com/gopherjs/gopherjs/js"

const (
	fontSize  = 12
	padx      = 8
	pady      = 8
	baseColor = "#bbb"
	linepad   = 3
	font      = "Monaco, Consolas, Menlo, Monospace, \"Times New Roman\", Times"
	retina    = true
	dbgBorder = true
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
	dgray   = "[90m"
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

type Terminal struct {
	parent, canvas *js.Object
	ctx            *js.Object
	textarea       *js.Object
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
	scrollY        float64
	cancelScroll   bool
	Input          func(s string)
	Up, Down       func()
	color          string
	bright         bool
	mdown          bool
}

func New(parent *js.Object) (*Terminal, error) {
	t := &Terminal{
		parent: parent,
		dirty:  true,
	}
	js.Global.Call("addEventListener", "resize", func() {
		t.layout()
	})
	js.Global.Get("window").Call("addEventListener", "mousewheel", func(ev *js.Object) bool {
		deltaY := ev.Get("deltaY").Float()
		t.scroll(deltaY)
		return true
	})

	js.Global.Get("document").Call("addEventListener", "mousedown", func(ev *js.Object) bool {
		t.mdown = true
		x, y := ev.Get("offsetX").Float(), ev.Get("offsetY").Float()
		println(x, y)
		return true
	})

	js.Global.Get("document").Call("addEventListener", "mousemove", func(ev *js.Object) bool {
		if t.mdown {
			println("mousemove")
			x, y := ev.Get("offsetX").Float(), ev.Get("offsetY").Float()
			println(x, y)
		}
		return true
	})

	js.Global.Get("document").Call("addEventListener", "mouseup", func(ev *js.Object) bool {
		t.mdown = false
		println("mouseup")
		return true
	})

	js.Global.Get("document").Call("addEventListener", "keydown", func(ev *js.Object) bool {
		t.scrollToEnd()
		code := ev.Get("keyCode").Int()
		if code == 8 || code == 46 {
			ev.Call("preventDefault")
			switch code {
			case 8:
				t.delete()
			case 46:

			}
			return false
		}
		switch code {
		case 37:
			t.arrow(-1)
		case 38:
			if t.Up != nil {
				t.Up()
			}
		case 39:
			t.arrow(+1)
		case 40:
			if t.Down != nil {
				t.Down()
			}
		}
		return true
	})
	js.Global.Get("document").Call("addEventListener", "keypress", func(ev *js.Object) bool {
		t.scrollToEnd()
		if !t.acceptInput {
			return false
		}
		code := ev.Get("keyCode").Int()
		if code == 13 {
			input := t.input
			t.WriteString(t.prompt + t.input + "\n")
			if input != "" {
				t.input = ""
				t.cursorIdx = 0
				t.acceptInput = false
				if t.Input != nil {
					t.Input(input)
				}
			}
		} else {
			t.input = t.input[:t.cursorIdx] + string(rune(code)) + t.input[t.cursorIdx:]
			t.cursorIdx++
		}
		t.dirty = true
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
	defer t.layout()
	count := 0
	var f func(*js.Object)
	f = func(timestampJS *js.Object) {
		js.Global.Call(raf, f)
		t.loop(Duration(timestampJS.Float() / 1000))
		count++
	}
	js.Global.Call(raf, f)
	return t, nil
}

func (t *Terminal) getRowColForPixel(x, y float64) (row, col int) {

	return 0, 0
}

func (t *Terminal) scrollToEnd() {
	t.scrollY = 0
	js.Global.Get("window").Call("scrollTo", 0, 0)
	t.dirty = true
}

func (t *Terminal) delete() {
	if t.cursorIdx > 0 {
		t.cursorIdx--
		t.input = t.input[:t.cursorIdx] + t.input[t.cursorIdx+1:]
		t.dirty = true
	}
}

func (t *Terminal) arrow(delta int) {
	t.cursorIdx += delta
	if t.cursorIdx < 0 {
		t.cursorIdx = 0
	} else if t.cursorIdx > len(t.input) {
		t.cursorIdx = len(t.input)
	}
	t.dirty = true
}

func (t *Terminal) scroll(deltaY float64) {
	d := deltaY / t.charHeight
	t.scrollY -= d
	t.dirty = true
}

func (t *Terminal) layout() {
	t.dirty = true
	ratio := js.Global.Get("devicePixelRatio").Float()
	if !retina {
		ratio = 1
	}
	width := t.parent.Get("offsetWidth").Float() * ratio
	height := t.parent.Get("offsetHeight").Float() * ratio
	if t.canvas != nil && t.width == width && t.height == height && t.ratio == ratio {
		return
	}
	t.width, t.height, t.ratio = width, height, ratio
	if t.canvas != nil {
		t.parent.Call("removeChild", t.canvas)
	}

	t.canvas = js.Global.Get("document").Call("createElement", "canvas")
	t.ctx = t.canvas.Call("getContext", "2d")
	t.canvas.Set("width", t.width)
	t.canvas.Set("height", t.height)
	t.canvas.Get("style").Set("width", ftoa(t.width/t.ratio)+"px")
	t.canvas.Get("style").Set("height", ftoa(t.height/t.ratio)+"px")
	t.canvas.Get("style").Set("position", "absolute")
	t.parent.Call("appendChild", t.canvas)
	t.ctx.Set("font", itoa(int(fontSize*t.ratio))+"px "+font)
	t.charWidth = t.ctx.Call("measureText", "01234567890123456789").Get("width").Float() / 20 / t.ratio
	t.charHeight = fontSize + linepad
	t.rows = int((t.height - (pady * 2 * t.ratio)) / (t.charHeight * t.ratio))
	t.cols = int((t.width - (padx * 2 * t.ratio)) / (t.charWidth * t.ratio))
	t.resetFontStyle()

	// if t.textarea == nil {
	// 	t.textarea = js.Global.Get("document").Call("createElement", "textarea")
	// 	t.parent.Call("appendChild", t.textarea)
	// 	t.textarea.Get("style").Set("position", "absolute")
	// 	//t.textarea.Get("style").Set("display", "none")
	// }

	t.loop(t.timestamp)
}

func (t *Terminal) loop(timestamp Duration) {
	if t.textarea != nil {
		t.textarea.Call("focus")
	}
	if timestamp == 0 || t.timestamp == 0 {
		t.timestamp = timestamp
		return
	}
	t.timestamp = timestamp
	if !t.dirty {
		return
	}
	defer func() {
		t.dirty = false
	}()
	t.ctx.Call("clearRect", 0, 0, t.width, t.height)
	t.draw()
}

func (t *Terminal) ClearInput() {
	t.prompt = ""
	t.acceptInput = false
	t.input = ""
	t.dirty = true
}

func (t *Terminal) SetInput(input string) {
	t.input = input
	t.cursorIdx = len(t.input)
	t.dirty = true
}

func (t *Terminal) GetInput() string {
	return t.input
}

func (t *Terminal) Prompt(prompt string) {
	t.prompt = prompt
	t.acceptInput = true
	t.dirty = true
}

func (t *Terminal) WriteString(s string) {
	t.buffer += s
	t.dirty = true
}

func (t *Terminal) drawChar(row, col int, ch rune) {
	if row < 0 || row >= t.rows || col < 0 || col >= t.cols {
		return
	}
	x, y := (padx*t.ratio + float64(col)*t.charWidth*t.ratio), (pady+float64(row+1)*t.charHeight-linepad)*t.ratio
	t.ctx.Call("fillText", string(ch), x, y)
}

func (t *Terminal) drawCursor(row, col int) {
	if row < 0 || row >= t.rows || col < 0 || col >= t.cols {
		return
	}
	x, y := (padx*t.ratio + float64(col)*t.charWidth*t.ratio), (pady+float64(row+1)*t.charHeight)*t.ratio
	t.ctx.Call("fillRect", x, y-(t.charHeight*t.ratio), t.charWidth*t.ratio, t.charHeight*t.ratio)
}

func (t *Terminal) setColor(color string) {
	t.color = color
	t.setFontStyle()
}
func (t *Terminal) setBright(bright bool) {
	t.bright = bright
	t.setFontStyle()
}
func (t *Terminal) setFontStyle() {
	t.ctx.Call("restore")
	t.ctx.Call("save")
	var s string
	if t.bright {
		s += "Bold "
	}
	s += itoa(int(fontSize*t.ratio)) + "px "
	s += font
	t.ctx.Set("font", s)
	t.ctx.Set("fillStyle", t.color)
}

func (t *Terminal) resetFontStyle() {
	t.color = baseColor
	t.bright = false
	t.setFontStyle()
}

func (t *Terminal) handleESC(esc string) {
	switch esc {
	default:
		println(esc)
	case clear:
		t.resetFontStyle()
	case red:
		t.setColor("red")
	case bright:
		t.setBright(true)
	case dim:
		t.setBright(false)
	case green:
		t.setColor("green")
	case yellow:
		t.setColor("yellow")
	case blue:
		t.setColor("blue")
	case magenta:
		t.setColor("magenta")
	case cyan:
		t.setColor("cyan")
	case white:
		t.setColor("white")
	case dgray:
		t.setColor("#666")
	}
}
func (t *Terminal) drawBuffer(s string, x, y int, esc bool, escs string, cursorIdx int, blit bool) (int, int, bool, string) {
	i := 0
	cursor := false
	for _, ch := range s {

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
		} else {
			if ch == 0x1B {
				if escs != "" {
					if blit {
						t.handleESC(escs)
					}
				}
				esc = true
				escs = ""
				continue
			}
		}
		if escs != "" {
			if blit {
				t.handleESC(escs)
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
					t.drawCursor(y, x)
					t.setColor("black")
					t.drawChar(y, x, ch)
					t.setColor(baseColor)
				}
				cursor = true
			} else {
				if blit {
					t.drawChar(y, x, ch)
				}
			}
			x++
			if x >= t.cols {
				x = 0
				y++
			}
			i++
		}
	}
	if !cursor && cursorIdx != -1 {
		if blit {
			t.drawCursor(y, x)
		}
	}
	return x, y, esc, escs
}

func (t *Terminal) draw() {
	// predraw - do not blit
	x, y, esc, escs := 0, 0, true, ""
	x, y, esc, escs = t.drawBuffer(t.buffer, x, y, esc, escs, -1, false)
	if t.acceptInput {
		x, y, esc, escs = t.drawBuffer(t.prompt, x, y, esc, escs, -1, false)
		x, y, esc, escs = t.drawBuffer(t.input, x, y, esc, escs, t.cursorIdx, false)
	}

	minScrollY := 0.0
	maxScrollY := float64(y - t.rows + 1)
	if maxScrollY < 0 {
		maxScrollY = 0
	}

	t.ctx.Call("save")
	defer t.ctx.Call("restore")

	if dbgBorder {
		defer func() {
			t.ctx.Call("restore")
			t.ctx.Set("strokeStyle", baseColor)
			t.ctx.Call("strokeRect", padx*t.ratio, pady*t.ratio, (float64(t.cols) * t.charWidth * t.ratio), (float64(t.rows)*t.charHeight)*t.ratio)
			t.ctx.Call("save")
		}()
	}

	if y >= t.rows {
		y = (t.rows - y - 1)
	} else {
		y = 0
	}

	if t.scrollY < minScrollY {
		t.scrollY = minScrollY
	} else if t.scrollY > maxScrollY {
		t.scrollY = maxScrollY
	}

	y += int(t.scrollY)

	// real draw
	x, esc, escs = 0, true, ""
	x, y, esc, escs = t.drawBuffer(t.buffer, x, y, esc, escs, -1, true)
	if t.acceptInput {
		x, y, esc, escs = t.drawBuffer(t.prompt, x, y, esc, escs, -1, true)
		x, y, esc, escs = t.drawBuffer(t.input, x, y, esc, escs, t.cursorIdx, true)
	}
}
