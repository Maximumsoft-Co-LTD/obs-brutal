package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	obsv "github.com/Maximumsoft-Co-LTD/obs-brutal/logbrutal"

	"github.com/gin-gonic/gin"
)

// minimal in-memory registries for demo
type inMemFeatureReg struct{ m map[string]obsv.Feature }

func (r *inMemFeatureReg) Register(name string, f obsv.Feature) error {
	if r.m == nil {
		r.m = map[string]obsv.Feature{}
	}
	r.m[name] = f
	return nil
}
func (r *inMemFeatureReg) Get(name string) (obsv.Feature, error) {
	if v, ok := r.m[name]; ok {
		return v, nil
	}
	return nil, context.Canceled
}
func (r *inMemFeatureReg) List() []string {
	a := make([]string, 0, len(r.m))
	for k := range r.m {
		a = append(a, k)
	}
	return a
}
func (r *inMemFeatureReg) Apply(l obsv.Logger, names []string) obsv.Logger {
	for _, n := range names {
		if f, ok := r.m[n]; ok {
			l = f.Apply(l)
		}
	}
	return l
}

type inMemErrReg struct{ m map[string]obsv.ErrHandler }

func (r *inMemErrReg) Register(name string, h obsv.ErrHandler) error {
	if r.m == nil {
		r.m = map[string]obsv.ErrHandler{}
	}
	r.m[name] = h
	return nil
}
func (r *inMemErrReg) Get(name string) (obsv.ErrHandler, error) {
	if v, ok := r.m[name]; ok {
		return v, nil
	}
	return nil, context.Canceled
}
func (r *inMemErrReg) List() []string {
	a := make([]string, 0, len(r.m))
	for k := range r.m {
		a = append(a, k)
	}
	return a
}
func (r *inMemErrReg) Handle(l obsv.Logger, err error, cat string, det map[string]interface{}) {
	if h, ok := r.m[cat]; ok {
		h.Handle(l, err, det)
	}
}

// simple service adapter
type simpleService struct{ base obsv.Logger }

func (s *simpleService) Log(ctx context.Context, level obsv.Level, msg string, fields map[string]interface{}) error {
	log := s.base.Ctx(ctx)
	if len(fields) > 0 {
		log = log.Fs(fields)
	}
	switch level {
	case obsv.DebugLevel:
		log.Debug(msg)
	case obsv.InfoLevel:
		log.Info(msg)
	case obsv.WarnLevel:
		log.Warn(msg)
	case obsv.ErrorLevel:
		log.Error(msg)
	case obsv.FatalLevel:
		log.Fatal(msg)
	}
	return nil
}
func (s *simpleService) LogErr(ctx context.Context, err error, category string, details map[string]interface{}) error {
	s.base.Ctx(ctx).Err(err).F("category", category).Fs(details).Error("error")
	return nil
}
func (s *simpleService) LogStruct(ctx context.Context, err obsv.StructuredError) error {
	return s.LogErr(ctx, fmt.Errorf("%s", err.Error()), err.Category, err.Details)
}
func (s *simpleService) New(cfg obsv.Config) (obsv.Logger, error) { return s.base, nil }
func (s *simpleService) NewSimple(ctx context.Context, operationName string) (obsv.Simple, error) {
	return nil, nil
}

func main() {
	// 1) Logger + sinks
	stdout := obsv.NewStdoutSink()
	logger, _ := obsv.NewLogger(obsv.WithLevel(obsv.InfoLevel), obsv.WithSinks(stdout))

	// 2) Create registries for features and error handlers
	// feat := &inMemFeatureReg{}
	// errReg := &inMemErrReg{}

	// 3) HTTP adapter and Gin
	r := obsv.NewGinEngine()
	r.Use(obsv.GinMiddleware(logger))

	// demo route
	r.GET("/ping", func(c *gin.Context) {
		lg := obsv.GetLogFrmGin(c, "PingHandler")
		defer lg.Close()

		var tracer = lg.F("path", "/ping").FlatPr("ping")
		defer tracer.End()
		tracer.Add(
			tracer.Str("path", "/ping"),
			tracer.Str("method", "GET"),
			tracer.Str("status", "200"),
			tracer.Str("response", "pong"),
		)
		lg.R(http.StatusOK, obsv.OptsResponse().Response(gin.H{"ok": "pong", "ts": time.Now().Unix()}))
	})

	fmt.Println("Server starting on :8080")
	_ = r.Run(":8080")
}
