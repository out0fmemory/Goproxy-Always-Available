package main

import (
	"fmt"
	"net"
)

func main() {
	v, err := net.LookupIP("localhost")
	fmt.Printf("%+v, err=%#v\n", v, err)
}
