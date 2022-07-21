package rpcinterface

type CallReply struct {
	ErrCode   int
	ErrReason string
}

type Auth struct {
	AccessID    string
	AccessToken string
	Sign        string
}
type NewUserArgs struct {
	Name     string
	Cipher   string
	Password string
	Port     string
}

type CommonArgs struct {
	Name     string
	Password string
	Cipher   string
}

type NoArgs struct {
}

type Filters struct {
	URL []string
}

type SingleTrafficReply struct {
	Traffic  uint64
	UsedTime int64
}

type TrafficReply map[string]SingleTrafficReply
