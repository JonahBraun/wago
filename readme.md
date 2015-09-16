*Update 2016-Sep-16*: WaGo 1.0.1 released, [download available](https://github.com/JonahBraun/wago/releases).

**Do you routinely?:**

1. Save your code.
2. Do something in a terminal: kill a command, restart a daemon, wait for stuff to finish successfully, refresh a browser.
3. *Repeat…*

WaGo watches your code, then starts a conditional action chain capable of process monitoring and management.

## Example WaGo<sup>Watch, Go</sup> Usage
* Run a Ruby script.
```bash
wago -cmd='ruby script.rb'
```
* Watch your **Go** webapp, test, install, launch server, wait for it to connect to the DB, kick off a custom curl test suite.
```bash
wago -cmd='go test -race -short && go install -race' -daemon='appName' -timer=35 -pcmd='test_suite.sh'
```
* Watch your **Elixir** webapp, restarting iex, waiting for it to load, refreshing Chrome. You can still interact with iex between builds!
```bash
wago -q -dir=lib -daemon='iex -S mix' -trigger='iex(1)>' -url='http://localhost:8123/'
```
* Watch your **Compass/SASS** directory for changes. Recompile and refresh your Chrome tab so you can see the results. `compass watch` will also watch your files, but Wago is orders of magnitude more effecient and so better for laptop battery life.
```bash
wago -dir sass/ -cmd 'compass compile' -url 'http://localhost:8080/somewhere.html'
```
* Recursively develop WaGo!
```bash
wago -q -ignore='(\.git|tmp)' -cmd='go install -race' -daemon='wago -v -dir tmp -cmd "echo foo"' -pcmd='touch tmp/a && rm tmp/a'
```
* Run a **static webserver** in the current directory for a one-off HTML/CSS/JS test page.
```bash
wago -fiddle
```

## Install
Go (golang), requires Go 1.5+: `go get github.com/JonahBraun/wago`

Mac OS X, Intel (darwin/amd64): [Download from the Releases page](https://github.com/JonahBraun/wago/releases)

Linux (amd64): [Download from the Releases page](https://github.com/JonahBraun/wago/releases)

# How it Works
### Action Chain
Actions are run in the following order. All actions are optional but there must be at least one. The chain is stopped if an action fails (exit status >0).

1. -cmd is run and waited to finish.
1. -daemon is run. If -trigger, chain continues after -daemon outputs the exact trigger string. Otherwise, -timer milliseconds is waited and then the chain continues.
1. -pcmd is run and waited to finish.
1. -url is opened.

When a matching file system event occurs, all actions are killed and the chain is started from the beginning.

Commands are executed by -shell, which defaults to your current shell. This allows you to do stuff like `some_command && some_script.sh`. Output and input are connected to your terminal so that you can interact with commands. Note that -daemon and -pcmd will run concurrently and input will be sent to both. If you require distinct input, use shell I/O redirection or wrap a command in a script.

WaGo reports actions as they occur. Once you are comfortable with what is happening, consider using -q to make things less noisy.

### File system events
WaGo begins by recursively (-recursive defaults to true) watching all the directories in -dir except for those matching -ignore.

Events are ignored unless they match -watch. You can listen for all sorts of events, even deletes. Use -v to see all events and modify -watch accordingly.

### Webserver
If you are developing a static site, WaGo can run a static web server for you. Set the port with -web to start it.


# Command Reference
Run WaGo without any switches to get this reference:
```
WaGo (Watch, Go) build tool. Version 1.0
  -cmd string
    	Run command, wait for it to complete.
  -daemon string
    	Run command and leave running in the background.
  -dir string
    	Directory to watch, defaults to current.
  -exitwait int
    	Max miliseconds a process has after a SIGTERM to exit before a SIGKILL. (default 50)
  -fiddle
    	CLI fiddle mode! Start a web server, open browser to URL of targetDir/index.html
  -ignore string
    	Ignore directories matching regex. (default "\\.(git|hg|svn)")
  -pcmd string
    	Run command after daemon starts. Use this to kick off your test suite.
  -q	Quiet, only warnings and errors
  -recursive
    	Watch directory tree recursively. (default true)
  -shell string
    	Shell used to run commands, defaults to $SHELL, fallback to /bin/sh
  -timer int
    	Wait miliseconds after starting daemon, then continue.
  -trigger string
    	Wait for daemon to output this string, then continue.
  -url string
    	Open browser to this URL after all commands are successful.
  -v	Verbose
  -watch string
    	React to FS events matching regex. Use -v to see all events. (default "/\\w[\\w\\.]*\": (CREATE|MODIFY)")
  -web string
    	Start a web server at this address, e.g. :8420
```

# Troubleshooting

### ☠  Error… too many open files
Use -dir to specify a subdirectory or set -recursive=false. Another option is to expand the regex of -ignore which will prevent directories from being watched.

You can also raise the open file limit for your system. Try `ulimit -n` to see the current limit and raise it with `ulimit -n 2000`.

### Orphaned sub processes or resources are being left open
Short answer: Try increasing -exitwait to something longer than the default of 50ms.

Explanation: WaGo runs commands in a new process group, sends SIGTERM, waits -exitwait, then sends SIGKILL if the process group is still running. Some commands (eg: Elixir) will spin up their own subprocesses in a new process group which will not receive WaGo's signals. Your command should be cleaning up for exit when it receives SIGTERM, so check that it is doing so. 50ms should be long enough in most circumstances. If you continue to have problems with a popular tool or library, please open an issue. 
