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
	http.HandleFunc("/server/", server)
	http.HandleFunc("/cli/", cli)
	http.HandleFunc("/", canvas)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}
