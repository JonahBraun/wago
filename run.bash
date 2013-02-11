#!/bin/bash

simulate_change() {
	sleep 10
	for i in {1..4}; do
		echo "touching"
		touch tmp.$(date +%s)
		sleep 4
	done
	rm tmp.*
}

simulate_change &

go install && wago \
	-cmd='sleep 2s && echo "BUILDCMD"' \
	-daemon='./test_daemon.bash' \
	-trigger='Quick' \
	-pcmd='echo POSTCMD' \
	#-timer=700 \
	#-url='http://localhost:80/'
