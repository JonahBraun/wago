go install && wago \
	-cmd='sleep 3s && echo "done sleeping 3s"' \
	-daemon='echo "begin daemon" && while true; do echo "more output foo bar $(date)" && sleep 10s; done' \
	-timer=700 \
	#-url='http://localhost:80/'
