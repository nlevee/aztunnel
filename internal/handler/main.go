package handler

import "net"

type TunnelHandler interface {
	Handle(l net.Listener) error
}
