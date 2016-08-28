package main

import (
	"fmt"
	"net"
)

func main() {
	v, err := net.InterfaceAddrs()
	fmt.Printf("%+v, err=%#v\n", v, err)
}
