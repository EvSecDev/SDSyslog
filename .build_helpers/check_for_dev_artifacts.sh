#!/bin/bash
command -v git >/dev/null

function check_for_dev_artifacts() {
	local repoDir headCommitHash lastReleaseCommitHash lastReleaseVersionNumber currentVersionNumber
	repoDir=$1

	# Always ensure we start in the root of the repository
	cd "$repoDir"/

	echo "[*] Checking for development artifacts in source code..."

	# Get head commit hash
	headCommitHash=$(git rev-parse HEAD)

	# Get commit where last release was generated from
	lastReleaseCommitHash=$(cat "$repoDir"/.last_release_commit)

	# Retrieve the program version from the last release commit
	lastReleaseVersionNumber=$(git show "$lastReleaseCommitHash":"cmd/sdsyslog/main.go" 2>/dev/null | grep "progVersion string" | cut -d" " -f5 | sed 's/"//g')

	# Get the current version number
	currentVersionNumber=$(grep "progVersion string" "cmd/sdsyslog/main.go" | cut -d" " -f5 | sed 's/"//g')

	# Exit if version number hasn't been upped since last commit
	if [[ $lastReleaseVersionNumber == $currentVersionNumber ]] && ! [[ $headCommitHash == $lastReleaseCommitHash ]] && [[ -n $lastReleaseVersionNumber ]]; then
		echo -e "${RED}[-] ERROR${RESET}: Version number in cmd/sdsyslog/main.go has not been bumped since last commit, exiting build"
		exit 1
	fi

	# Quick check for any left over debug prints
	if grep -ER "DEBUG" "$repoDir"/ | grep -Ev "ebpf/include/"; then
		echo -e "   ${YELLOW}[?] WARNING${RESET}: Debug print found in source code. You might want to remove that before release."
	fi

	# Want functions that have an actual body attached (safety)
	if grep -RIn --include="*.go" -E 'var[[:space:]]+[a-zA-Z0-9_]+[[:space:]]+func\(' . | grep -Ev ") = "; then
		echo -e "   ${YELLOW}[?] WARNING${RESET}: Found function variables that do not have an actualy function set."
	fi

	echo -e "${GREEN}[+] DONE${RESET}"
	echo "[*] Running static analysis..."

	# Source Linting
	set +e
	# Disagree on including name in description comments
	staticcheck -checks all ./... | grep -Ev "comment on exported method|package comment should be of the form|comment on exported type|comment on exported function"
	go vet ./...
	if [[ -x $(which golangci-lint) ]]; then
		golangci-lint run ./...
	fi
	set -e

	echo -e "${GREEN}[+] DONE${RESET}"
}
