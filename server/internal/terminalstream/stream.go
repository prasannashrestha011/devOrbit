package terminalstream

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"github.com/prasannashrestha011/devOrbit/internal/process"
)

var upgrader = websocket.Upgrader{ReadBufferSize: 4096, WriteBufferSize: 16 << 10}

type message struct {
	Type string `json:"type"`
	Data string `json:"data"`
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

func After(r *http.Request) (uint64, error) {
	values := r.URL.Query()["after"]
	if len(values) == 0 {
		return 0, nil
	}
	if len(values) != 1 || values[0] == "" {
		return 0, process.ErrInvalid
	}
	return strconv.ParseUint(values[0], 10, 64)
}

func Serve(w http.ResponseWriter, r *http.Request, terminal *process.Terminal, after uint64, input func([]byte) error, resize func(uint16, uint16) error) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.SetReadLimit(8192)
	write := func(frame process.TerminalFrame) error {
		_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		return conn.WriteJSON(frame)
	}
	frames, gap, live, unsubscribe := terminal.Subscribe(after)
	defer unsubscribe()
	if gap && write(process.TerminalFrame{Type: "gap"}) != nil {
		return
	}
	for _, frame := range frames {
		if write(frame) != nil {
			return
		}
	}
	readDone := make(chan error, 1)
	go func() {
		for {
			_, body, err := conn.ReadMessage()
			if err != nil {
				readDone <- err
				return
			}
			var request message
			if len(body) > 8192 || json.Unmarshal(body, &request) != nil {
				readDone <- process.ErrInvalid
				return
			}
			switch request.Type {
			case "input":
				err = input([]byte(request.Data))
			case "resize":
				err = resize(request.Cols, request.Rows)
			case "close":
				readDone <- nil
				return
			default:
				err = process.ErrInvalid
			}
			if err != nil {
				readDone <- err
				return
			}
		}
	}()
	for {
		select {
		case frame, ok := <-live:
			if !ok {
				typeName := "stopped"
				if !terminal.Closed() {
					typeName = "gap"
				}
				_ = write(process.TerminalFrame{Type: typeName})
				return
			}
			if write(frame) != nil {
				return
			}
		case err := <-readDone:
			if err != nil && !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				_ = write(process.TerminalFrame{Type: "error"})
			}
			return
		case <-r.Context().Done():
			return
		}
	}
}
