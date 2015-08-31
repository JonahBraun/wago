package main

import (
	"fmt"
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

type Browser struct {
	*Cmd
	url string
}

func NewBrowser(url string) *Browser {
	return &Browser{url: url}
}

func (c *Browser) Run() bool {
	c.Cmd = NewCmd("osascript")

	in, err := c.StdinPipe()
	if err != nil {
		panic(err)
	}

	go func(cmd *Cmd) {
		in.Write([]byte(fmt.Sprintf(chromeApplescript, c.url)))
		in.Close()

		log.Info("Opening url (macosx/chrome):", *url)

		output, err := cmd.CombinedOutput()

		if cmd.killed {
			return
		}

		if err != nil {
			log.Fatal("AppleScript Error:", string(output))(3)
		}

		// finished successfully
		machine.Trans <- "next"
	}(c.Cmd)

	return true
}
