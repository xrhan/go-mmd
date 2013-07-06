package mmd

import "testing"

// import "time"

/*func TestConnect(t *testing.T) {
	con := LocalConnect()
	con.Close()
}
*/
func TestEchoCall(t *testing.T) {
	mmdc := LocalConnect()
	t.Log("Created mmd connection:", mmdc)
	resp, err := mmdc.Call("echo", "Howdy Doody")
	t.Logf("Response: %+v\nError: %v\n", resp, err)
	t.Log("Shutting down MMD connection")
	mmdc.Close()
}

func TestEncode(t *testing.T) {
	buffer := NewBuffer(1024)
	toEncode := "Hello"
	// toEncode := []interface {
	// 	0x01,
	// 	0xFF,
	// 	0xFFFF,
	// 	0xFFFFFFFF,
	// 	0xFFFFFFFFFFFFFFFF,
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

}
