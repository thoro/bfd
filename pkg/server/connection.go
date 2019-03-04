package server

import (
	"net"
	"os"
)

type Connection interface {
	File() (f *os.File, err error)
	ReadMsgUDP(b, oob []byte) (n, oobn, flags int, addr *net.UDPAddr, err error)
	Write(b []byte) (int, error)
}
