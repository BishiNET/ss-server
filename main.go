package main

import (
	"os"
	"os/signal"
	"syscall"

	api "github.com/BishiNET/ss-server/rpcAPI"
)

func main() {
	r := api.New("127.0.0.1:50899", "127.0.0.1:6379")
	defer r.RedisClose()
	r.FastRestore()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
}
