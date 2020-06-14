package main

import (
	"flag"
	"log"
)

var addr = flag.String("addr", ":8088", "http server address")

func init() {
	log.SetFlags(log.Ldate | log.Lshortfile)
}

func main() {
	flag.Parse()
	s := newServer(*addr)
	s.run()
}
