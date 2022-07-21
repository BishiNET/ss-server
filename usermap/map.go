package usermap

import (
	"fmt"
	"sync"

	"github.com/BishiNET/ss-server/server"
)

type UserMap map[string]*server.User

var (
	// Read Lock protects the user map for reading.
	// atomic protects the internal vars.
	rwlock sync.RWMutex
)

func NewMap() UserMap {
	return UserMap{}
}

func (u UserMap) Exists(name string) bool {
	rwlock.RLock()
	defer rwlock.RUnlock()
	_, ok := u[name]
	return ok
}
func (u UserMap) AddUser(name, cipher, password, port string) error {
	user_entry, err := server.New(cipher, fmt.Sprintf("0.0.0.0:%s", port), password)
	if err != nil {
		return err
	}
	rwlock.Lock()
	defer rwlock.Unlock()
	u[name] = user_entry
	return nil
}

func (u UserMap) DeleteUser(name string) {
	rwlock.RLock()
	defer rwlock.RUnlock()
	u[name].Shutdown()
	delete(u, name)
}

func (u UserMap) ResetUser(name string) {
	rwlock.RLock()
	defer rwlock.RUnlock()
	u[name].Reset()
}

func (u UserMap) ResetUserTraffic(name string) {
	rwlock.RLock()
	defer rwlock.RUnlock()
	u[name].ResetTraffic()
}
func (u UserMap) ResetUserTime(name string) {
	rwlock.RLock()
	defer rwlock.RUnlock()
	u[name].ResetTime()
}

func (u UserMap) ResetAll() {
	rwlock.RLock()
	defer rwlock.RUnlock()
	for _, v := range u {
		v.Reset()
	}
}
func (u UserMap) GetUser(name string) (uint64, int64) {
	rwlock.RLock()
	defer rwlock.RUnlock()
	return u[name].Get()
}

func (u UserMap) GetUserTraffic(name string) uint64 {
	rwlock.RLock()
	defer rwlock.RUnlock()
	return u[name].GetTraffic()
}

func (u UserMap) GetUserUsedTime(name string) int64 {
	rwlock.RLock()
	defer rwlock.RUnlock()
	return u[name].GetUsedTime()
}

func (u UserMap) SetUser(name string, traffic uint64, time int64) {
	rwlock.RLock()
	defer rwlock.RUnlock()
	u[name].Set(traffic, time)
}

func (u UserMap) GetAll(executor func(name string, traffic uint64, usedtime int64)) {
	rwlock.RLock()
	defer rwlock.RUnlock()
	for k, v := range u {
		executor(k, v.GetTraffic(), v.GetUsedTime())
	}
}
