package server

import (
	"bufio"
	"context"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	filter "github.com/BishiNET/ss-server/domainfilter"

	"github.com/BishiNET/ss-server/socks"
	"github.com/kpango/fastime"
)

var (
	cb = context.Background()
)

func resolve(domain []byte) (string, bool, error) {
	if filter.CheckDomain(domain) {
		return "", true, nil
	}
	ctx, cancel := context.WithTimeout(cb, time.Second)
	defer cancel()
	IPs, err := net.DefaultResolver.LookupIP(ctx, "ip4", string(domain))
	if err != nil {
		return "", true, errors.New("cannot find a host")
	}
	if len(IPs) == 0 {
		return "", true, errors.New("no record")
	}
	if len(IPs) > 1 {
		ips := IPs[rand.Intn(len(IPs))]
		if doIPCheck(ips) {
			return "", true, errors.New("private addr")
		}
		return ips.String(), false, nil
	}
	if doIPCheck(IPs[0]) {
		return "", true, errors.New("private addr")
	}
	return IPs[0].String(), false, nil
}

// This function hijacks users' traffic to redirect.
func showBlock(left net.Conn) {
	rc, err := net.Dial("tcp", "")
	if err != nil {
		logf("failed to connect to target: %v", err)
		return
	}
	defer rc.Close()
	_ = relay(left, rc)
}

// Listen on addr for incoming connections.
func (u *User) tcpRemote(isDone chan struct{}, l net.Listener, shadow func(net.Conn) net.Conn) {
	defer func() {
		if err := recover(); err != nil {
			log.Println(err)
		}
	}()

	for {
		c, err := l.Accept()
		if err != nil {
			select {
			case <-isDone:
				logf("TCP Exit...")
				return
			default:
				logf("failed to accept: %v", err)
				continue
			}
		}

		go func() {
			defer c.Close()
			sc := shadow(c)
			tgt, err := socks.ReadAddr(sc)
			if err != nil {
				//logf("failed to get target address from %v: %v", c.RemoteAddr(), err)
				// drain c to avoid leaking server behavioral features
				// see https://www.ndss-symposium.org/ndss-paper/detecting-probe-resistant-proxies/
				_, err = io.Copy(ioutil.Discard, c)
				if err != nil {
					logf("discard error: %v", err)
				}
				return
			}
			host, port, stype, domain, IPs := tgt.String()
			//logf(rAddr)
			var rAddr string
			switch stype {
			case socks.AtypIPv4:
				if doIPCheck(IPs) {
					logf("Someone is trying to visiting the private addr")
					return
				}
				rAddr = net.JoinHostPort(host, port)
			case socks.AtypDomainName:
				real_remote, isBlock, err := resolve(domain)
				if err != nil {
					return
				} else if isBlock {
					showBlock(sc)
					return
				} else {
					rAddr = net.JoinHostPort(real_remote, port)
				}
			default:
				rAddr = net.JoinHostPort(host, port)
			}
			//log.Println(rAddr)
			t1 := fastime.UnixNanoNow()
			rc, err := net.Dial("tcp", rAddr)
			tcpKeepAlive(rc)
			if err != nil {
				logf("failed to connect to target: %v", err)
				return
			}
			defer rc.Close()

			_ = relay(sc, rc)
			t2 := fastime.UnixNanoNow() - t1
			if t2 > 0 {
				_ = atomic.AddInt64(&u.UsedMilliTime, t2/1e6)
			}

		}()
	}
}

// relay copies between left and right bidirectionally
func relay(left, right net.Conn) error {
	var err, err1 error
	var wg sync.WaitGroup
	var wait = 5 * time.Second
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err1 = io.Copy(right, left)
		right.SetReadDeadline(time.Now().Add(wait)) // unblock read on right
	}()
	_, err = io.Copy(left, right)
	left.SetReadDeadline(time.Now().Add(wait)) // unblock read on left
	wg.Wait()

	if err1 != nil && !errors.Is(err1, os.ErrDeadlineExceeded) { // requires Go 1.15+
		return err1
	}
	if err != nil && !errors.Is(err, os.ErrDeadlineExceeded) {
		return err
	}
	return nil
}

type corkedConn struct {
	net.Conn
	bufw   *bufio.Writer
	corked bool
	delay  time.Duration
	err    error
	lock   sync.Mutex
	once   sync.Once
}

func timedCork(c net.Conn, d time.Duration, bufSize int) net.Conn {
	return &corkedConn{
		Conn:   c,
		bufw:   bufio.NewWriterSize(c, bufSize),
		corked: true,
		delay:  d,
	}
}

func (w *corkedConn) Write(p []byte) (int, error) {
	w.lock.Lock()
	defer w.lock.Unlock()
	if w.err != nil {
		return 0, w.err
	}
	if w.corked {
		w.once.Do(func() {
			time.AfterFunc(w.delay, func() {
				w.lock.Lock()
				defer w.lock.Unlock()
				w.corked = false
				w.err = w.bufw.Flush()
			})
		})
		return w.bufw.Write(p)
	}
	return w.Conn.Write(p)
}
