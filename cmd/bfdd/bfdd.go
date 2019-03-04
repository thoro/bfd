package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	// "runtime/pprof"

	"github.com/Thoro/bfd/internal/app"
	"github.com/golang/glog"
	"github.com/spf13/pflag"
)

var (
	version   string
	commit    string
	buildDate string
)

func main() {
	/*
		PPROF
	    f, err := os.Create("bfdd.prof")
	    if err != nil {
	        glog.Fatal(err)
	    }
	    pprof.StartCPUProfile(f)
	    defer pprof.StopCPUProfile()
	*/

	options := NewOptions()
	options.AddFlags(pflag.CommandLine)
	pflag.Parse()

	flag.CommandLine.Parse([]string{})
	flag.Set("logtostderr", "true")

	if options.HelpRequested {
		PrintVersion()
		pflag.Usage()
		os.Exit(0)
	}

	app := app.NewBfdApp()
	app.Start()

	if err := app.LoadConfig(options.Config); err != nil {
		glog.Errorf("%s", err.Error())
	}

	exit_ch := make(chan os.Signal)

	signal.Notify(exit_ch, syscall.SIGTERM)
	signal.Notify(exit_ch, os.Interrupt)

	glog.Info("Listening for shutdown!")

	for {
		select {
		case <-exit_ch:
			app.Shutdown()
			return
		}
	}
}

func PrintVersion() {
	output := fmt.Sprintf("Running %v version %s (%s), built on %s, %s\n", os.Args[0], version, commit, buildDate, runtime.Version())

	fmt.Fprintf(os.Stderr, output)
}
