package main

import (
	"os/exec"
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

func openUrl() {
	if *url == "" {
		return
	}

	Note("Opening url (macosx/chrome):", *url)

	cmd := exec.Command("osascript")

	in, err := cmd.StdinPipe()
	if err != nil {
		panic(err)
	}

	in.Write([]byte(chrome_applescript))
	in.Close()

	output, err := cmd.CombinedOutput()

	if err != nil {
		Fatal("AppleScript Error:", string(output))
	}
}
