wago -cmd='echo "enter a line, it will be echoed" && read a && echo $a' \
	-daemon='echo "enter a line, it will be echoed" && read a && echo $a' \
	-pcmd='echo "enter a line, it will be echoed twice!" && read a && echo $a' 
