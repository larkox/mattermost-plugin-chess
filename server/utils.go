package main

import (
	"encoding/json"
	"fmt"
)

func dumpObject(v interface{}) {
	b, _ := json.MarshalIndent(v, "", "    ")
	fmt.Println(string(b))
}
