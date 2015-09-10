Do you routinely?:
1. Save your code.
2. Do something in a terminal (kill a command, run a server) or refresh a browser.
3. Repeat…

Then Wago was built for you! Wago<sup>Watch, Go</sup> watches your filesystem and responds by building, monitoring servers, refreshing Chrome and more.

## Example Wago Usage
* Watch your **JS or SASS directory for changes. Lint, recompile, etc., then refresh your Chrome tab so you can see the results. `wago `
* Watch your **Elixir** webapp, restarting iex, waiting for it to load, refreshing Chrome. You can still interact with iex between builds!: `wago -q -dir=lib -exitwait=3 -daemon='iex -S mix' -trigger='iex(1)>' -url='http://localhost:8123/'`
* Watch your **Go** webapp, test, install, launch server, wait for it to connect to the DB, kick off a custom cURL test suite:`wago -cmd='go test -race' -daemon='go install -race' -timer=35 -pcmd='test_suite.sh'`
* Recursively develop Wago!: `wago -q -ignore='(\.git|tmp)' -cmd='go install -race' -daemon='wago -v -dir tmp -cmd "echo foo"' -pcmd='touch tmp/a && rm tmp/a'`
* Run a **static webserver** in the current directory for a one-off HTML/CSS/JS test page. `wago -fiddle`

## Install
Go (golang), requires Go 1.5+: `go get github.com/JonahBraun/wago`
Mac OS X, Intel (amd64): Binary to be posted soon.
Linux (amd64): Binary to be posted soon.

## Features

* Watch a directory for file change events (filterable with regex)
* Start and manage the processes of commands and daemons
* Run a static web server

When a file change event occurs, a chain of actions is started. All actions are optional, but must succeed for the next action to occur:

1. Run a command
1. Start a daemon, wait a number of miliseconds or wait for certain output before continuing
1. Run a command
1. Open a url in a browser (currently limited to MacOSX/Chrome)

If a new event occurs when the chain is running, all processes are killed and the chain is started again.

Wago features a "CLI fiddle" mode that watches the current directory, starts a web server, and opens Chrome to index.html (or a directory listing if index.html is not present in the directory).

I/O is connected to the user's shell, so you can use Wago with interactive commands.
