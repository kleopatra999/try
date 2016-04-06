package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
)

func main() {
	var port int
	flag.IntVar(&port, "p", 8000, "server port")
	flag.Parse()
	log.Printf("Starting server on port %d", port)
	http.HandleFunc("/tile38-server/", tile38Server)
	http.HandleFunc("/tile38-cli/", tile38CLI)
	http.HandleFunc("/", canvas)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}
