*Update 19 Oct 2015*: Wago 1.2.0 beta released, [release notes, download available](https://github.com/JonahBraun/wago/releases).

**Do you routinely?:**

1. Save your code.
2. Do something in a terminal: kill a command, restart a daemon, wait for stuff to finish successfully, refresh a browser.
3. *Repeat…*

Wago<sup>Watch, Go</sup> watches your code, then starts a conditional action chain capable of process monitoring and management.

## Examples
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
* Watch your **Compass/SASS** directory for changes. Recompile and refresh your Chrome tab so you can see the results. `compass watch` will also watch your files, but does so with far greater processor usage.
```bash
wago -dir sass/ -cmd 'compass compile' -url 'http://localhost:8080/somewhere.html'
```
* Recursively develop Wago!
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

1. `-cmd` is run and waited to finish.
1. `-daemon` is run. If `-trigger`, chain continues after `-daemon` outputs the exact trigger string. Otherwise, `-timer` milliseconds is waited and then the chain continues.
1. `-pcmd` is run and waited to finish.
1. `-url` is opened.

When a matching file system event occurs, all actions are killed and the chain is started from the beginning.

Commands are executed by `-shell`, which defaults to your current shell. This allows you to do stuff like `some_command && some_script.sh`. Output and input are connected to your terminal so that you can interact with commands. Note that `-daemon` and `-pcmd` will run concurrently and input will be sent to both. If you require distinct input, use shell I/O redirection or wrap a command in a script.

Wago reports actions as they occur. Once you are comfortable with what is happening, consider using `-q` to make things less noisy.

### File system events
Wago begins by recursively (`-recursive` defaults to true) watching all the directories in `-dir` except for those matching `-ignore`.

Events are ignored unless they match `-watch`. You can listen for all sorts of events, even deletes. Use `-v` to see all events and modify `-watch` accordingly.

Regex explained:
- **-ignore** `\.(git|hg|svn)` Ignore directories a dot followed by either git, hg, or svn.
- **-watch** `/[^\.][^/]*": (CREATE|MODIFY$)` Only react to CREATE and MODIFY events where the filename (everything after the last /) does not start with a dot. A simple regex to watch all files is: `(CREATE|MODIFY)$`

### Webserver
If you are developing a static site, Wago can run a static web server for you. To start it, set the port number(s) with `-http` and/or `-h2`.

`-h2` serves both HTTPS (TLS) and HTTP2 on the same port. A X.509 certificate is required for this and one is included in Wago. Because the private key is published in GitHub, **the bundled certificate is unsafe** and should only be used over local and private networks. You can set your own certificate with `-key` and `-cert`.

To generate your own self-signed certificate pair, try this command:
```bash
yes "" | openssl req -x509 -newkey rsa:2048 -keyout key.pem -out cert.pem -days 4000 -nodes
```

# Command Reference
Run Wago without any switches to get this reference:
```
WaGo (Watch, Go) build tool. Version 1.2.0
  -cert string
    	X.509 cert file for HTTP2/TLS, eg: cert.pem
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
  -h2 string
    	Start a HTTP/TLS server on this port, e.g. :8421
  -http string
    	Start a HTTP server on this port, e.g. :8420
  -ignore string
    	Ignore directories matching regex. (default "\\.(git|hg|svn)")
  -key string
    	X.509 key file for HTTP2/TLS, eg: key.pem
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
    	React to FS events matching regex. Use -v to see all events. (default "/[^\\.][^/]*\": (CREATE|MODIFY$)")
  -webroot string
    	Local directory to use as root for web server, defaults to -dir.
```

# Troubleshooting

### ☠  Error… too many open files
Use `-dir` to specify a subdirectory or set `-recursive=false`. Another option is to expand the regex of `-ignore` which will prevent directories from being watched.

You can also raise the open file limit for your system. Try `ulimit -n` to see the current limit and raise it with `ulimit -n 2000`.

### Orphaned sub processes or resources are being left open
Short answer: Try increasing `-exitwait` to something longer than the default of 50ms.

Explanation: Wago runs commands in a new process group, sends SIGTERM, waits `-exitwait`, then sends SIGKILL if the process group is still running. Some commands (eg: Elixir) will spin up their own subprocesses in a new process group which will not receive WaGo's signals. Your command should be cleaning up for exit when it receives SIGTERM, so check that it is doing so. 50ms should be long enough in most circumstances. If you continue to have problems with a popular tool or library, please open an issue. 
