package terminal

import (
	"bytes"
	"time"

	"github.com/gopherjs/gopherjs/js"
)

const (
	fontSize        = 12
	padx            = 8
	pady            = 8
	baseColor       = "#bbb"
	selectionColor  = "#7be"
	backgroundColor = "#000"
	linepad         = 3
	font            = "Monaco, Consolas, Menlo, Monospace, \"Times New Roman\", Times"
	retina          = true
	dbgBorder       = false
	clickDuration   = time.Millisecond * 100
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
	rowOffset      int
	maxRowOffset   int
	mdownrow       int
	mdowncol       int
	mmoverow       int
	mmovecol       int
	mtime          time.Time
	selStart       int
	selEnd         int
	selectedString *bytes.Buffer
	scrollInt      *js.Object
	scrolling      bool
	pasted         bool
}

func New(parent *js.Object) (*Terminal, error) {
	t := &Terminal{
		parent:         parent,
		dirty:          true,
		selectedString: &bytes.Buffer{},
	}
	js.Global.Call("addEventListener", "resize", func() {
		t.layout()
	})
	js.Global.Get("window").Call("addEventListener", "focus", func(ev *js.Object) bool {
		t.dirty = true
		return true
	})
	js.Global.Get("window").Call("addEventListener", "blur", func(ev *js.Object) bool {
		t.dirty = true
		return true
	})
	js.Global.Get("window").Call("addEventListener", "wheel", func(ev *js.Object) bool {
		deltaY := ev.Get("deltaY").Float()
		t.scroll(deltaY)
		t.scrolling = true
		js.Global.Get("window").Call("clearTimeout", t.scrollInt)
		t.scrollInt = js.Global.Get("window").Call("setTimeout", func() {
			t.scrolling = false
		}, 250)
		return true
	})

	js.Global.Get("document").Call("addEventListener", "mousedown", func(ev *js.Object) bool {
		t.mdown = true
		t.mdownrow, t.mdowncol = t.getRowColForPixel(ev.Get("offsetX").Float(), ev.Get("offsetY").Float())
		t.dirty = true
		t.mtime = time.Now()
		t.selStart = 0
		t.selEnd = 0
		t.dirty = true
		return true
	})

	js.Global.Get("document").Call("addEventListener", "mousemove", func(ev *js.Object) bool {
		if t.mdown {
			t.mmoverow, t.mmovecol = t.getRowColForPixel(ev.Get("offsetX").Float(), ev.Get("offsetY").Float())
			t.dirty = true
			if t.mdownrow == t.mmoverow {
				rowStart := (t.mdownrow) * t.cols
				if t.mdowncol == t.mmovecol {
					t.selStart = 0
					t.selEnd = 0
				} else if t.mdowncol < t.mmovecol {
					t.selStart = rowStart + t.mdowncol
					t.selEnd = rowStart + t.mmovecol
				} else {
					t.selStart = rowStart + t.mmovecol
					t.selEnd = rowStart + t.mdowncol
				}
			} else if t.mdownrow < t.mmoverow {
				rowStart := (t.mdownrow) * t.cols
				rowEnd := (t.mmoverow) * t.cols
				t.selStart = rowStart + t.mdowncol
				t.selEnd = rowEnd + t.mmovecol
			} else {
				rowStart := (t.mmoverow) * t.cols
				rowEnd := (t.mdownrow) * t.cols
				t.selStart = rowStart + t.mmovecol
				t.selEnd = rowEnd + t.mdowncol
			}
		}
		return true
	})

	// js.Global.Get("window").Call("addEventListener", "click", func(ev *js.Object) bool {
	// 	row, col := t.getRowColForPixel(ev.Get("offsetX").Float(), ev.Get("offsetY").Float())
	// 	pos := row*t.cols + col

	// 	if ev.Get("detail").Int() >= 3 {
	// 		//	println("triple click")
	// 	} else if ev.Get("detail").Int() >= 2 {
	// 		//	println("double click")
	// 		println(pos)
	// 	} else if ev.Get("detail").Int() >= 1 {

	// 	}
	// 	return true
	// })

	js.Global.Get("document").Call("addEventListener", "mouseup", func(ev *js.Object) bool {
		if t.mdown {
			t.mdown = false
			t.dirty = true
		}
		return true
	})

	js.Global.Get("document").Call("addEventListener", "keydown", func(ev *js.Object) bool {
		code := ev.Get("keyCode").Int()
		switch code {
		default:
			return true
		case 8, 46, 37, 38, 39, 40:
		}
		t.scrollToEndIfNotScrolling()

		if code == 8 || code == 46 {
			ev.Call("preventDefault")
			switch code {
			case 8:
				t.backspace()
			case 46:
				t.delete()
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
		if !t.acceptInput {
			return false
		}
		t.scrollToEndIfNotScrolling()
		code := ev.Get("keyCode").Int()
		t.appendChar(rune(code), true)
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
	col = int((x - padx) / (float64(t.cols) * t.charWidth) * float64(t.cols))
	if col < 0 {
		col = 0
	} else if col > t.cols {
		col = t.cols
	}
	row = int((y - pady) / (float64(t.rows) * t.charHeight) * float64(t.rows))
	if row < 0 {
		row = 0
	} else if row > t.rows {
		row = t.rows
	}
	row += t.rowOffset
	return
}

func (t *Terminal) appendChar(code rune, fromEvent bool) {
	t.dirty = true
	if (fromEvent && code == 13) || (!fromEvent && code == 10) {
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
		return
	}
	t.input = t.input[:t.cursorIdx] + string(rune(code)) + t.input[t.cursorIdx:]
	t.cursorIdx++
}

func (t *Terminal) appendStr(s string) {
	for _, code := range s {
		t.appendChar(code, false)
	}
}

func (t *Terminal) scrollToEnd() {
	t.scrollY = 0
	js.Global.Get("window").Call("scrollTo", 0, 0)
	t.dirty = true
}

func (t *Terminal) scrollToEndIfNotScrolling() {
	if !t.scrolling {
		t.scrollToEnd()
	}
}

func (t *Terminal) backspace() {
	if t.cursorIdx > 0 {
		t.cursorIdx--
		t.input = t.input[:t.cursorIdx] + t.input[t.cursorIdx+1:]
		t.dirty = true
	}
}

func (t *Terminal) delete() {
	if t.cursorIdx < len(t.input) {
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

	if t.textarea == nil {
		t.textarea = js.Global.Get("document").Call("createElement", "textarea")
		t.parent.Call("appendChild", t.textarea)
		t.textarea.Get("style").Set("position", "absolute")
		t.textarea.Get("style").Set("opacity", 0)
		t.textarea.Call("addEventListener", "paste", func(ev *js.Object) bool {
			t.pasted = true
			return true
		})
	}

	t.canvas = js.Global.Get("document").Call("createElement", "canvas")
	t.ctx = t.canvas.Call("getContext", "2d")
	t.canvas.Set("width", t.width)
	t.canvas.Set("height", t.height)
	t.canvas.Get("style").Set("width", ftoa(t.width/t.ratio)+"px")
	t.canvas.Get("style").Set("height", ftoa(t.height/t.ratio)+"px")
	t.canvas.Get("style").Set("position", "absolute")
	t.canvas.Get("style").Set("backgroundColor", backgroundColor)
	t.parent.Call("appendChild", t.canvas)

	t.ctx.Set("font", itoa(int(fontSize*t.ratio))+"px "+font)
	t.charWidth = t.ctx.Call("measureText", "01234567890123456789").Get("width").Float() / 20 / t.ratio
	t.charHeight = fontSize + linepad
	t.rows = int((t.height - (pady * 2 * t.ratio)) / (t.charHeight * t.ratio))
	t.cols = int((t.width - (padx * 2 * t.ratio)) / (t.charWidth * t.ratio))
	t.resetFontStyle()

	t.loop(t.timestamp)
}

func (t *Terminal) loop(timestamp Duration) {
	if t.textarea != nil {
		t.textarea.Call("focus")
	}
	if t.pasted {
		t.pasted = false
		t.appendStr(t.textarea.Get("value").String())
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

func (t *Terminal) drawCursor(row, col int, solid bool) {
	if row < 0 || row >= t.rows || col < 0 || col >= t.cols {
		return
	}
	x, y := (padx*t.ratio + float64(col)*t.charWidth*t.ratio), (pady+float64(row+1)*t.charHeight)*t.ratio
	if solid || js.Global.Get("document").Call("hasFocus").Bool() {
		t.ctx.Call("fillRect", x, y-(t.charHeight*t.ratio), (t.charWidth+0.5)*t.ratio, (t.charHeight+0.5)*t.ratio)
	} else {
		t.ctx.Set("lineWidth", 1*t.ratio)
		t.ctx.Call("strokeRect", x, y-(t.charHeight*t.ratio), (t.charWidth+0.5)*t.ratio, (t.charHeight+0.5)*t.ratio)
	}
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
	t.ctx.Set("strokeStyle", t.color)
}

func (t *Terminal) resetFontStyle() {
	t.color = baseColor
	t.bright = false
	t.setFontStyle()
}

func (t *Terminal) handleESC(esc string) {
	switch esc {
	default:
		//println(esc)
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
func (t *Terminal) drawBuffer(s string, charIdx int, x, y int, esc bool, escs string, cursorIdx int, blit bool) (int, int, int, bool, string) {
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
			if !blit {
				pos := (y+(t.rowOffset))*t.cols + x
				if pos >= t.selStart && pos < t.selEnd {
					t.selectedString.WriteRune(ch)
				}
			}
		} else if ch == '\n' {
			x = 0
			y++
			if !blit {
				pos := (y+(t.rowOffset))*t.cols + x
				if pos >= t.selStart && pos < t.selEnd {
					t.selectedString.WriteRune(ch)
				}
			}
		} else {
			pos := (y+(t.rowOffset))*t.cols + x
			if i == cursorIdx {
				if blit {
					t.drawCursor(y, x, false)
					t.setColor("black")
					t.drawChar(y, x, ch)
					t.setColor(baseColor)
				}
				cursor = true
			} else {
				if pos >= t.selStart && pos < t.selEnd {
					if blit {
						t.setColor(selectionColor)
						t.drawCursor(y, x, true)
						t.setColor("black")
						t.drawChar(y, x, ch)
						t.setColor(baseColor)
					} else {
						t.selectedString.WriteRune(ch)
					}
				} else {
					if blit {
						t.drawChar(y, x, ch)
					}
				}
			}
			x++
			if x >= t.cols {
				x = 0
				y++
			}
			i++
			charIdx++
		}
	}
	if !cursor && cursorIdx != -1 {
		if blit {
			t.drawCursor(y, x, false)
		}
	}
	return x, y, charIdx, esc, escs
}

func (t *Terminal) drawSelectionBlocks() {
	for y := 0 + t.rowOffset; y < t.rows+t.rowOffset; y++ {
		for x := 0; x < t.cols; x++ {
			pos := y*t.cols + x
			if pos >= t.selStart && pos < t.selEnd {
				t.setColor(selectionColor)
				t.drawCursor(y+t.rowOffset*-1, x, true)
				t.setColor(baseColor)
			}
		}
	}
}

func (t *Terminal) calcDrawBuffers(x, y int, esc bool, escs string, blit bool) (int, int, bool, string) {
	charIdx := 0
	x, y, charIdx, esc, escs = t.drawBuffer(t.buffer, charIdx, x, y, esc, escs, -1, blit)
	if t.acceptInput {
		x, y, charIdx, esc, escs = t.drawBuffer(t.prompt, charIdx, x, y, esc, escs, -1, blit)
		x, y, charIdx, esc, escs = t.drawBuffer(t.input, charIdx, x, y, esc, escs, t.cursorIdx, blit)
	}
	return x, y, esc, escs
}

func (t *Terminal) draw() {
	t.selectedString.Reset()

	// predraw - do not blit
	x, y, esc, escs := t.calcDrawBuffers(0, 0, true, "", false)

	minScrollY := 0.0
	maxScrollY := float64(y - t.rows + 1)
	if maxScrollY < 0 {
		maxScrollY = 0
	}

	t.ctx.Call("save")
	defer t.ctx.Call("restore")

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
	t.rowOffset = 0 - y
	t.maxRowOffset = int(maxScrollY)

	// real drawing goes here
	t.drawSelectionBlocks()
	x, esc, escs = 0, true, ""
	x, y, esc, escs = t.calcDrawBuffers(x, y, esc, escs, true)

	if dbgBorder {
		t.ctx.Call("restore")
		t.ctx.Set("strokeStyle", baseColor)
		t.ctx.Call("strokeRect", padx*t.ratio, pady*t.ratio, (float64(t.cols) * t.charWidth * t.ratio), (float64(t.rows)*t.charHeight)*t.ratio)
		t.ctx.Call("save")
	}
	//t.textarea.Get("style").Set("display", "none")
	t.textarea.Set("value", t.selectedString.String())
	t.textarea.Call("focus")
	t.textarea.Call("select")
}
