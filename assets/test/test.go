package main

import (
	"fmt"
	"net"
)

func main() {
	v, err := net.LookupIP("www.google.com")
	fmt.Printf("%+v, err=%#v\n", v, err)
}
