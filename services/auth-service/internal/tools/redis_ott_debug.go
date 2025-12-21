package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

func main() {
	var (
		addr    = flag.String("addr", "127.0.0.1:6379", "redis address host:port")
		pass    = flag.String("pass", "", "redis password")
		db      = flag.Int("db", 0, "redis db")
		pattern = flag.String("pattern", "ott:*", "scan pattern")
		doDel   = flag.Bool("del", false, "delete matched keys")
		limit   = flag.Int64("count", 200, "SCAN COUNT hint")
		timeout = flag.Duration("timeout", 2*time.Second, "per-command timeout")
	)
	flag.Parse()

	rdb := goredis.NewClient(&goredis.Options{
		Addr:     *addr,
		Password: *pass,
		DB:       *db,
	})
	defer rdb.Close()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		fmt.Fprintf(os.Stderr, "redis ping failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Connected: addr=%s db=%d pattern=%q\n", *addr, *db, *pattern)

	var cursor uint64
	total := 0

	for {
		ctxScan, cancelScan := context.WithTimeout(context.Background(), *timeout)
		keys, next, err := rdb.Scan(ctxScan, cursor, *pattern, *limit).Result()
		cancelScan()
		if err != nil {
			fmt.Fprintf(os.Stderr, "SCAN error: %v\n", err)
			os.Exit(1)
		}

		for _, k := range keys {
			total++
			ctxCmd, cancelCmd := context.WithTimeout(context.Background(), *timeout)
			val, _ := rdb.Get(ctxCmd, k).Result() // ignore Nil here; we'll show empty
			ttl, _ := rdb.TTL(ctxCmd, k).Result()
			cancelCmd()

			fmt.Printf("%d) %s\n   ttl=%s\n   val=%q\n", total, k, ttl, val)

			if *doDel {
				ctxDel, cancelDel := context.WithTimeout(context.Background(), *timeout)
				n, err := rdb.Del(ctxDel, k).Result()
				cancelDel()
				if err != nil {
					fmt.Printf("   DEL error: %v\n", err)
				} else {
					fmt.Printf("   DEL ok: %d\n", n)
				}
			}
		}

		cursor = next
		if cursor == 0 {
			break
		}
	}

	if total == 0 {
		fmt.Println("No keys matched.")
	}
}
