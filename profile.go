package main

import (
	"log"
	"os"
	"runtime/pprof"
	"strconv"
	"time"
)

// startProfile writes a CPU profile to $SANITY_CPUPROFILE for
// $SANITY_CPUPROFILE_SECS seconds (default 30) when the variable is set. It is
// a diagnostic for UI latency and does nothing otherwise.
func startProfile() {
	path := os.Getenv("SANITY_CPUPROFILE")
	if path == "" {
		return
	}
	secs := 30
	if s, err := strconv.Atoi(os.Getenv("SANITY_CPUPROFILE_SECS")); err == nil && s > 0 {
		secs = s
	}
	f, err := os.Create(path)
	if err != nil {
		log.Println("cpu profile:", err)
		return
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		log.Println("cpu profile:", err)
		f.Close()
		return
	}
	log.Printf("cpu profile: recording %ds to %s", secs, path)
	time.AfterFunc(time.Duration(secs)*time.Second, func() {
		pprof.StopCPUProfile()
		f.Close()
		log.Println("cpu profile: written", path)
	})
}

func init() { startProfile() }
