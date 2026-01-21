package main

import (
	_ "embed"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"unicode/utf8"

	"github.com/creack/pty"
	"golang.org/x/net/websocket"
)

//go:embed index.html
var index string

func main() {
	http.Handle("/ws", websocket.Handler(handleWs))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(index))
	})
	log.Println("webterm running on localhost:" + env("PORT", "10000"))
	log.Fatalln(http.ListenAndServe("localhost:"+env("PORT", "10000"), nil))
}

func handleWs(ws *websocket.Conn) {
	defer ws.Close()
	conn := &wsUtf8Buffer{conn: ws}
	command := exec.Command(env("SHELL", "bash"))
	command.Dir = env("HOME", "")
	ptyHandle, err := pty.StartWithSize(command, &pty.Winsize{
		Rows: 40,
		Cols: 80,
	})
	if err != nil {
		log.Println("error starting pty", err)
		ws.Close()
		return
	}

	go func() {
		state, err := command.Process.Wait()
		log.Println("command ended", state.ExitCode(), err)
		ptyHandle.Close()
		ptyHandle = nil
	}()

	go func() {
		for {
			data := map[string]interface{}{}
			if err := json.NewDecoder(ws).Decode(&data); err != nil {
				log.Println("error reading message from websocket", err)
				ptyHandle.Close()
				command.Process.Kill()
				return
			}
			event := data["event"].(string)

			if event == "close" {
				ws.Close()
				ptyHandle.Close()
				command.Process.Kill()
				return
			}

			if event == "resize" {
				rows := uint16(data["rows"].(float64))
				cols := uint16(data["cols"].(float64))
				pty.Setsize(ptyHandle, &pty.Winsize{Rows: rows, Cols: cols})
				continue
			}

			if event == "text" {
				_, err := ptyHandle.WriteString(data["text"].(string))
				if err != nil {
					log.Println("error writing to ptyHandle", err)
				}
				continue
			}

			log.Println("unknown event", event)
		}
	}()

	io.Copy(conn, ptyHandle)
	log.Println("done copying")
}

type wsUtf8Buffer struct {
	conn *websocket.Conn
	buf  []byte
}

// Buffer bytes and only write to connection once we have valid UTF8
func (ws *wsUtf8Buffer) Write(b []byte) (i int, err error) {
	ws.buf = append(ws.buf, b...)
	if utf8.Valid(ws.buf) {
		_, err := ws.conn.Write(ws.buf)
		ws.buf = []byte{}
		return len(b), err
	}
	return len(b), nil
}

func env(key, alt string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return alt
}
