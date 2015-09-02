#!/bin/bash

simulate_change() {
	sleep 1
	touch tmp.1 && touch tmp.2 && touch tmp.3 && touch tmp.4 && touch tmp.5
	rm tmp.*

	sleep 2
	touch tmp.1 && touch tmp.2 && touch tmp.3 && touch tmp.4 && touch tmp.5
	rm tmp.*

}

simulate_change &
go install -race && wago -cmd 'echo foo' -v
