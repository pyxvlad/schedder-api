#!/bin/bash

clean() {
	sudo rm -rf postgres appdata
	echo "Friendly reminder: next time run using: $0 firstrun"
}

build() {
	docker build . --tag=schedder-api:latest --network=host --file Dockerfile
	echo "Done, now run it using $0 firstrun or $0 run"
}

run() {
	echo "Starting containerized environment... Press CTRL+C to close this"
	docker compose up
}

firstrun() {
	echo "Starting containerized environment... Press CTRL+C to close this"
	docker compose up -d database
	sleep 5s;
	docker compose up
}

help() {
	echo "Schedder Docker Script:"
	echo "	$0 build - build the container image"
	echo "	$0 clean - clean the container's data (i.e. delete postgres folder)"
	echo "	$0 run - run the containerized environment"
	echo "	$0 firstrun - same as run, except it waits for the database to init"
	echo "	$0 help - show this message"
}

SUBCOMMAND=$1

case "$SUBCOMMAND" in
build) build ;;
clean) clean ;;
run) run ;;
firstrun) firstrun;;
help) help ;;
*) help ;;
esac
