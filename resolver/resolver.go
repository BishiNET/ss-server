package resolver

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	filter "github.com/BishiNET/ss-server/domainfilter"
	"github.com/cornelk/hashmap"
)

var (
	resolverInit    sync.Once
	lock            sync.Mutex
	DefaultResolver *Resolver
	cb              context.Context = context.Background()
)

type Resolver struct {
	dns *hashmap.HashMap
}

type dnsEntry struct {
	IP      string
	IsBlock bool
}

func New() *Resolver {
	return &Resolver{
		dns: hashmap.New(50000),
	}
}

func (r *Resolver) Resolve(domain []byte) (string, bool, error) {
	if v, ok := r.dns.Get(domain); ok {
		return v.(*dnsEntry).IP, v.(*dnsEntry).IsBlock, nil
	}
	if filter.CheckDomain(domain) {
		go func() {
			r.dns.Set(domain, &dnsEntry{
				IP:      "",
				IsBlock: true,
			})
		}()
		return "", true, nil
	}
	ctx, cancel := context.WithTimeout(cb, time.Second)
	defer cancel()
	IPs, err := net.DefaultResolver.LookupIP(ctx, "ip4", string(domain))
	if err != nil {
		return "", true, fmt.Errorf("Cannot find a host")
	}
	ip := IPs[0].String()
	go func() {
		r.dns.Set(domain, &dnsEntry{
			IP:      ip,
			IsBlock: false,
		})
	}()
	return ip, false, nil
}

func (r *Resolver) Reset() {
	lock.Lock()
	defer lock.Unlock()
	r.dns = hashmap.New(50000)
}

func getResolver() *Resolver {
	resolverInit.Do(func() {
		DefaultResolver = New()
	})
	return DefaultResolver
}

func Resolve(domain []byte) (string, bool, error) {
	if filter.CheckDomain(domain) {
		return "", true, nil
	}
	ctx, cancel := context.WithTimeout(cb, time.Second)
	defer cancel()
	IPs, err := net.DefaultResolver.LookupIP(ctx, "ip4", string(domain))
	if err != nil {
		return "", true, fmt.Errorf("Cannot find a host")
	}
	return IPs[0].String(), false, nil
}
