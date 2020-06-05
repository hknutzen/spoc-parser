package main

import (
	//	"encoding/json"
	"fmt"
	"github.com/hknutzen/spoc-parser/parser"
	"github.com/hknutzen/spoc-parser/printer"
	"io/ioutil"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage:\n  %s filename\n", os.Args[0])
		os.Exit(1)
	}
	path := os.Args[1]
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Can't %s\n", err)
		os.Exit(1)
	}
	list := parser.ParseFile(bytes, path)
	printer.File(list, bytes)
	/*	json, err := json.MarshalIndent(list, "", " ")
		if err != nil {
			panic(err)
		}
		fmt.Println(string(json))
	*/
}
