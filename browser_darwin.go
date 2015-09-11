package main

import (
	"fmt"
	"os/exec"
)

var chromeApplescript = `
  tell application "Google Chrome"
    activate
    set theUrl to "%v"
    
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

func NewBrowser(url string) Runnable {
	return func(kill chan struct{}) (chan bool, chan struct{}) {
		cmd := &Cmd{
			Cmd:  exec.Command("osascript"),
			Name: url,
			done: make(chan bool, 1),
			dead: make(chan struct{}),
		}

		go cmd.RunBrowser(url)

		return cmd.done, cmd.dead
	}
}

func (cmd *Cmd) RunBrowser(url string) {
	defer close(cmd.done)
	defer close(cmd.dead)

	in, err := cmd.StdinPipe()
	if err != nil {
		panic(err)
	}

	in.Write([]byte(fmt.Sprintf(chromeApplescript, url)))
	in.Close()

	log.Info("Opening url (macosx/chrome):", url)

	output, err := cmd.CombinedOutput()

	if err != nil {
		log.Fatal("AppleScript Error:", string(output))(3)
	}
}
