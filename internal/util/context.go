package util

import "context"

// type-safe keys
type ctxKey string

const (
    keyTraceID   ctxKey = "trace_id"
    keyUserID    ctxKey = "user_id"
    keyRequestID ctxKey = "request_id"
)

// With helpers (type-safe)
func WithTraceID(ctx context.Context, id string) context.Context   { return context.WithValue(ctx, keyTraceID, id) }
func WithUserID(ctx context.Context, id string) context.Context    { return context.WithValue(ctx, keyUserID, id) }
func WithRequestID(ctx context.Context, id string) context.Context { return context.WithValue(ctx, keyRequestID, id) }

// Get helpers (prefer type-safe, fallback to legacy string keys)
func GetTraceID(ctx context.Context) string   { return getString(ctx, keyTraceID, "trace_id") }
func GetUserID(ctx context.Context) string    { return getString(ctx, keyUserID, "user_id") }
func GetRequestID(ctx context.Context) string { return getString(ctx, keyRequestID, "request_id") }

func getString(ctx context.Context, typedKey ctxKey, legacy string) string {
    if ctx == nil { return "" }
    if v := ctx.Value(typedKey); v != nil {
        if s, ok := v.(string); ok { return s }
    }
    if v := ctx.Value(legacy); v != nil {
        if s, ok := v.(string); ok { return s }
    }
    return ""
}

// Deprecated legacy extractor (kept for compatibility)
func ExtractFromContext(ctx context.Context, key string) string {
    if ctx == nil { return "" }
    if val := ctx.Value(key); val != nil { if str, ok := val.(string); ok { return str } }
    return ""
}
