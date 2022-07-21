package domainfilter

import (
	"bufio"
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	cuckoo "github.com/seiflotfy/cuckoofilter"
)

type DomainFilter struct {
	filter *cuckoo.Filter
}

var (
	lock          sync.Mutex
	DefaultFilter *DomainFilter
	filterInit    sync.Once
	domainList    = []string{
		"https://zerodot1.gitlab.io/CoinBlockerLists/list.txt",
	}
)

func New(domainList []string) *DomainFilter {
	_filter := cuckoo.NewFilter(1000000)
	_df := &DomainFilter{
		filter: _filter,
	}
	_df.AddDomainList(domainList)
	return _df
}

func (d *DomainFilter) Lookup(domain []byte) bool {
	return d.filter.Lookup(domain)
}

func (d *DomainFilter) Reset() {
	d.filter.Reset()
}

func (d *DomainFilter) AddDomainList(domainList []string) {
	var scanner *bufio.Scanner
	addHook := func(v string) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, "GET", v, nil)
		if err != nil {
			return
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return
		}
		if err != nil {
			log.Println(err)
			return
		}
		defer resp.Body.Close()
		scanner = bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			//log.Println(scanner.Text())
			d.filter.InsertUnique(scanner.Bytes())
		}
	}
	for _, v := range domainList {
		addHook(v)
		scanner = nil
	}
}

func getFilter() *DomainFilter {
	filterInit.Do(func() {
		DefaultFilter = New(domainList)
	})
	return DefaultFilter
}

func CheckDomain(hostname []byte) bool {
	return getFilter().Lookup(hostname)
}

func UpgradeFilter() {
	lock.Lock()
	defer lock.Unlock()
	if DefaultFilter != nil {
		DefaultFilter.Reset()
		DefaultFilter.AddDomainList(domainList)
	}
}

func AddFilter(List []string) {
	lock.Lock()
	defer lock.Unlock()
	getFilter().AddDomainList(List)
	domainList = append(domainList, List...)

}
