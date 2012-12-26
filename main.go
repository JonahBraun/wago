// Watches the current directory for changes, and runs the command you supply as arguments
// in response to changes

package main

import (
	"flag"
	"fmt"
	"github.com/howeyc/fsnotify"
	"log"
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

var chrome_applescript = `
  tell application "Google Chrome"
    activate
    set theUrl to "` + *url + `"
    
    if (count every window) = 0 then
      make new window
    end if
    
    set found to false
    set theTabIndex to -1
    repeat with theWindow in every window
      set theTabIndex to 0
      repeat with theTab in every tab of theWindow
        set theTabIndex to theTabIndex + 1
        if theTab's URL = theUrl then
          set found to true
          exit
        end if
      end repeat
      
      if found then
        exit repeat
      end if
    end repeat
    
    if found then
      tell theTab to reload
      set theWindow's active tab index to theTabIndex
      set index of theWindow to 1
    else
      tell window 1 to make new tab with properties {URL:theUrl}
    end if
  end tell
`

func help() {
	fmt.Println("WaGo (Wait, Go) build watcher")
}

func action() {

	if len(*buildCmd) > 0 {
		talk("Running -cmd: ", *buildCmd)
		cmd := exec.Command("/bin/bash", "-c", *buildCmd)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		err := cmd.Run()

		if err != nil {
			log.Print(FgRed, "Error executing -cmd: ", err, TR)
		}
	}

	if len(*daemonCmd) > 0 {
		talk("Running -daemon: ", *daemonCmd)
		cmd := exec.Command("/bin/bash", "-c", *daemonCmd)

		if len(*daemonTrigger) == 0 {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		}

		err := cmd.Start()

		if *daemonTimer > 0 {
			talk("Waiting -timer: ", *daemonTimer)

			go func() {
				time.Sleep(time.Duration(*daemonTimer) * time.Millisecond)
				openUrl()
			}()
		}

		if err != nil {
			log.Print(FgRed, "Error executing -cmd: ", err, TR)
		}
	}

}

func openUrl() {
	if *url == "" {
		return
	}

	talk("Opening url (macosx/chrome): ", *url)

	cmd := exec.Command("osascript")

	in, err := cmd.StdinPipe()
	if err != nil {
		panic(err)
	}

	in.Write([]byte(chrome_applescript))
	in.Close()

	output, err := cmd.CombinedOutput()

	if err != nil {
		Err("AppleScript Error: ", string(output))
	}
}

func main() {
	if len(os.Args) < 2 {
		help()
		log.Fatal("Must specify an action")
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
		log.Panic("fsnotify error: ", err)
	}
	defer watcher.Close()

	err = watcher.Watch(*watchDir)
	if err != nil {
		log.Panic("error setting up watcher on -dir ", *watchDir, ": ", err)
	}

	// perform action on start
	action()

	for {
		select {
		case ev := <-watcher.Event:
			talk("event: ", ev.String())
			action()

		case err = <-watcher.Error:
			log.Panic("error with watcher: ", err)
		}
	}

}
