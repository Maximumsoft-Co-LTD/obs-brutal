// Redis adapter demo. Requires a reachable Redis at REDIS_ADDR
// (defaults to localhost:6379). Run:
//
//   go run ./examples/redis
//
// Each command issued by the client becomes a boeng operation named
// "redis.<command>". Pipelines get "redis.pipeline".
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/redis/go-redis/v9"

	"obs-brutal/boeng"
	boengredis "obs-brutal/boeng/redis"
)

func main() {
	defer boeng.Init(boeng.Config{Service: "redis_demo", Env: "dev"}).Close()

	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}

	client := redis.NewClient(&redis.Options{Addr: addr})
	client.AddHook(boengredis.Hook())
	defer client.Close()

	ctx := context.Background()
	if err := client.Set(ctx, "boeng:demo", "hello", 0).Err(); err != nil {
		fmt.Println("set:", err)
		return
	}
	val, err := client.Get(ctx, "boeng:demo").Result()
	if err != nil {
		fmt.Println("get:", err)
		return
	}
	fmt.Println("got:", val)
}
