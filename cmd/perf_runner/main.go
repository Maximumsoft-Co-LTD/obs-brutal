package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"obs-brutal/internal/core"
	"obs-brutal/internal/core/domain"
)

// devNullSink discards all writes to remove IO bottlenecks from perf tests
type devNullSink struct{}

func (s *devNullSink) Write(entry *domain.LogEntry) error     { return nil }
func (s *devNullSink) Close() error                           { return nil }
func (s *devNullSink) Name() string                           { return "devnull" }
func (s *devNullSink) Health() error                          { return nil }
func (s *devNullSink) Configure(map[string]interface{}) error { return nil }

func main() {
	var (
		totalOps   = flag.Int("total", 300000, "total log operations per scenario")
		workersIn  = flag.String("workers", "1,cpu", "comma list of worker counts: e.g. '1,cpu,2cpu'")
		modesIn    = flag.String("modes", "unified,async,strategy", "logger modes to run")
		structured = flag.Bool("structured", false, "include structured fields in logs")
		sinkType   = flag.String("sink", "devnull", "sink type: devnull|stdout|buffer|file")
		filePath   = flag.String("file", "logs/perf.log", "file path for sink=file")
		bufSize    = flag.Int("buffer_size", 1000, "buffer size for sink=buffer")
		bufTimeout = flag.Duration("buffer_timeout", 100*time.Millisecond, "flush timeout for sink=buffer")
		summary    = flag.Bool("summary", false, "print concise summary to stderr only")
	)
	flag.Parse()

	// Parse workers
	var workersList []int
	parts := strings.Split(*workersIn, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		switch p {
		case "cpu":
			workersList = append(workersList, runtime.NumCPU())
		case "2cpu":
			workersList = append(workersList, runtime.NumCPU()*2)
		default:
			var w int
			_, err := fmt.Sscanf(p, "%d", &w)
			if err == nil && w > 0 {
				workersList = append(workersList, w)
			}
		}
	}
	if len(workersList) == 0 {
		workersList = []int{1, runtime.NumCPU()}
	}

	// Parse modes
	modeSet := map[string]bool{}
	for _, m := range strings.Split(*modesIn, ",") {
		m = strings.TrimSpace(strings.ToLower(m))
		if m != "" {
			modeSet[m] = true
		}
	}
	if len(modeSet) == 0 {
		modeSet = map[string]bool{"unified": true, "async": true, "strategy": true}
	}

	// Setup sink
	var selectedSink core.Sink
	switch strings.ToLower(*sinkType) {
	case "stdout":
		selectedSink = core.NewFastStdoutSink()
	case "buffer":
		selectedSink = core.NewBufferedSinkWith(*bufSize, *bufTimeout)
	case "file":
		fs := core.NewOptimalFileSink()
		_ = fs.Configure(map[string]interface{}{
			"filename": *filePath,
		})
		selectedSink = fs
	case "devnull":
		fallthrough
	default:
		selectedSink = &devNullSink{}
	}

	if !*summary {
		fmt.Printf("\n== OBS-Brutal Perf Runner ==\n")
		fmt.Printf("GoMaxProcs: %d, CPU: %d, Structured: %v\n", runtime.GOMAXPROCS(0), runtime.NumCPU(), *structured)
		fmt.Printf("Total ops/scenario: %d\n", *totalOps)
	}

	printResult := func(mode string, d time.Duration, lps float64, us float64) {
		line := fmt.Sprintf("%s  | %s | logs/sec=%0.f | per_log_us=%.2f\n", mode, d, lps, us)
		if *summary {
			_, _ = os.Stderr.WriteString(line)
		} else {
			fmt.Print(line)
		}
	}

	run := func(name string, logger core.LogBrt, total, workers int, structured bool) (logsPerSec float64, perLogUs float64, duration time.Duration) {
		perWorker := total / workers
		start := time.Now()
		var wg sync.WaitGroup
		wg.Add(workers)
		for i := 0; i < workers; i++ {
			go func(worker int) {
				defer wg.Done()
				if structured {
					for n := 0; n < perWorker; n++ {
						logger.F("worker", worker).
							F("iteration", n).
							F("success", true).
							F("module", "perf").
							Info("perf structured")
					}
				} else {
					for n := 0; n < perWorker; n++ {
						logger.Info("perf basic")
					}
				}
			}(i)
		}
		wg.Wait()

		// Best-effort flush for async-based loggers
		switch l := logger.(type) {
		case *core.AsyncLogBrt:
			l.Stop()
		case *core.StrategyLogBrt:
			l.Stop()
		}

		duration = time.Since(start)
		if duration <= 0 {
			duration = time.Nanosecond
		}
		logsPerSec = float64(total) / duration.Seconds()
		perLogUs = float64(duration.Microseconds()) / float64(total)
		return
	}

	for _, workers := range workersList {
		if !*summary {
			fmt.Printf("\n--- Workers: %d ---\n", workers)
		}

		if modeSet["unified"] {
			l := core.NewUnifiedLogBrt(core.INFO, selectedSink)
			l2 := l // as core.LogBrt
			lps, us, d := run("unified-basic", l2, *totalOps, workers, *structured)
			printResult("unified  ", d, lps, us)
		}

		if modeSet["async"] {
			l := core.NewAsyncLogBrt(core.INFO, selectedSink)
			var logger core.LogBrt = l
			lps, us, d := run("async", logger, *totalOps, workers, *structured)
			printResult("async    ", d, lps, us)
		}

		if modeSet["strategy"] {
			l := core.NewStrategyLogBrt(core.INFO, selectedSink)
			var logger core.LogBrt = l
			lps, us, d := run("strategy", logger, *totalOps, workers, *structured)
			printResult("strategy ", d, lps, us)
		}
	}
}
