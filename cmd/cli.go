package main

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

func cli(w http.ResponseWriter, r *http.Request) {
	var id string
	idp := strings.Split(r.URL.Path, "/")
	if len(idp) >= 3 {
		id = idp[2]
	}

	shmu.Lock()
	port := idmap[id]
	shmu.Unlock()
	if port == 0 {
		log.Printf("invalid id '%s'", id)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()

	var wrmu sync.Mutex

	cmd := exec.Command("tile38-cli", "-p", fmt.Sprintf("%d", port), "--raw")
	erd, err := cmd.StderrPipe()
	if err != nil {
		log.Printf("error: %s", err.Error())
		return
	}
	defer erd.Close()

	go func() {
		defer func() {
			wrmu.Lock()
			cmd.Process.Kill()
			wrmu.Unlock()
		}()
		rd := bufio.NewReader(erd)
		for {
			line, err := rd.ReadBytes('\n')
			if err != nil {
				return
			}
			wrmu.Lock()
			err = conn.WriteMessage(websocket.TextMessage, append([]byte(`stderr: `), line...))
			wrmu.Unlock()
			if err != nil {
				return
			}
		}
	}()

	ord, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("error: %s", err.Error())
		return
	}
	defer ord.Close()
	go func() {
		defer func() {
			wrmu.Lock()
			cmd.Process.Kill()
			wrmu.Unlock()
		}()
		rd := bufio.NewReader(ord)
		for {
			line, err := rd.ReadBytes('\n')
			if err != nil {
				return
			}
			wrmu.Lock()
			err = conn.WriteMessage(websocket.TextMessage, append([]byte(`stdout: `), line...))
			wrmu.Unlock()
			if err != nil {
				return
			}
		}
	}()
	iwr, err := cmd.StdinPipe()
	if err != nil {
		log.Print(err)
		return
	}
	defer iwr.Close()

	if err := cmd.Start(); err != nil {
		log.Print(err)
		return
	}

	go func() {
		defer func() {
			wrmu.Lock()
			cmd.Process.Kill()
			wrmu.Unlock()
		}()
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}

			_, err = fmt.Fprintf(iwr, "%s\n", string(msg))
			if err != nil {
				return
			}
		}
	}()

	log.Printf("started tile38-cli %s", id)
	defer func() {
		log.Printf("stopped tile38-cli %s", id)
	}()
	if err := cmd.Wait(); err != nil {
		if err.Error() != "signal: killed" {
			log.Print(err)
		}
		return
	}

}
