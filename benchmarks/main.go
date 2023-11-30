package main

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/td0m/zima"
)

var totalInserts = 0
var depths = []int{100, 3, 2}
var clean = true

func init() {
	count := 1
	for _, i := range depths {
		count *= i
	}
	fmt.Println(count)
}

func cleanup(ctx context.Context, conn *pgxpool.Pool) {
	if !clean {
		return
	}
	query := `
		delete from tuples;
		delete from caches;
		delete from changes;
	`
	if _, err := conn.Exec(ctx, query); err != nil {
		panic(err)
	}
}

func conn() *pgxpool.Pool {
	ctx := context.Background()
	conn, err := pgxpool.New(ctx, "")
	if err != nil {
		panic(err)
	}
	cleanup(ctx, conn)
	return conn
}

func main() {
	c := conn()
	s := zima.NewServer(c)

	go func() {
		for {
			if err := s.ProcessOne(context.Background()); err != nil {
				fmt.Printf("failed to process change: %s\n", err)
				break
			}
		}
	}()

	start := time.Now()
	setupTesting(s, randString(100), depths)

	for {
		count := 0
		ctx := context.Background()
		c.QueryRow(ctx, `select count(*) from changes where not processed`).Scan(&count)
		if count == 0 {
			break
		}
		time.Sleep(time.Millisecond * 10)
	}
	fmt.Println(time.Since(start)/time.Duration(totalInserts), totalInserts)
}

func setupTesting(s *zima.Server, a string, ns []int) {
	if len(ns) == 0 {
		return
	}
	ctx := context.Background()


	for i := 0; i < ns[0]; i++ {
		b := randString(1000)

		totalInserts++

		t := zima.Tuple{
			Parent: set("collection", a, "member"),
			Child:  set("collection", b, "member"),
		}

		check(s.Add(ctx, t))

		setupTesting(s, b, ns[1:])
	}
}

func set(typ, id, rel string) zima.Set {
	s, _ := zima.NewSet(typ, id, rel)
	return s
}
func check(err error) {
	if err != nil {
		panic(err)
	}
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func randString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}
