package server

import (
	"net"
	"time"
)

var (
	_, local1, _ = net.ParseCIDR("10.0.0.0/8")
	_, local2, _ = net.ParseCIDR("172.16.0.0/12")
	_, local3, _ = net.ParseCIDR("192.168.0.0/16")
	_, local4, _ = net.ParseCIDR("127.0.0.0/8")
	_, local5, _ = net.ParseCIDR("0.0.0.0/8")
)

// Source: https://github.com/Dreamacro/clash/blob/master/adapter/outbound/util.go#L14
func tcpKeepAlive(c net.Conn) {
	if tcp, ok := c.(*net.TCPConn); ok {
		tcp.SetKeepAlive(true)
		tcp.SetKeepAlivePeriod(30 * time.Second)
	}
}

// Thanks to https://github.com/shadowsocks/go-shadowsocks2/pull/233
func doIPCheck(ip net.IP) bool {
	return local1.Contains(ip) || local2.Contains(ip) || local3.Contains(ip) || local4.Contains(ip) || local5.Contains(ip)
}
