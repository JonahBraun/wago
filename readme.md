# Wago (Watch, Go) development tool

Use Wago to automatically build a program when you save changes. Specially built for webapps, Wago will run a daemon, wait for it to start and then refresh a browser tab.

Written with Go but as a tool Wago is language agnostic.

## Features

* (optional) Start a static web server
* Watch a directory for file change events

When an event occurs, a pipeline of actions is started. All actions are optional, but must succeed for the next action to occur:

1. Run a command
1. Start a daemon, waiting a number of miliseconds or waiting for certain output before continuing
1. Run a command
1. Open a url in a browser (currently limited to Mac OS X and Chrome)

Wago features a "CLI fiddle" mode that watches the current directory, starts a web server, and opens Chrome to index.html (or a directory listing if index.html is not present in the directory).

Bash is used to execute all commands. I/O is connected properly, so you can use Wago with interactive commands.

### To do

* Recursive directory watching
* Support for refreshing tabs in other browsers and operating systems.

This is my first Go app. Feedback and pull requests are welcome!
