#!/bin/bash

echo "START daemon in 5s" && for i in {1..3}; do
	sleep 5 && echo -e "$i.BEGIN The quick brown fox jumps over the lazy dog $(date) EOL"
done
