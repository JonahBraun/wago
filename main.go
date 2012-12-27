// Watches the current directory for changes, and runs the command you supply as arguments
// in response to changes

package main

import (
	"flag"
	"fmt"
	"github.com/howeyc/fsnotify"
	"os"
	"os/exec"
	"time"
)

var (
	verbose       = flag.Bool("v", true, "Verbose")
	watchDir      = flag.String("dir", "", "Directory to watch, defaults to current")
	buildCmd      = flag.String("cmd", "", "Bash command to run on change. Wabo will wait for this command to finish.")
	daemonCmd     = flag.String("daemon", "", "Bash command that starts a daemon. Wago will halt if the daemon exits before the trigger or timer.")
	daemonTrigger = flag.String("trigger", "", "A string the daemon will output that indicates it has started successfuly. Wago will continue on this trigger.")
	daemonTimer   = flag.Int("timer", 0, "Miliseconds to wait after starting daemon before continuing.")
	webServer     = flag.String("web", "", "Start a web server at this address, e.g. :8080")
	url           = flag.String("url", "", "URL to open in Chrome")

	daemon = exec.Command("/bin/bash", "-c", *daemonCmd)
)


func help() {
	fmt.Println("WaGo (Wait, Go) build watcher")
}

func action() {
	if len(*daemonCmd) > 0 && daemon.Process != nil{
		talk("Killing daemon")
		if err := daemon.Process.Kill(); err != nil {
			Fatal("Failed to kill daemon: ", err)
		}
	}

	if len(*buildCmd) > 0 {
		talk("Running -cmd: ", *buildCmd)
		cmd := exec.Command("/bin/bash", "-c", *buildCmd)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		err := cmd.Run()

		if err != nil {
			Err("Error executing -cmd: ", err)
		}
	}

	if len(*daemonCmd) > 0 {
		talk("Running -daemon: ", *daemonCmd)
		daemon = exec.Command("/bin/bash", "-c", *daemonCmd)

		if len(*daemonTrigger) == 0 {
			daemon.Stdout = os.Stdout
			daemon.Stderr = os.Stderr
		}

		err := daemon.Start()

		if *daemonTimer > 0 {
			talk("Waiting -timer: ", *daemonTimer)

			go func() {
				time.Sleep(time.Duration(*daemonTimer) * time.Millisecond)
				openUrl()
			}()
		}

		if err != nil {
			Err("Error executing -daemon: ", err)
		}
	}

}

func main() {
	if len(os.Args) < 2 {
		help()
		Fatal("You must specify an action")
	}

	flag.Parse()

	if *watchDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			panic(err)
		}
		watchDir = &cwd
	}
	talk("Watching -dir: ", *watchDir)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic(err)
	}
	defer watcher.Close()

	err = watcher.Watch(*watchDir)
	if err != nil {
		panic(err)
	}

	// perform action on start
	action()

	for {
		select {
		case ev := <-watcher.Event:
			talk("event: ", ev.String())
			action()

		case err = <-watcher.Error:
			panic(err)
		}
	}

}
