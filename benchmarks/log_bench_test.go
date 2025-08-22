package benchmarks

import (
	"runtime"
	"sync"
	"testing"

	"obs-brutal/logtrc"
)

func BenchmarkLogging(b *testing.B) {
	// Silence info output for fair measurement
	log := logtrc.New(logtrc.LogLevel(logtrc.WARN))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		log.F("i", i).Info("bench")
	}
}

func BenchmarkLoggingParallel(b *testing.B) {
	log := logtrc.New(logtrc.LogLevel(logtrc.WARN))
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			log.F("k", 1).Info("benchp")
		}
	})
}

func BenchmarkLoggingGoroutines(b *testing.B) {
	log := logtrc.New(logtrc.LogLevel(logtrc.WARN))
	workers := runtime.NumCPU()
	perWorker := b.N / workers
	var wg sync.WaitGroup
	wg.Add(workers)
	b.ReportAllocs()
	b.ResetTimer()
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				log.F("w", w).Info("benchg")
			}
		}()
	}
	wg.Wait()
}
