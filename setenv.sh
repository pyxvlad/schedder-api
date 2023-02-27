#!/usr/bin/env bash

$(return >/dev/null 2>&1)

if [[ "$?" -eq "0" ]]; then
	echo "Loading $1 env"
else
	echo "Do not execute this script!"
	echo "Source it, like: source setenv.sh testing"
	exit
fi

if [[ ! -e "$1.env" ]]; then
	echo "Error: Couldn't find $1.env"
	if [[ -e "$1.env.example" ]]; then
		echo "Hint: Probably you should copy $1.env.example to $1.env and modify it."
	fi
else
	# Show env vars
	grep -v '^#' $1.env

	# Export env vars
	export $(grep -v '^#' $1.env | xargs)
fi

