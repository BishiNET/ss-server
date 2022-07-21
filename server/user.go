package server

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	reuse "github.com/libp2p/go-reuseport"
)

type User struct {
	Traffic       uint64
	UsedMilliTime int64
	Signal        chan struct{}
	lock          sync.Mutex
}

var (
	availableCipher = []string{
		"AES-128-GCM", "AES-256-GCM", "CHACHA20-IETF-POLY1305",
	}
)

func checkCipher(cipher string) bool {
	cipher = strings.ToUpper(cipher)
	i := sort.SearchStrings(availableCipher, cipher)
	if i < len(availableCipher) && availableCipher[i] == cipher {
		return true
	}
	return false
}

func New(cipher, addr, password string) (*User, error) {
	//Check Cipher
	if !checkCipher(cipher) {
		return nil, fmt.Errorf("invalid cipher")
	}
	sig := make(chan struct{})
	user := &User{
		Signal: sig,
	}
	go user.NewServer(sig, cipher, addr, password)
	return user, nil
}

func (u *User) NewServer(userSignal chan struct{}, cipher, addr, password string) {
	tcpDone := make(chan struct{})
	udpDone := make(chan struct{})
	tcpListener, err1 := reuse.Listen("tcp", addr)
	udpListener, err2 := reuse.ListenPacket("udp", addr)

	if err1 != nil || err2 != nil {
		logf("failed to listen on %s: ", addr)
		return
	}

	ciph, err := PickCipher(cipher, nil, password, u)
	if err != nil {
		log.Println(err)
		return
	}
	go u.udpRemote(udpDone, udpListener, ciph.PacketConn)
	go u.tcpRemote(tcpDone, tcpListener, ciph.StreamConn)
	<-userSignal
	close(tcpDone)
	tcpListener.Close()
	close(udpDone)
	udpListener.Close()

}

func (u *User) Shutdown() {
	close(u.Signal)
}

// These methods are THREAD-SAFE.
// Nevermind using them.
func (u *User) Reset() {
	atomic.StoreUint64(&u.Traffic, 0)
	atomic.StoreInt64(&u.UsedMilliTime, 0)
}
func (u *User) ResetTraffic() {
	atomic.StoreUint64(&u.Traffic, 0)
}

func (u *User) ResetTime() {
	atomic.StoreInt64(&u.UsedMilliTime, 0)
}

// just keep it for futher uses
func (u *User) AddToTraffic(traffic int64) {
	_ = atomic.AddUint64(&u.Traffic, uint64(traffic))
}

func (u *User) AddToTime(MilliTime int64) {
	_ = atomic.AddInt64(&u.UsedMilliTime, MilliTime)
}

func (u *User) AddToTrafficInt(traffic int) {
	_ = atomic.AddUint64(&u.Traffic, uint64(traffic))
}

func (u *User) Set(traffic uint64, usedtime int64) {
	atomic.StoreUint64(&u.Traffic, traffic)
	atomic.StoreInt64(&u.UsedMilliTime, usedtime)
}
func (u *User) Get() (uint64, int64) {
	return atomic.LoadUint64(&u.Traffic), atomic.LoadInt64(&u.UsedMilliTime)
}

func (u *User) GetTraffic() uint64 {
	return atomic.LoadUint64(&u.Traffic)
}

func (u *User) GetUsedTime() int64 {
	return atomic.LoadInt64(&u.UsedMilliTime)
}
