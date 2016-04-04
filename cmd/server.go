package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

var portmu sync.Mutex
var port = 9850

func nextPort() int {
	portmu.Lock()
	defer portmu.Unlock()
	port++
	if port > 49151 {
		port = 9851
	}
	return port
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

var shmu sync.Mutex
var idmap = make(map[string]int)

func server(w http.ResponseWriter, r *http.Request) {
	var invalidid string
	var id string
	idp := strings.Split(r.URL.Path, "/")
	if len(idp) >= 3 {
		id = idp[2]
	}

	if id != "" {
		fi, err := os.Stat(path.Join("data", id))
		if err != nil || !fi.IsDir() {
			invalidid = id
			id = ""
		}
	}
	if id == "" {
		rb := make([]byte, 16)
		if _, err := io.ReadFull(rand.Reader, rb); err != nil {
			log.Print(err)
			return
		}
		id = hex.EncodeToString(rb)
	}

	var wrmu sync.Mutex
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()
	if invalidid != "" {
		if err := conn.WriteMessage(websocket.TextMessage, []byte("invalidid: "+invalidid)); err != nil {
			log.Print(err)
			return
		}

	}
	if err := conn.WriteMessage(websocket.TextMessage, []byte("id: "+id)); err != nil {
		log.Print(err)
		return
	}

	shmu.Lock()
	port := nextPort()
	for i := 0; i < 50000; i++ {
		l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			port = nextPort()
			continue
		}
		l.Close()
		break
	}
	if err := os.MkdirAll("data", 0700); err != nil {
		shmu.Unlock()
		log.Print(err)
		return
	}
	if idmap[id] != 0 {
		if err := conn.WriteMessage(websocket.TextMessage, []byte("err: server already started")); err != nil {
			log.Print(err)
		}
		shmu.Unlock()
		return
	}
	cmd := exec.Command("tile38-server", "-p", fmt.Sprintf("%d", port), "-d", path.Join("data", id))
	erd, err := cmd.StderrPipe()
	if err != nil {
		shmu.Unlock()
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
		shmu.Unlock()
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
	go func() {
		defer func() {
			wrmu.Lock()
			cmd.Process.Kill()
			wrmu.Unlock()
		}()
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}()
	if err := cmd.Start(); err != nil {
		shmu.Unlock()
		log.Printf("error: %s", err.Error())
		return
	}
	idmap[id] = port
	shmu.Unlock()

	log.Printf("started tile38-server %s", id)
	defer func() {
		shmu.Lock()
		delete(idmap, id)
		shmu.Unlock()
		log.Printf("stopped tile38-server %s", id)
	}()
	if err := cmd.Wait(); err != nil {
		if err.Error() != "signal: killed" {
			log.Printf("error: %s", err.Error())
		}
		return
	}
}
