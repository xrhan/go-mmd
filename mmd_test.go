package mmd

import (
	"encoding/hex"
	"testing"
	"time"
)

func TestEchoCall(t *testing.T) {
	if testing.Short() {
		t.Skip("network tests disabled")
	}
	mmdc := LocalConnect()
	t.Log("Created mmd connection:", mmdc)
	resp, err := mmdc.Call("echo", "Howdy Doody")
	t.Logf("Response: %+v\nError: %v\n", resp, err)
	t.Log("Shutting down MMD connection")
	mmdc.Close()
}

func TestEncode(t *testing.T) {
	buffer := NewBuffer(1024)
	toEncode := []interface{}{
		"Hello",
		0,
		0x7F,
		0x7FFF,
		0x7FFFFFFF,
		0x7FFFFFFFFFFFFFFF,
		true,
		false,
		[]int{1, 2, 3},
		map[string]interface{}{"ABC": 1, "def": []byte{9, 8, 7}},
		time.Now(),
	}
	// toEncode := []interface{} {
	// 	0x01,
	// 	true,
	// 	false,
	// 	"hello",
	// 	[]int{1,2,3},
	// 	// map[string] int {
	// 	// 	"Hello":123,
	// 	// 	"Bye":456,
	// 	// },
	// 	time.Now(),
	// 	}
	t.Log("Encoding", toEncode)
	err := Encode(buffer, toEncode)
	if err != nil {
		t.Fatal(err)
	}
	bytes := buffer.Flip().Bytes()
	t.Log("bytes", len(bytes), cap(bytes))
	t.Logf("Buffer: \n%s", hex.Dump(bytes))
}
