package mmd

import (
	"encoding/hex"
	"math"
	"reflect"
	"testing"
	"time"
)

var allTypes []interface{} = []interface{}{
	"Hello",
	true,
	false,
	0,
	math.MinInt8,
	math.MaxInt8,
	math.MinInt16,
	math.MaxInt16,
	math.MinInt32,
	math.MaxInt32,
	math.MinInt64,
	math.MaxInt64,
	math.MaxUint8,
	math.MaxUint16,
	math.MaxUint32,
	uint64(math.MaxUint64),
	float32(-1.0),
	math.MaxFloat32,
	float64(-1.0),
	math.MaxFloat64,
	[]int{1, 2, 3},
	map[string]interface{}{"ABC": 1, "def": []byte{9, 8, 7}},
	time.Now(),
}

func TestCodecEncode(t *testing.T) {
	buffer := NewBuffer(1024)
	toEncode := allTypes
	t.Log("Encoding", toEncode)
	err := Encode(buffer, toEncode)
	if err != nil {
		t.Fatal(err)
	}
	bytes := buffer.Flip().Bytes()
	t.Logf("Buffer: \n%s", hex.Dump(bytes))
}

func TestCodecEncodeDecode(t *testing.T) {
	toEncode := allTypes
	buffer := NewBuffer(1024)
	err := Encode(buffer, toEncode)
	if err != nil {
		t.Fatal(err)
	}
	read := buffer.Flip()
	t.Logf("Decoding: \n%s", hex.Dump(read.Bytes()))
	decoded, err := Decode(read)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(toEncode, decoded) {
		t.Fatalf("Not equal\n   Orig: %v\nDecoded: %v", toEncode, decoded)
	}
}
