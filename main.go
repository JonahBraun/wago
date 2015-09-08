// Wago (Watch, Go)
// A general purpose watch / build development tool.

// TODO: catch SIGINT and send dog.TR to ensure a clean term
// see https://askubuntu.com/questions/171449/shell-does-not-show-typed-in-commands-reset-works-but-what-happened

package main

import (
	"flag"
	"fmt"
	"github.com/JonahBraun/dog"
	"github.com/howeyc/fsnotify"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"
)

var (
	log     = dog.NewDog(dog.DEBUG)
	verbose = flag.Bool("v", false, "Verbose")
	quiet   = flag.Bool("q", false, "Quiet, only warnings and errors")

	buildCmd      = flag.String("cmd", "", "Bash command to run on change, Wabo will wait for this command to finish")
	daemonCmd     = flag.String("daemon", "", "Bash command that starts a daemon, Wago will halt if the daemon exits before the trigger or timer")
	daemonTimer   = flag.Int("timer", 0, "Miliseconds to wait after starting daemon before continuing")
	daemonTrigger = flag.String("trigger", "", "A string the daemon will output that indicates it has started successfuly, Wago will continue on this trigger")
	exitWait      = flag.Int("exitwait", 0, "If 0, kills processes immediately, if >0, sends SIGINT and waits X ms for process to exit before killing")
	fiddle        = flag.Bool("fiddle", false, "CLI fiddle mode, starts a web server and opens url to targetDir/index.html")
	leader        = flag.String("leader", "", "Leader character for wago output (to differentiate from command output), defaults to emoji")
	postCmd       = flag.String("pcmd", "", "Bash command to run after the daemon has successfully started, use this to kick off your test suite")
	recursive     = flag.Bool("recursive", true, "Watch directory tree recursively")
	targetDir     = flag.String("dir", "", "Directory to watch, defaults to current")
	url           = flag.String("url", "", "URL to open")
	watchRegex    = flag.String("watch", `/\w[\w\.]*": (CREATE|MODIFY)`, "Regex to match watch event, use -v to see all events")
	webServer     = flag.String("web", "", "Start a web server at this address, e.g. :8420")
	shell         = flag.String("shell", "", "Shell used to run commands, defaults to $SHELL, fallback to /bin/sh")
)

type Watcher struct {
	Event chan fmt.Stringer
	Error chan error
}

func main() {
	// the following function calls merely serve to logically organize what
	// is otherwise a VERY lengthy setup

	// TODO: have configSetup return a config object so that the reliance on
	// config globals is removed
	configSetup()

	startWebServer()

	runChain(newWatcher(), make(chan struct{}))
}

func runChain(watcher *Watcher, quit chan struct{}) {
	chain := make([]Runnable, 0, 5)

	// build chain of runnables
	if len(*buildCmd) > 0 {
		chain = append(chain, NewRunWait(*buildCmd))
	}
	if len(*daemonCmd) > 0 {
		if len(*daemonTrigger) > 0 {
			chain = append(chain, NewDaemonTrigger(*daemonCmd, *daemonTrigger))
		} else {
			chain = append(chain, NewDaemonTimer(*daemonCmd, *daemonTimer))
		}
	}
	if len(*postCmd) > 0 {
		chain = append(chain, NewRunWait(*postCmd))
	}
	if *url != "" {
		chain = append(chain, NewBrowser(*url))
	}

	eventRegex, err := regexp.Compile(*watchRegex)
	if err != nil {
		log.Fatal("Watch regex compile error:", err)(1)
	}

	var wg sync.WaitGroup

	// main loop
	for {
		// all channels of struct{} are disposable, single use
		// kill is passed to all Runnable so they know when they should exit
		kill := make(chan struct{})
		// abort is to single the event loop that a Runnable did not succeed so cancel everything
		abort := make(chan struct{})

		var drain func()
		drain = func() {
			select {
			case ev := <-watcher.Event:
				log.Debug("Extra event ignored:", ev.String())
				drain()
			default:
			}
		}
		drain()

		// event loop
		go func() {
			for {
				select {
				case ev := <-watcher.Event:
					if eventRegex.MatchString(ev.String()) {
						log.Info("Matched event:", ev.String())
						close(kill)
						return
					} else {
						log.Debug("Ignored event:", ev.String())
					}
				case err = <-watcher.Error:
					log.Fatal("Watcher error:", err)(5)
				case <-abort:
					close(kill)
					return
				case <-quit:
					// currently only used by test suite
					close(kill)
					return
				}
			}
		}()

	RunLoop:
		for _, runnable := range chain {
			done, dead := runnable(kill)
			wg.Add(1)

			go func() {
				wg.Done()
				<-dead
			}()

			select {
			case d := <-done:
				if !d {
					// Runnable failed it's success metric
					close(abort)
					break RunLoop
				}
			case <-kill:
				break RunLoop
			}
		}

		// ensure an event has occured, we may be here because all runnables completed
		<-kill

		// ensure all runnables (procs) are dead before restarting the chain
		wg.Wait()

		// check if we should quit, currently only used by test suites for teardown
		select {
		case <-quit:
			log.Warn("Quitting run chain")
			return
		default:
		}
	}
}

func newWatcher() *Watcher {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic(err)
	}

	watchDir := func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			return nil
		}

		log.Debug("Watching dir:", path)

		if err != nil {
			log.Err("Skipping dir:", path, err)
			return filepath.SkipDir
		}

		err = watcher.Watch(path)
		if err != nil {
			panic(err)
		}

		return nil
	}

	if *recursive == true {
		err = filepath.Walk(*targetDir, watchDir)
		if err != nil {
			panic(err)
		}
	} else {
		err = watcher.Watch(*targetDir)
		if err != nil {
			panic(err)
		}
	}

	// To facilitate testing (which sends artifical events from a timer),
	// we have an abstracted struct Watcher that holds the applicable channels.
	// fsnotify.FileEvent is a fmt.Stringer, but channels cannot be converted.
	// Unfortunately, an extra channel is necessary to perform the conversion.
	event := make(chan fmt.Stringer)
	go func() {
		for {
			event <- <-watcher.Event
		}
	}()

	return &Watcher{event, watcher.Error}
}

func startWebServer() {
	if *webServer != "" {
		go func() {
			log.Info("Starting web server on port", *webServer)
			err := http.ListenAndServe(*webServer, http.FileServer(http.Dir(*targetDir)))
			if err != nil {
				log.Fatal("Error starting web server:", err)(2)
			}
		}()
	}
}

func configSetup() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	flag.Usage = func() {
		fmt.Println("WaGo (Watch, Go) build tool. Usage:")
		flag.PrintDefaults()
	}

	// TODO: this should check for actions
	if len(os.Args) < 2 {
		flag.Usage()
		log.Fatal("You must specify an action")(1)
	}

	flag.Parse()

	if *verbose {
		log = dog.NewDog(dog.DEBUG)
	} else if *quiet {
		log = dog.NewDog(dog.WARN)
	} else {
		log = dog.NewDog(dog.INFO)
	}

	if len(*shell) == 0 {
		*shell = os.Getenv("SHELL")
		if len(*shell) == 0 {
			*shell = "/bin/sh"
		}
	}
	log.Debug("Using shell", *shell)

	if (len(*daemonTrigger) > 0) && (*daemonTimer > 0) {
		log.Fatal("Both daemon trigger and timer specified, use only one")(1)
	}

	if (len(*daemonTrigger) > 0 || *daemonTimer > 0) && len(*daemonCmd) == 0 {
		log.Fatal("Specify a daemon command to use the trigger or timer")(1)
	}

	if len(*buildCmd) == 0 && len(*daemonCmd) == 0 && !*fiddle && len(*postCmd) == 0 && len(*url) == 0 && len(*webServer) == 0 {
		flag.Usage()
		log.Fatal("You must specify an action")(1)
	}

	if *fiddle {
		if *webServer == "" {
			*webServer = ":9933"
		}
		if *url == "" {
			*url = "http://localhost" + *webServer + "/"
		}
	}

	if *targetDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			panic(err)
		}
		targetDir = &cwd
	}
	log.Debug("Target dir:", *targetDir)
}
