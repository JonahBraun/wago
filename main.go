/*
	Watches the current directory for changes, and runs the command you supply as arguments
	in response to changes.

	This is my first go project, suggestions welcome!
*/

package main

import (
	"flag"
	"fmt"
	"github.com/howeyc/fsnotify"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
)

var (
	verbose = flag.Bool("v", false, "Verbose")
	quiet   = flag.Bool("q", false, "Quiet, only warnings and errors")

	buildCmd      = flag.String("cmd", "", "Bash command to run on change, Wabo will wait for this command to finish")
	daemonCmd     = flag.String("daemon", "", "Bash command that starts a daemon, Wago will halt if the daemon exits before the trigger or timer")
	daemonTimer   = flag.Int("timer", 0, "Miliseconds to wait after starting daemon before continuing")
	daemonTrigger = flag.String("trigger", "", "A string the daemon will output that indicates it has started successfuly, Wago will continue on this trigger")
	fiddle        = flag.Bool("fiddle", false, "CLI fiddle mode, starts a web server and opens url to targetDir/index.html")
	leader        = flag.String("leader", "", "Leader character for wago output (to differentiate from command output), defaults to emoji")
	postCmd       = flag.String("pcmd", "", "Bash command to run after the daemon has successfully started")
	recursive     = flag.Bool("recursive", true, "Watch directory tree recursively")
	targetDir     = flag.String("dir", "", "Directory to watch, defaults to current")
	url           = flag.String("url", "", "URL to open")
	watchRegex    = flag.String("watch", `/\w[\w\.]*": (CREATE|MODIFY)`, "Regex to match watch event, use -v to see all events")
	webServer     = flag.String("web", "", "Start a web server at this address, e.g. :8420")

	machine Machine
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	flag.Usage = func() {
		fmt.Println("WaGo (Watch, Go) build tool. Usage:")
		flag.PrintDefaults()
	}

	// TODO: this should check for actions
	if len(os.Args) < 2 {
		flag.Usage()
		Fatal("You must specify an action")
	}

	flag.Parse()

	if (len(*daemonTrigger) > 0) && (*daemonTimer > 0) {
		Fatal("Both daemon trigger and timer specified, use only one")
	}

	if len(*daemonTrigger) > 0 || *daemonTimer > 0 && len(*daemonCmd) == 0 {
		Fatal("Specify a daemon command to use the trigger or timer")
	}

	if *fiddle {
		if *webServer == "" {
			*webServer = ":9933"
		}
		if *url == "" {
			*url = "http://localhost" + *webServer + "/"
		}
	}

	if *targetDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			panic(err)
		}
		targetDir = &cwd
	}
	Talk("Target dir:", *targetDir)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic(err)
	}
	defer watcher.Close()

	watchDir := func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			return nil
		}

		Talk("Watching dir:", path)

		if err != nil {
			Err("Skipping dir:", path, err)
			return filepath.SkipDir
		}

		err = watcher.Watch(path)
		if err != nil {
			panic(err)
		}

		return nil
	}

	if *recursive == true {
		err = filepath.Walk(*targetDir, watchDir)
		if err != nil {
			panic(err)
		}
	} else {
		err = watcher.Watch(*targetDir)
		if err != nil {
			panic(err)
		}
	}

	if *webServer != "" {
		go func() {
			Note("Starting web server on port", *webServer)
			err := http.ListenAndServe(*webServer, http.FileServer(http.Dir(*targetDir)))
			if err != nil {
				Fatal("Error starting web server:", err)
			}
		}()
	}

	machine = NewMachine(watcher)
	go machine.RunHandler()

	<-make(chan int)
}
