#!/bin/bash

simulate_change() {
	sleep 10
	for i in {1..1}; do
		echo "touching"
		touch tmp.$(date +%s)
		sleep 4
	done
	rm tmp.*
}

simulate_change &

go install && wago \
	-cmd='sleep 1s && echo "BUILDCMD"' \
	-daemon='./test_daemon.bash' \
	-trigger='Quick' \
	-pcmd='echo POSTCMD' \
	-web=':4567' \
	-url='http://localhost:4567/main.go' \ 
	#-timer=700 \
	#-url='http://localhost:80/'
