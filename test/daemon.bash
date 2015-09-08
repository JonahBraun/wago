#!/bin/bash

simulate_change() {
	sleep 3
	touch tmp.1 && touch tmp.2 && touch tmp.3 && touch tmp.4 && touch tmp.5
	rm tmp.*

	sleep 5
	touch tmp.1 && touch tmp.2 && touch tmp.3 && touch tmp.4 && touch tmp.5
	rm tmp.*

	sleep 7
	touch tmp.1 && touch tmp.2 && touch tmp.3 && touch tmp.4 && touch tmp.5
	rm tmp.*

	sleep 15
	touch tmp.1 && touch tmp.2 && touch tmp.3 && touch tmp.4 && touch tmp.5
	rm tmp.*
}

simulate_change &

go install -race && wago -v\
	-cmd='echo "BUILDCMD 1s" && sleep 1' \
	-daemon='test/mock_daemon.bash' \
	-pcmd='echo POSTCMD' \
	-web=':4567' \
	-url='http://localhost:4567/main.go' \
	-timer=7000 \
	#-trigger='quick' \
	#-url='http://localhost:80/'
