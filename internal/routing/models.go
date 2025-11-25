package routing

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"time"
)

type PlayingState struct {
	IsPaused bool
}

type GameLog struct {
	CurrentTime time.Time
	Message     string
	Username    string
}

func Encode(gl GameLog) ([]byte, error) {
	var data bytes.Buffer

	enc := gob.NewEncoder(&data)
	err := enc.Encode(gl)

	if err != nil {
		fmt.Printf("Could not encode %s", err.Error())
		return []byte{}, err
	}

	return data.Bytes(), nil
}

func Decode(data []byte) (GameLog, error) {
	var g GameLog
	b := bytes.NewBuffer(data)
	dec := gob.NewDecoder(b)
	err := dec.Decode(&g)

	if err != nil {
		return g, err
	}

	return g, nil
}
