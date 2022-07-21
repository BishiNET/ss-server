package server

import (
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	filter "github.com/BishiNET/ss-server/domainfilter"
	"github.com/BishiNET/ss-server/socks"
	"github.com/kpango/fastime"
)

type mode int

const (
	remoteServer mode = iota
	relayClient
	socksClient
)

const udpBufSize = 64 * 1024

// Listen on addr for encrypted packets and basically do UDP NAT.
func (u *User) udpRemote(isDone chan struct{}, c net.PacketConn, shadow func(net.PacketConn) net.PacketConn) {
	defer func() {
		if err := recover(); err != nil {
			log.Println(err)
		}
	}()
	c = shadow(c)

	nm := newNATmap(5 * time.Minute)
	buf := make([]byte, udpBufSize)
	//var t1 int64
	for {
		n, raddr, err := c.ReadFrom(buf)
		if err != nil {
			select {
			case <-isDone:
				logf("UDP Exit...")
				return
			default:
				logf("UDP remote read error: %v", err)
				continue
			}
		}

		tgtAddr := socks.SplitAddr(buf[:n])
		if tgtAddr == nil {
			logf("failed to split target address from packet: %q", buf[:n])
			continue
		}

		host, port, stype, domain, IPs := tgtAddr.String()
		rAddr := net.JoinHostPort(host, port)
		switch stype {
		case socks.AtypIPv4:
			if doIPCheck(IPs) {
				continue
			}

		case socks.AtypDomainName:
			if filter.CheckDomain(domain) {
				continue
			}
		}
		t1 := fastime.UnixNanoNow()
		tgtUDPAddr, err := net.ResolveUDPAddr("udp", rAddr)
		if err != nil {
			logf("failed to resolve target UDP address: %v", err)
			continue
		}

		payload := buf[len(tgtAddr):n]

		pc := nm.Get(raddr.String())
		if pc == nil {
			pc, err = net.ListenPacket("udp", "")
			if err != nil {
				logf("UDP remote listen error: %v", err)
				continue
			}

			nm.Add(raddr, c, pc, remoteServer)
		}
		_, err = pc.WriteTo(payload, tgtUDPAddr) // accept only UDPAddr despite the signature
		if err != nil {
			logf("UDP remote write error: %v", err)
			continue
		}
		t2 := fastime.UnixNanoNow() - t1
		if t2 > 0 {
			_ = atomic.AddInt64(&u.UsedMilliTime, t2/1e6)
		}
	}
}

// Packet NAT table
type natmap struct {
	sync.RWMutex
	m       map[string]net.PacketConn
	timeout time.Duration
}

func newNATmap(timeout time.Duration) *natmap {
	m := &natmap{}
	m.m = make(map[string]net.PacketConn)
	m.timeout = timeout
	return m
}

func (m *natmap) Get(key string) net.PacketConn {
	m.RLock()
	defer m.RUnlock()
	return m.m[key]
}

func (m *natmap) Set(key string, pc net.PacketConn) {
	m.Lock()
	defer m.Unlock()

	m.m[key] = pc
}

func (m *natmap) Del(key string) net.PacketConn {
	m.Lock()
	defer m.Unlock()

	pc, ok := m.m[key]
	if ok {
		delete(m.m, key)
		return pc
	}
	return nil
}

func (m *natmap) Add(peer net.Addr, dst, src net.PacketConn, role mode) {
	m.Set(peer.String(), src)

	go func() {
		timedCopy(dst, peer, src, m.timeout, role)
		if pc := m.Del(peer.String()); pc != nil {
			pc.Close()
		}
	}()
}

// copy from src to dst at target with read timeout
func timedCopy(dst net.PacketConn, target net.Addr, src net.PacketConn, timeout time.Duration, role mode) error {
	buf := make([]byte, udpBufSize)

	for {
		src.SetReadDeadline(time.Now().Add(timeout))
		n, raddr, err := src.ReadFrom(buf)
		if err != nil {
			return err
		}

		switch role {
		case remoteServer: // server -> client: add original packet source
			srcAddr := socks.ParseAddr(raddr.String())
			copy(buf[len(srcAddr):], buf[:n])
			copy(buf, srcAddr)
			_, err = dst.WriteTo(buf[:len(srcAddr)+n], target)
		case relayClient: // client -> user: strip original packet source
			srcAddr := socks.SplitAddr(buf[:n])
			_, err = dst.WriteTo(buf[len(srcAddr):n], target)
		case socksClient: // client -> socks5 program: just set RSV and FRAG = 0
			_, err = dst.WriteTo(append([]byte{0, 0, 0}, buf[:n]...), target)
		}

		if err != nil {
			return err
		}
	}
}
