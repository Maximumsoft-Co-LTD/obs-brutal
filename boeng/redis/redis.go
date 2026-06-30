// Package boengredis instruments github.com/redis/go-redis/v9 with
// boeng's Operation Runtime. Each Redis command (and pipeline) becomes
// a boeng operation; duration and errors are recorded automatically.
//
// Apply once at client construction:
//
//	client := redis.NewClient(&redis.Options{Addr: addr})
//	client.AddHook(boengredis.Hook())
//
// Business code never imports OpenTelemetry; the hook handles the
// trace/log/metric correlation.
package boengredis

import (
	"context"
	"net"
	"strings"

	"github.com/redis/go-redis/v9"

	"obs-brutal/boeng"
)

// Hook returns a redis.Hook implementation that wraps every Redis
// command and pipeline in a boeng operation.
func Hook() redis.Hook {
	return &hook{}
}

type hook struct{}

func (h *hook) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		var conn net.Conn
		err := boeng.Run(ctx, "redis.dial", dialFields{Network: network, Addr: addr},
			func(opCtx context.Context) error {
				c, err := next(opCtx, network, addr)
				if err == nil {
					conn = c
				}
				return err
			},
		)
		return conn, err
	}
}

func (h *hook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		return boeng.Run(ctx, "redis."+cmd.Name(), commandFields{Command: cmd.Name(), Args: argsAsString(cmd)},
			func(opCtx context.Context) error {
				return next(opCtx, cmd)
			},
		)
	}
}

func (h *hook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		return boeng.Run(ctx, "redis.pipeline", pipelineFields{N: len(cmds)},
			func(opCtx context.Context) error {
				return next(opCtx, cmds)
			},
		)
	}
}

type dialFields struct {
	Network string
	Addr    string
}

func (d dialFields) LogFields() map[string]any {
	return map[string]any{
		"db.system":      "redis",
		"net.peer.name":  d.Addr,
		"net.transport":  d.Network,
	}
}

type commandFields struct {
	Command string
	Args    string
}

func (c commandFields) LogFields() map[string]any {
	return map[string]any{
		"db.system":     "redis",
		"db.command":    c.Command,
		"db.statement":  c.Args,
	}
}

type pipelineFields struct {
	N int
}

func (p pipelineFields) LogFields() map[string]any {
	return map[string]any{
		"db.system":            "redis",
		"db.redis.pipeline.n":  p.N,
	}
}

// argsAsString flattens a command's args into a single space-separated
// string for the log/span field. The first element is the command name;
// we drop it because boeng's op name already captures it. The value is
// truncated to keep log lines bounded.
func argsAsString(cmd redis.Cmder) string {
	args := cmd.Args()
	if len(args) <= 1 {
		return ""
	}
	var b strings.Builder
	for i, a := range args[1:] {
		if i > 0 {
			b.WriteByte(' ')
		}
		switch v := a.(type) {
		case string:
			b.WriteString(v)
		default:
			b.WriteString(redisArgString(v))
		}
		if b.Len() > 256 {
			b.WriteString("…")
			break
		}
	}
	return b.String()
}

func redisArgString(v any) string {
	// Conservative — go-redis args are typically strings/numbers; we
	// rely on fmt.Sprint only for unusual cases to keep allocs low.
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
