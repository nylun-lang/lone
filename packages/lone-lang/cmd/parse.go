package main

import (
	"encoding/json"
	"fmt"
	p "lonelang/parser"
	"os"
)

func main() {
	f := os.Args[1]
	if _, err := os.Lstat(f); err != nil {
		panic(fmt.Errorf("file '%s' does not exist; err=%s", f, err.Error()))
	}

	program, err := p.ParseFile(f)
	if err != nil {
		panic(fmt.Errorf("parsing error: %s", err.Error()))
	}

	jsounOut, err := json.MarshalIndent(&program, " ", " ")
	if err != nil {
		panic(fmt.Errorf("json error: %s", err.Error()))
	}

	fmt.Printf("%s", jsounOut)
}
