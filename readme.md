Wago (Watch, Go) development tool

## Some things you can use Wago to do:
* Watch your JS or SASS directory for changes. Lint, recompile, etc., then refresh your Chrome tab so you can see the results. `wago `
* Watch the go source directory of the webapp your building. On change: stop the webapp, recompile, start, wait for the webapp to load it's database, then run a cURL command to test a REST URL. `wago `
* Run a webserver in the current directory for a one-off JS test page. `wago -fiddle`

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

### To do

* Support for refreshing tabs in other browsers and operating systems.
