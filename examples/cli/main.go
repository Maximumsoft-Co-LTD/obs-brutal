// CLI dogfood: a short-lived command-line tool. The boeng pattern
// here is the same as a request handler — wrap the work in one
// Operation, return error from the closure, exit non-zero on failure.
//
// What boeng buys you here:
//   - Even one-shot processes get structured stdout JSON suitable for
//     log aggregators. No "did you set up zerolog?" question.
//   - Panics inside the command are captured, recorded, and re-raised
//     so the user still sees the stack trace, but the failed
//     completion log lands FIRST.
//
// Run:
//
//	go run ./examples/cli greet     "Alice"
//	go run ./examples/cli factorial "5"
//	go run ./examples/cli explode               # panics on purpose
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"

	"obs-brutal/boeng"
)

func main() {
	defer boeng.Init(boeng.Config{Service: "cli_demo", Env: "dev"}).Close()

	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: cli <greet|factorial|explode> [arg]")
		os.Exit(2)
	}

	command := os.Args[1]
	arg := ""
	if len(os.Args) > 2 {
		arg = os.Args[2]
	}

	exit, err := boeng.RunR(context.Background(), "cli."+command,
		map[string]any{"arg": arg},
		func(ctx context.Context) (int, error) {
			return dispatch(ctx, command, arg)
		},
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
	}
	os.Exit(exit)
}

func dispatch(ctx context.Context, command, arg string) (int, error) {
	switch command {
	case "greet":
		if arg == "" {
			return 2, errors.New("greet requires a name")
		}
		boeng.L(ctx).F("name", arg).Info("greeting")
		fmt.Println("hello,", arg)
		return 0, nil
	case "factorial":
		n, err := strconv.Atoi(arg)
		if err != nil {
			return 2, fmt.Errorf("not an integer: %q", arg)
		}
		result, err := boeng.RunR(ctx, "factorial.compute", map[string]any{"n": n},
			func(ctx context.Context) (int, error) {
				return factorial(n)
			})
		if err != nil {
			return 1, err
		}
		fmt.Printf("%d! = %d\n", n, result)
		return 0, nil
	case "explode":
		panic("intentional explosion for demo")
	default:
		return 2, fmt.Errorf("unknown command %q", command)
	}
}

func factorial(n int) (int, error) {
	if n < 0 {
		return 0, errors.New("negative not allowed")
	}
	r := 1
	for i := 2; i <= n; i++ {
		r *= i
	}
	return r, nil
}
