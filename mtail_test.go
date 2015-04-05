// Copyright 2011 Google Inc. All Rights Reserved.
// This file is available under the Apache license.

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/golang/glog"
	"github.com/google/mtail/vm"
	"github.com/google/mtail/watcher"
)

const testProgram = "/$/ { }"

func startMtail(t *testing.T, logPathnames []string, progPathname string) *mtail {
	m := newMtail()
	w, err := watcher.NewLogWatcher()
	if err != nil {
		t.Errorf("Couldn't create watcher: %s", err)
	}
	p := vm.NewLoader(w, &m.store)
	// start server
	prog, errors := vm.Compile("test", strings.NewReader(testProgram), &m.store)
	if len(errors) > 0 {
		t.Errorf("Couldn't compile program: %s", errors)
	}
	p.E.AddVM("test", prog)
	if progPathname != "" {
		p.LoadProgs(progPathname)
	}
	vm.LineCount.Set(0)
	go p.E.Run(m.lines)
	m.StartTailing(logPathnames)
	return m
}

func TestHandleLogUpdates(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}
	// make temp dir
	workdir, err := ioutil.TempDir("", "mtail_test")
	if err != nil {
		t.Errorf("could not create temporary directory: %s", err)
	}
	defer func() {
		err := os.RemoveAll(workdir)
		if err != nil {
			t.Errorf("Could not remove temp dir: %s", err)
		}
	}()
	// touch log file
	logFilepath := path.Join(workdir, "log")
	logFile, err := os.Create(logFilepath)
	if err != nil {
		t.Errorf("could not touch log file: %s", err)
	}
	defer logFile.Close()
	pathnames := []string{logFilepath}
	m := startMtail(t, pathnames, "")
	defer m.Close()
	inputLines := []string{"hi", "hi2", "hi3"}
	for i, x := range inputLines {
		// write to log file
		logFile.WriteString(x + "\n")
		// TODO(jaq): remove slow sleep
		time.Sleep(100 * time.Millisecond)
		// check log line count increase
		expected := fmt.Sprintf("%d", i+1)
		if vm.LineCount.String() != expected {
			t.Errorf("Line count not increased\n\texpected: %s\n\treceived: %s", expected, vm.LineCount.String())
		}
	}
}

func TestHandleLogRotation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}
	// make temp dir
	workdir, err := ioutil.TempDir("", "mtail_test")
	if err != nil {
		t.Errorf("could not create temporary directory: %s", err)
	}
	defer func() {
		err := os.RemoveAll(workdir)
		if err != nil {
			t.Errorf("Could not remove temp dir: %s", err)
		}
	}()
	logFilepath := path.Join(workdir, "log")
	// touch log file
	logFile, err := os.Create(logFilepath)
	if err != nil {
		t.Errorf("could not touch log file: %s", err)
	}
	defer logFile.Close()
	// Create a logger
	stop := make(chan bool, 1)
	hup := make(chan bool, 1)
	pathnames := []string{logFilepath}
	m := startMtail(t, pathnames, "")
	defer m.Close()

	go func() {
		logFile := logFile
		var err error
		i := 0
		running := true
		for running {
			select {
			case <-hup:
				// touch log file
				logFile, err = os.Create(logFilepath)
				if err != nil {
					t.Errorf("could not touch log file: %s", err)
				}
				defer logFile.Close()
			default:
				logFile.WriteString(fmt.Sprintf("%d\n", i))
				time.Sleep(100 * time.Millisecond)
				i++
				if i >= 10 {
					running = false
				}
			}
		}
		stop <- true
	}()
	go func() {
		for {
			select {
			case <-time.After(5 * 100 * time.Millisecond):
				err = os.Rename(logFilepath, logFilepath+".1")
				if err != nil {
					t.Errorf("could not rename log file: %s", err)
				}
				hup <- true
				return
			}
		}
	}()
	<-stop
	expected := "10"
	if vm.LineCount.String() != expected {
		t.Errorf("Line count not increased\n\texpected: %s\n\treceived: %s", expected, vm.LineCount.String())
	}
}

func TestHandleNewLogAfterStart(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}
	// make temp dir
	workdir, err := ioutil.TempDir("", "mtail_test")
	if err != nil {
		t.Errorf("could not create temporary directory: %s", err)
	}
	defer func() {
		err := os.RemoveAll(workdir)
		if err != nil {
			t.Errorf("Could not remove temp dir: %s", err)
		}
	}()
	// Start up mtail
	logFilepath := path.Join(workdir, "log")
	pathnames := []string{logFilepath}
	m := startMtail(t, pathnames, "")
	defer m.Close()

	// touch log file
	logFile, err := os.Create(logFilepath)
	if err != nil {
		t.Errorf("could not touch log file: %s", err)
	}
	defer logFile.Close()
	inputLines := []string{"hi", "hi2", "hi3"}
	for _, x := range inputLines {
		// write to log file
		logFile.WriteString(x + "\n")
		logFile.Sync()
	}
	// TODO(jaq): remove slow sleep
	time.Sleep(100 * time.Millisecond)
	// check log line count increase
	expected := fmt.Sprintf("%d", len(inputLines))
	if vm.LineCount.String() != expected {
		t.Errorf("Line count not increased\n\texpected: %s\n\treceived: %s", expected, vm.LineCount.String())
	}
}

func TestHandleNewLogIgnored(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}
	// make temp dir
	workdir, err := ioutil.TempDir("", "mtail_test")
	if err != nil {
		t.Errorf("could not create temporary directory: %s", err)
	}
	defer func() {
		err := os.RemoveAll(workdir)
		if err != nil {
			t.Errorf("Could not remove temp dir: %s", err)
		}
	}()
	// Start mtail
	logFilepath := path.Join(workdir, "log")
	pathnames := []string{logFilepath}
	m := startMtail(t, pathnames, "")
	defer m.Close()

	// touch log file
	newLogFilepath := path.Join(workdir, "log1")

	logFile, err := os.Create(newLogFilepath)
	if err != nil {
		t.Errorf("could not touch log file: %s", err)
	}
	defer logFile.Close()
	expected := "0"
	if vm.LineCount.String() != expected {
		t.Errorf("Line count not increased\n\texpected: %s\n\treceived: %s", expected, vm.LineCount.String())
	}
}

func makeTempDir(t *testing.T) (workdir string) {
	var err error
	if workdir, err = ioutil.TempDir("", "mtail_test"); err != nil {
		t.Errorf("ioutil.TempDir failed: %s", err)
	}
	return
}

func removeTempDir(t *testing.T, workdir string) {
	if err := os.RemoveAll(workdir); err != nil {
		t.Errorf("os.RemoveAll failed: %s", err)
	}
}

// TODO(jaq): The sleeps in here are racy.  What can we use to sync through inotify?
func TestHandleNewProgram(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	workdir := makeTempDir(t)
	defer removeTempDir(t, workdir)

	m := startMtail(t, []string{}, workdir)
	defer m.Close()

	expectedProgLoads := "{}"
	if vm.ProgLoads.String() != expectedProgLoads {
		t.Errorf("Prog loads not same\n\texpected: %s\n\treceived: %s", expectedProgLoads, vm.ProgLoads.String())
	}

	progPath := path.Join(workdir, "prog.mtail")
	progFile, err := os.Create(progPath)
	if err != nil {
		t.Errorf("prog create failed: %s", err)
	}
	progFile.WriteString("/$/ {}\n")
	progFile.Close()
	glog.Infof("hi")

	// Wait for inotify
	time.Sleep(100 * time.Millisecond)
	expectedProgLoads = `{"prog.mtail": 1}`
	if vm.ProgLoads.String() != expectedProgLoads {
		t.Errorf("Prog loads not same\n\texpected: %s\n\treceived: %s", expectedProgLoads, vm.ProgLoads.String())
	}

	badProgPath := path.Join(workdir, "prog.mtail.dpkg-dist")
	badProgFile, err := os.Create(badProgPath)
	if err != nil {
		t.Errorf("prog create failed: %s", err)
	}
	badProgFile.WriteString("/$/ {}\n")
	badProgFile.Close()

	time.Sleep(100 * time.Millisecond)
	expectedProgLoads = `{"prog.mtail": 1}`
	if vm.ProgLoads.String() != expectedProgLoads {
		t.Errorf("Prog loads not same\n\texpected: %s\n\treceived: %s", expectedProgLoads, vm.ProgLoads.String())
	}
	expectedProgErrs := `{}`
	if vm.ProgLoadErrors.String() != expectedProgErrs {
		t.Errorf("Prog errors not same\n\texpected: %s\n\treceived: %s", expectedProgErrs, vm.ProgLoadErrors.String())
	}

	os.Rename(badProgPath, progPath)
	time.Sleep(100 * time.Millisecond)
	expectedProgLoads = `{"prog.mtail": 1}`
	if vm.ProgLoads.String() != expectedProgLoads {
		t.Errorf("Prog loads not same\n\texpected: %s\n\treceived: %s", expectedProgLoads, vm.ProgLoads.String())
	}
	expectedProgErrs = `{}`
	if vm.ProgLoadErrors.String() != expectedProgErrs {
		t.Errorf("Prog errors not same\n\texpected: %s\n\treceived: %s", expectedProgErrs, vm.ProgLoadErrors.String())
	}

	brokenProgPath := path.Join(workdir, "broken.mtail")
	brokenProgFile, err := os.Create(brokenProgPath)
	if err != nil {
		t.Errorf("prog create failed: %s", err)
	}
	brokenProgFile.WriteString("?\n")
	brokenProgFile.Close()

	time.Sleep(100 * time.Millisecond)

	expectedProgLoads = `{"prog.mtail": 1}`
	if vm.ProgLoads.String() != expectedProgLoads {
		t.Errorf("Prog loads not same\n\texpected: %s\n\treceived: %s", expectedProgLoads, vm.ProgLoads.String())
	}
	expectedProgErrs = `{"broken.mtail": 1}`
	if vm.ProgLoadErrors.String() != expectedProgErrs {
		t.Errorf("Prog errors not same\n\texpected: %s\n\treceived: %s", expectedProgErrs, vm.ProgLoadErrors.String())
	}

}
