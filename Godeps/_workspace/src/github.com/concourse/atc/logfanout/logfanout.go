package logfanout

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"unicode/utf8"

	"github.com/gorilla/websocket"
)

type LogDB interface {
	BuildLog(build int) ([]byte, error)
	AppendBuildLog(build int, log []byte) error
}

type LogFanout struct {
	build int

	db LogDB

	lock *sync.Mutex

	sinks []*websocket.Conn

	closed bool
}

func NewLogFanout(build int, db LogDB) *LogFanout {
	return &LogFanout{
		build: build,
		db:    db,

		lock: new(sync.Mutex),
	}
}

func (fanout *LogFanout) WriteMessage(msg *json.RawMessage) error {
	fanout.lock.Lock()
	defer fanout.lock.Unlock()

	err := fanout.db.AppendBuildLog(fanout.build, []byte(*msg))
	if err != nil {
		return err
	}

	newSinks := []*websocket.Conn{}
	for _, sink := range fanout.sinks {
		err := sink.WriteJSON(msg)
		if err != nil {
			continue
		}

		newSinks = append(newSinks, sink)
	}

	fanout.sinks = newSinks

	return nil
}

func (fanout *LogFanout) Attach(sink *websocket.Conn) error {
	fanout.lock.Lock()

	buildLog, err := fanout.db.BuildLog(fanout.build)
	if err == nil {
		decoder := json.NewDecoder(bytes.NewBuffer(buildLog))

		for {
			var msg *json.RawMessage
			err := decoder.Decode(&msg)
			if err != nil {
				if err != io.EOF {
					fanout.emitBackwardsCompatible(sink, buildLog)
				}

				break
			}

			err = sink.WriteJSON(msg)
			if err != nil {
				fanout.lock.Unlock()
				return err
			}
		}
	}

	if fanout.closed {
		sink.Close()
	} else {
		fanout.sinks = append(fanout.sinks, sink)
	}

	fanout.lock.Unlock()

	return nil
}

func (fanout *LogFanout) Close() error {
	fanout.lock.Lock()
	defer fanout.lock.Unlock()

	if fanout.closed {
		return errors.New("close twice")
	}

	for _, sink := range fanout.sinks {
		sink.Close()
	}

	fanout.closed = true
	fanout.sinks = nil

	return nil
}

func (fanout *LogFanout) emitBackwardsCompatible(sink *websocket.Conn, log []byte) {
	err := sink.WriteMessage(websocket.TextMessage, []byte(`{"version":"0.0"}`))
	if err != nil {
		return
	}

	var dangling []byte
	for i := 0; i < len(log); i += 1024 {
		end := i + 1024
		if end > len(log) {
			end = len(log)
		}

		text := append(dangling, log[i:end]...)

		checkEncoding, _ := utf8.DecodeLastRune(text)
		if checkEncoding == utf8.RuneError {
			dangling = text
			continue
		}

		err := sink.WriteMessage(websocket.TextMessage, text)
		if err != nil {
			return
		}

		dangling = nil
	}
}
