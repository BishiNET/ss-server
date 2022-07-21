package rpcapi

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/rpc"
	"strconv"
	"time"

	filter "github.com/BishiNET/ss-server/domainfilter"
	R "github.com/BishiNET/ss-server/rpcinterface"
	u "github.com/BishiNET/ss-server/usermap"
	"github.com/go-redis/redis/v8"
	reuse "github.com/libp2p/go-reuseport"
)

var (
	ctx = context.Background()
)

const (
	NO_ERROR = iota
	USER_EXISTS
	USER_NON_EXISTS
	PARAMS_ERROR
)

type UserRpc struct {
	Users u.UserMap
	rdb   *redis.Client
}

func mustPing(rdb *redis.Client) {
	_ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := rdb.Ping(_ctx).Result()
	if err != nil {
		log.Fatalln(err)
	}
}
func New(rpcAddr string, redisOpts ...string) *UserRpc {
	var opts *redis.Options
	switch len(redisOpts) {
	case 1:
		opts = &redis.Options{
			Addr: redisOpts[0],
		}
	case 2:
		opts = &redis.Options{
			Addr:     redisOpts[0],
			Password: redisOpts[1],
		}
	case 3:
		db, _ := strconv.Atoi(redisOpts[2])
		opts = &redis.Options{
			Addr:     redisOpts[0],
			Password: redisOpts[1],
			DB:       db,
		}
	default:
		opts = &redis.Options{
			Addr:     "localhost:6379",
			Password: "",
		}
	}
	rdb := redis.NewClient(opts)
	mustPing(rdb)

	_uRpc := &UserRpc{
		Users: u.NewMap(),
		rdb:   rdb,
	}
	rpc.Register(_uRpc)
	rpc.HandleHTTP()
	l, e := reuse.Listen("tcp", rpcAddr)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
	return _uRpc
}

func (r *UserRpc) RedisClose() {
	r.rdb.Close()
}

func (r *UserRpc) GetUserInfo(name string) (string, string, string, error) {
	cipher, err1 := r.rdb.HGet(ctx, name, "cipher").Result()
	password, err2 := r.rdb.HGet(ctx, name, "password").Result()
	port, err3 := r.rdb.HGet(ctx, name, "port").Result()
	if err1 != nil || err2 != nil || err3 != nil {
		return "", "", "", fmt.Errorf("user doesn't exist")
	}
	return cipher, password, port, nil
}

func (r *UserRpc) GetAll() R.TrafficReply {
	_users := R.TrafficReply{}
	executor := func(name string, traffic uint64, usedtime int64) {
		_users[name] = R.SingleTrafficReply{
			Traffic:  traffic,
			UsedTime: usedtime,
		}
	}
	r.Users.GetAll(executor)
	return _users
}

func (r *UserRpc) FastRestore() {
	userSlices, err := r.rdb.Keys(ctx, "*").Result()
	if err != nil {
		log.Fatal(err)
	}
	for _, name := range userSlices {
		log.Println("Restart "+name, r.startUser(name))
	}

}

func (r *UserRpc) startUser(name string) error {
	cipher, password, port, err := r.GetUserInfo(name)
	if err != nil {
		//log.Println(err)
		r.rdb.Del(ctx, name)
		return err
	}
	traffic, err := r.rdb.HGet(ctx, name, "traffic").Uint64()
	if err != nil {
		traffic = 0
	}
	time, err := r.rdb.HGet(ctx, name, "time").Int64()
	if err != nil {
		time = 0
	}

	err = r.Users.AddUser(name, cipher, password, port)
	if err != nil {
		//Invalid situation
		//so remove the user
		r.rdb.Del(ctx, name)
		return fmt.Errorf("params error")
	}
	r.Users.SetUser(name, traffic, time)
	return nil
}
func (r *UserRpc) Restore(args *R.NoArgs, reply *R.CallReply) error {
	userSlices, err := r.rdb.Keys(ctx, "*").Result()
	if err != nil {
		reply = &R.CallReply{
			ErrCode:   PARAMS_ERROR,
			ErrReason: "PARAMS ERROR",
		}
		return fmt.Errorf("params error")
	}
	for _, name := range userSlices {
		if err != nil {
			log.Println("Restart"+name, r.startUser(name))
		} else {
			err = r.startUser(name)
		}
	}
	if err != nil {
		reply = &R.CallReply{
			ErrCode:   PARAMS_ERROR,
			ErrReason: err.Error(),
		}
		return err
	}
	reply = &R.CallReply{
		ErrCode: NO_ERROR,
	}
	return nil
}
func (r *UserRpc) AddUser(args *R.NewUserArgs, reply *R.CallReply) error {
	if r.Users.Exists(args.Name) {
		log.Println("User has already existed")
		reply = &R.CallReply{
			ErrCode:   USER_EXISTS,
			ErrReason: "user has already existed",
		}
		return fmt.Errorf("user has already existed")
	}
	r.rdb.HSet(ctx, args.Name, "cipher", args.Cipher)
	r.rdb.HSet(ctx, args.Name, "password", args.Password)
	r.rdb.HSet(ctx, args.Name, "port", args.Port)
	err := r.Users.AddUser(args.Name, args.Cipher, args.Password, args.Port)
	if err != nil {
		reply = &R.CallReply{
			ErrCode:   PARAMS_ERROR,
			ErrReason: "PARAMS ERROR",
		}
		r.rdb.Del(ctx, args.Name)
		return fmt.Errorf("params error")
	}
	log.Println("Add User: " + args.Name)
	reply = &R.CallReply{
		ErrCode: NO_ERROR,
	}
	return nil
}
func (r *UserRpc) StartUser(args *R.CommonArgs, reply *R.CallReply) error {
	err := r.startUser(args.Name)
	if err != nil {
		reply = &R.CallReply{
			ErrCode:   PARAMS_ERROR,
			ErrReason: err.Error(),
		}
		return err
	}
	log.Println("Start User" + args.Name)
	reply = &R.CallReply{
		ErrCode: NO_ERROR,
	}
	return nil
}
func (r *UserRpc) StopUser(args *R.CommonArgs, reply *R.CallReply) error {
	if !r.Users.Exists(args.Name) {
		reply = &R.CallReply{
			ErrCode:   USER_NON_EXISTS,
			ErrReason: "user doesn't exist",
		}
		return fmt.Errorf("user doesn't exist")
	}
	traffic, time := r.Users.GetUser(args.Name)
	r.rdb.HSet(ctx, args.Name, "traffic", traffic)
	r.rdb.HSet(ctx, args.Name, "time", time)
	r.Users.DeleteUser(args.Name)
	reply = &R.CallReply{
		ErrCode: NO_ERROR,
	}
	log.Println("Stop User" + args.Name)
	return nil
}

func (r *UserRpc) DeleteUser(args *R.CommonArgs, reply *R.CallReply) error {
	if !r.Users.Exists(args.Name) {
		reply = &R.CallReply{
			ErrCode:   USER_NON_EXISTS,
			ErrReason: "user doesn't exist",
		}
		return fmt.Errorf("user doesn't exist")
	}
	r.rdb.Del(ctx, args.Name)
	r.Users.DeleteUser(args.Name)
	reply = &R.CallReply{
		ErrCode: NO_ERROR,
	}
	log.Println("Delete User" + args.Name)
	return nil
}

func (r *UserRpc) Modify(args *R.CommonArgs, reply *R.CallReply) error {
	if !r.Users.Exists(args.Name) {
		reply = &R.CallReply{
			ErrCode:   USER_NON_EXISTS,
			ErrReason: "user doesn't exist",
		}
		return fmt.Errorf("user doesn't exist")
	}
	cipher, password, port, err := r.GetUserInfo(args.Name)
	if err != nil {
		reply = &R.CallReply{
			ErrCode:   PARAMS_ERROR,
			ErrReason: "PARAMS ERROR",
		}
		log.Println(err)
		return err
	}

	if args.Password != "" {
		if args.Password != password {
			password = args.Password
			r.rdb.HSet(ctx, args.Name, "password", args.Password)
		}

	}
	if args.Cipher != "" {
		if args.Cipher != cipher {
			cipher = args.Cipher
			r.rdb.HSet(ctx, args.Name, "cipher", args.Cipher)
		}
	}

	if args.Password == password && args.Cipher == cipher {
		reply = &R.CallReply{
			ErrCode:   PARAMS_ERROR,
			ErrReason: "nothing is modfied",
		}
		//log.Println(err)
		return fmt.Errorf("nothing is modfied")
	}

	tmp := r.Users[args.Name]
	err = r.Users.AddUser(args.Name, cipher, password, port)
	if err != nil {
		reply = &R.CallReply{
			ErrCode:   PARAMS_ERROR,
			ErrReason: "PARAMS ERROR",
		}
		return fmt.Errorf("params error")
	}
	traffic, time := tmp.Get()
	r.Users.SetUser(args.Name, traffic, time)
	tmp.Shutdown()
	tmp = nil
	log.Println("User Change password" + args.Name)
	reply = &R.CallReply{
		ErrCode: NO_ERROR,
	}
	return nil
}

func (r *UserRpc) GetUser(args *R.CommonArgs, reply *R.TrafficReply) error {
	if args.Name != "" {
		if !r.Users.Exists(args.Name) {
			return fmt.Errorf("user doesn't exist")
		}
		ut := R.TrafficReply{}
		traffic, time := r.Users.GetUser(args.Name)
		ut[args.Name] = R.SingleTrafficReply{
			Traffic:  traffic,
			UsedTime: time,
		}
		//log.Println(traffic, time)
		*reply = ut
		return nil
	}
	ut := r.GetAll()
	*reply = ut
	return nil
}

func (r *UserRpc) ResetAll(args *R.NoArgs, reply *R.CallReply) error {
	r.Users.ResetAll()
	reply = &R.CallReply{
		ErrCode: NO_ERROR,
	}
	return nil
}

func (r *UserRpc) UpgradeFilter(args *R.NoArgs, reply *R.CallReply) error {
	filter.UpgradeFilter()
	reply = &R.CallReply{
		ErrCode: NO_ERROR,
	}
	return nil
}

func (r *UserRpc) AddFilter(args *R.Filters, reply *R.CallReply) error {
	if args.URL != nil {
		filter.AddFilter(args.URL)
	}
	reply = &R.CallReply{
		ErrCode: NO_ERROR,
	}
	return nil
}
