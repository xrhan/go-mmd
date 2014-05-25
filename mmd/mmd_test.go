package mmd

import (
	"testing"
)

func TestEchoCall(t *testing.T) {
	if testing.Short() {
		t.Skip("network tests disabled")
	}
	mmdc, err := LocalConnect()
	if err != nil {
		t.Fatal(err)
	}
	t.Log("Created mmd connection:", mmdc)
	resp, err := mmdc.Call("echo", "Howdy Doody")
	t.Logf("Response: %+v\nError: %v\n", resp, err)
	t.Log("Shutting down MMD connection")
	mmdc.Close()
}
