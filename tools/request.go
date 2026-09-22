package main

import (
	"encoding/hex"
	"fmt"
	"time"
)

func main() {
	targetTimeStr := "2028-10-08 00:00:00"
	loc, _ := time.LoadLocation("Local")
	t, _ := time.ParseInLocation("2006-01-02 15:04:05", targetTimeStr, loc)

	tsStr := fmt.Sprintf("%d", t.Unix())

	mask := []byte{0x0A, 0x08, 0x03, 0x00, 0x02, 0x09, 0x07, 0x06, 0x00, 0x00}

	cipher := make([]byte, len(mask))
	for i := 0; i < len(mask); i++ {
		cipher[i] = tsStr[i] ^ mask[i]
	}

	fmt.Println("new _tSec:")
	fmt.Printf("%q\n", hex.EncodeToString(cipher))
}
