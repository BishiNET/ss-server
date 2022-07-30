package server

import (
	"net"
	"time"
)

// Source: https://github.com/Dreamacro/clash/blob/master/adapter/outbound/util.go#L14
func tcpKeepAlive(c net.Conn) {
	if tcp, ok := c.(*net.TCPConn); ok {
		tcp.SetKeepAlive(true)
		tcp.SetKeepAlivePeriod(30 * time.Second)
	}
}

func doIPCheck(ip net.IP) bool {
	if ip4 := ip.To4(); ip4 != nil {
		return ip4[0] == 10 || ip4[0] == 127 || ip4[0] == 0 ||
			(ip4[0] == 172 && ip4[1]&0xf0 == 16) ||
			(ip4[0] == 192 && ip4[1] == 168)
	}
	return len(ip) == net.IPv6len && (ip[0]&0xfe == 0xfc || ip.Equal(net.IPv6loopback))
}
