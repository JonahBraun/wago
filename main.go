// Watches the current directory for changes, and runs the command you supply as arguments
// in response to changes

package main

import (
	"flag"
	"fmt"
	"github.com/howeyc/fsnotify"
	"os"
	"os/exec"
	"regexp"
	"time"
)

var (
	verbose       = flag.Bool("v", true, "Verbose")
	watchDir      = flag.String("dir", "", "Directory to watch, defaults to current")
	buildCmd      = flag.String("cmd", "", "Bash command to run on change. Wabo will wait for this command to finish.")
	daemonCmd     = flag.String("daemon", "", "Bash command that starts a daemon. Wago will halt if the daemon exits before the trigger or timer.")
	daemonTrigger = flag.String("trigger", "", "A string the daemon will output that indicates it has started successfuly. Wago will continue on this trigger.")
	daemonTimer   = flag.Int("timer", 0, "Miliseconds to wait after starting daemon before continuing.")
	webServer     = flag.String("web", "", "Start a web server at this address, e.g. :8420")
	postCmd       = flag.String("pcmd", "", "Bash command to run after the daemon has successfully started.")
	url           = flag.String("url", "", "URL to open")
	watchRegex = flag.String("watch", `/\w[\w\.]*": (CREATE|MODIFY)`, "Regex to match watch event. Use -v to see all events.")

	daemon = exec.Command("/bin/bash", "-c", *daemonCmd)
)


func help() {
	fmt.Println("WaGo (Wait, Go) build watcher")
}

func runCmd(cmds *string) bool {
		talk("Running command: ", *cmds)
		cmd := exec.Command("/bin/bash", "-c", *cmds)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		err := cmd.Run()

		if err != nil {
			Err("Error: ", err)
			return false
		}

		return true
}

func event() {
	// kill daemon if it's running
	if len(*daemonCmd) > 0 && daemon.Process != nil{
		talk("Killing daemon, pid: ", daemon.Process)
		if err := daemon.Process.Kill(); err != nil {
			Fatal("Failed to kill daemon: ", err)
		}
	}

	// run build command
	if len(*buildCmd) > 0 {
		ok := runCmd(buildCmd)
		if !ok {
			return
		}
	}

	// start the daemon
	if len(*daemonCmd) > 0 {
		if !startDaemon() {
			return
		}
	}

	// run post command
	if len(*postCmd) > 0 {
		ok := runCmd(postCmd)
		if !ok {
			return
		}
	}

	// open the url
	if len(*url) == 0 {
		openUrl()
	}
}

func startDaemon() bool {
	talk("Starting daemon: ", *daemonCmd)
	daemon = exec.Command("/bin/bash", "-c", *daemonCmd)

	if len(*daemonTrigger) == 0 {
		daemon.Stdout = os.Stdout
		daemon.Stderr = os.Stderr
	}

	err := daemon.Start()
	if err != nil {
		Err("Error starting daemon: ", err)
		return false
	}

	trigger := make(chan bool)

	if *daemonTimer > 0 {
		talk("Waiting for: ", *daemonTimer)

		go func() {
			time.Sleep(time.Duration(*daemonTimer) * time.Millisecond)
			trigger <- true
		}()
	}

	// wait for the tirgger
	<-trigger

	// ensure daemon is still running
	if daemon.ProcessState != nil {
		return false
	}

	return true
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
	talk("Watching dir: ", *watchDir)

	r, err := regexp.Compile(*watchRegex)
	if err != nil {
		Fatal("Watch regex compile error: ", err)
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

	// trigger event on start
	event()

	for {
		select {
		case ev := <-watcher.Event:
			if r.MatchString(ev.String()) {
				talk("Matched event: ", ev.String())
				event()
			} else {
				talk("Ignored event: ", ev.String())
			}

		case err = <-watcher.Error:
			panic(err)
		}
	}

}
