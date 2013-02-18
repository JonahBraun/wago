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
	"regexp"
)

var (
	verbose      = flag.Bool("v", false, "Verbose")
	verboseQuiet = flag.Bool("q", false, "Quiet, only warnings and errors")

	fiddle        = flag.Bool("fiddle", false, "CLI fiddle mode, starts a web server and opens url to watchDir/index.html")
	watchDir      = flag.String("dir", "", "Directory to watch, defaults to current")
	buildCmd      = flag.String("cmd", "", "Bash command to run on change, Wabo will wait for this command to finish")
	daemonCmd     = flag.String("daemon", "", "Bash command that starts a daemon, Wago will halt if the daemon exits before the trigger or timer")
	daemonTrigger = flag.String("trigger", "", "A string the daemon will output that indicates it has started successfuly, Wago will continue on this trigger")
	daemonTimer   = flag.Int("timer", 0, "Miliseconds to wait after starting daemon before continuing")
	webServer     = flag.String("web", "", "Start a web server at this address, e.g. :8420")
	postCmd       = flag.String("pcmd", "", "Bash command to run after the daemon has successfully started")
	url           = flag.String("url", "", "URL to open")
	watchRegex    = flag.String("watch", `/\w[\w\.]*": (CREATE|MODIFY)`, "Regex to match watch event, use -v to see all events")

	daemon = &Daemon{}
	cmd    = &Cmd{}
)

func event() {
	if cmd.Cmd != nil && cmd.ProcessState == nil {
		cmd.Kill()
	}

	if daemon.Cmd != nil && daemon.ProcessState == nil {
		daemon.Kill()
	}

	// run build command
	if len(*buildCmd) > 0 {
		cmd = NewCmd(*buildCmd)
		ok := cmd.Run()
		if !ok {
			return
		}
	}

	// start the daemon
	if len(*daemonCmd) > 0 {
		daemon = NewDaemon(*daemonCmd)

		if len(*daemonTrigger) > 0 {
			if !daemon.RunTrigger(*daemonTrigger) {
				return
			}
		} else {
			if !daemon.RunTimer(*daemonTimer) {
				return
			}
		}
	}

	// run post command
	if len(*postCmd) > 0 {
		cmd = NewCmd(*postCmd)
		ok := cmd.Run()
		if !ok {
			return
		}
	}

	// open the url
	if *url != "" {
		openUrl()
	}
}

func main() {
	flag.Usage = func() {
		fmt.Println("WaGo (Watch, Go) build tool. Usage:")
		flag.PrintDefaults()
	}

	if len(os.Args) < 2 {
		flag.Usage()
		Fatal("You must specify an action")
	}

	flag.Parse()

	if *fiddle {
		if *webServer == "" {
			*webServer = ":9933"
		}
		if *url == "" {
			*url = "http://localhost" + *webServer + "/index.html"
		}
	}

	if *watchDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			panic(err)
		}
		watchDir = &cwd
	}
	Talk("Watching dir:", *watchDir)

	r, err := regexp.Compile(*watchRegex)
	if err != nil {
		Fatal("Watch regex compile error:", err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic(err)
	}
	defer watcher.Close()

	err = watcher.Watch(*watchDir)
	if err != nil {
		panic(err)
	}

	if *webServer != "" {
		go func() {
			Note("Starting web server on port", *webServer)
			err := http.ListenAndServe(*webServer, http.FileServer(http.Dir(*watchDir)))
			if err != nil {
				Fatal("Error starting web server:", err)
			}
		}()
	}

	// trigger event on start
	go event()

	for {
		select {
		case ev := <-watcher.Event:
			if r.MatchString(ev.String()) {
				Note("Matched event:", ev.String())
				go event()
			} else {
				Talk("Ignored event:", ev.String())
			}

		case err = <-watcher.Error:
			panic(err)
		}
	}
}
