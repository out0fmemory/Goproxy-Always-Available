package main

import (
	"fmt"
	"net"
)

func main() {
	host := "www.google.com"
	addrs, err := net.LookupHost(host)
	fmt.Printf("addrs=%#v, err=%#v\n", addrs, err)
}
