package main

import (
	"github.com/gopherjs/gopherjs/js"
	"github.com/tile38/play/console"
)

func main() {
	sheet := js.Global.Get("document").Call("createElement", "style")
	sheet.Set("innerHTML",
		`html, body { 
			background: black; 
			padding:0; 
			margin:0; 
			border:0; 
			width:100%; 
			height:100%; 
			overflow:hidden;
		}`)
	js.Global.Get("document").Get("head").Call("appendChild", sheet)
	js.Global.Get("document").Set("title", "Tile38 Playground")
	js.Global.Call("addEventListener", "load", func() {
		_, err := console.New(js.Global.Get("document").Get("body"), "tile38")
		if err != nil {
			println(err.Error())
			return
		}
	})
}
