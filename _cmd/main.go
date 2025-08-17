package main

import (
	"context"
	"fmt"
	"time"

	inbound "obs-brutal/internal/adapter/inbound"
	outbound "obs-brutal/internal/adapter/outbound"
	pin "obs-brutal/internal/core/port/inbound"
	usecases "obs-brutal/internal/usecase"
	obsv "obs-brutal/obsvbrutal"

	"github.com/gin-gonic/gin"
)

// minimal in-memory registries for demo
type inMemFeatureReg struct{ m map[string]pin.Feature }

func (r *inMemFeatureReg) Register(name string, f pin.Feature) error {
	if r.m == nil {
		r.m = map[string]pin.Feature{}
	}
	r.m[name] = f
	return nil
}
func (r *inMemFeatureReg) Get(name string) (pin.Feature, error) {
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
func (r *inMemFeatureReg) Apply(l pin.Logger, names []string) pin.Logger {
	for _, n := range names {
		if f, ok := r.m[n]; ok {
			l = f.Apply(l)
		}
	}
	return l
}

type inMemErrReg struct{ m map[string]pin.ErrHandler }

func (r *inMemErrReg) Register(name string, h pin.ErrHandler) error {
	if r.m == nil {
		r.m = map[string]pin.ErrHandler{}
	}
	r.m[name] = h
	return nil
}
func (r *inMemErrReg) Get(name string) (pin.ErrHandler, error) {
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
func (r *inMemErrReg) Handle(l pin.Logger, err error, cat string, det map[string]interface{}) {
	if h, ok := r.m[cat]; ok {
		h.Handle(l, err, det)
	}
}

// simple service adapter to satisfy inbound.Service

type simpleService struct{ base pin.Logger }

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
func (s *simpleService) New(cfg pin.Config) (pin.Logger, error) { return s.base, nil }
func (s *simpleService) NewSimple(ctx context.Context, operationName string) (pin.Simple, error) {
	return nil, nil
}

func main() {
	// 1) Logger + sinks
	stdout := obsv.NewStdoutSink()
	logger, _ := obsv.NewLogger(obsv.WithLevel(obsv.InfoLevel), obsv.WithSinks(stdout))

	// 2) deps for usecase (kept for future extension)
	feat := &inMemFeatureReg{}
	errReg := &inMemErrReg{}
	_, _ = feat, errReg
	cfg, _ := outbound.NewRedisConfigProvider("127.0.0.1:6379", "", 0, "obsv:")
	metrics := outbound.NewSimpleMetricsProvider()
	_ = usecases.NewLoggingUseCase(logger, feat, errReg, cfg, metrics)

	// 3) HTTP adapter and Gin
	r := obsv.NewGinEngine()
	r.Use(inbound.GinMiddleware(logger))
	api := inbound.NewHTTPAdapter(&simpleService{base: logger}, feat, errReg)
	api.RegisterRoutes(r)

	// demo route
	r.GET("/ping", func(c *gin.Context) {
		if lg, ok := inbound.GetLoggerFromGinContext(c); ok {
			lg.F("path", "/ping").Info("pong")
		}
		c.JSON(200, gin.H{"ok": "pong", "ts": time.Now().Unix()})
	})

	_ = r.Run(":8080")
}
