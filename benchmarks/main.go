package main

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/td0m/zima"
)

var totalInserts int = 0

func cleanup(ctx context.Context, conn *pgxpool.Pool) {
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
	s := zima.NewServer(conn())

	start := time.Now()
	setupTesting(s, "root", []int{10, 6, 4, 3, 4})
	fmt.Println(time.Since(start)/time.Duration(totalInserts), totalInserts)
}

func setupTesting(s *zima.Server, a string, ns []int) {
	if len(ns) == 0 {
		return
	}
	ctx := context.Background()

	for i := 0; i < ns[0]; i++ {
		b := randString(10)

		totalInserts++

		check(s.Add(ctx, zima.Tuple{
			Parent: set("collection", a, "member"),
			Child:  set("collection", b, "member"),
		}))

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
