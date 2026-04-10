#!/bin/bash

function run_tests() {
	local intenseTests pkgExitCode internalExitCode
	intenseTests=$1

	if [[ -z $repoRoot ]]; then
		echo -e "${RED}[-] ERROR:${RESET} Missing environment variable repoRoot"
		return 1
	fi
	if [[ -z $SRCdir ]]; then
		echo -e "${RED}[-] ERROR:${RESET} Missing environment variable SRCdir"
		return 1
	fi

	# Run tests (excludes unimportant info)
	echo "[*] Running tests..."

	local testArgs
	testArgs=(
		"-timeout=4m"
	)

	if [[ $intenseTests == 'true' ]]; then
		testArgs+=(
			"-race"
			"-bench=."
		)
	fi

	local coverProfileOutPkg coverProfileOutSrc
	coverProfileOutPkg="$repoRoot/coverprofile_pkg.out"
	coverProfileOutSrc="$repoRoot/coverprofile_src.out"

	# shellcheck disable=SC2064
	trap "rm -f $coverProfileOutPkg $coverProfileOutSrc" EXIT

	set +e

	go -C "$repoRoot/pkg" test "${testArgs[@]}" -coverprofile="$coverProfileOutPkg" ./... \
		| grep -Ev "\[no test files\]|^PASS$|coverage: 0.0% of statements$|^coverage: "
	pkgExitCode=${PIPESTATUS[0]}

	if [[ $pkgExitCode != 0 ]]; then
		echo -e "${RED}[-] FAILED TESTS${RESET}"
		return 1
	fi

	# shellcheck disable=SC2046
	go -C "$repoRoot/$SRCdir" test "${testArgs[@]}" -coverprofile="$coverProfileOutSrc" \
		$(go list ./... | grep -Ev "internal/tests/integration|pkg/") \
		| grep -Ev "\[no test files\]|^PASS$|^goos: |^goarch: |^cpu: |coverage: 0.0% of statements$|^coverage: "
	internalExitCode=${PIPESTATUS[0]}

	if [[ $internalExitCode != 0 ]]; then
		echo -e "${RED}[-] FAILED TESTS${RESET}"
		return 1
	fi

	echo -e "${GREEN}[+] DONE${RESET}"

	local coverPercent

	echo "[*] Test Coverage Totals:"

	coverPercent=$(go tool cover -func="$coverProfileOutPkg" | grep "^total:" | awk '{print $3}')
	echo -e "   [*] Package Coverage: ${BOLD}$coverPercent${RESET}"
	rm "$coverProfileOutPkg"

	coverPercent=$(go tool cover -func="$coverProfileOutSrc" | grep "^total:" | awk '{print $3}')
	echo -e "   [*] Internal Coverage: ${BOLD}$coverPercent${RESET}"
	rm "$coverProfileOutSrc"

	set -e

	echo -e "${GREEN}[+] DONE${RESET}"

	if [[ $intenseTests == 'true' ]]; then
		local integrationDir integrationTests coverProfileInteg testEntry exitCode

		# Running integ tests one by one for specific coverage calculations
		integrationDir="$repoRoot/$SRCdir/tests/integration"
		integrationTests=(
			"TestSendReceivePipeline:sdsyslog/$SRCdir/receiver,sdsyslog/$SRCdir/sender"
			"TestRecvConstantFlow:sdsyslog/$SRCdir/receiver"
			"TestMultipleSenders:sdsyslog/$SRCdir/receiver,sdsyslog/$SRCdir/sender"
		)

		coverProfileInteg="$repoRoot/coverprofile_integ.out"
		# shellcheck disable=SC2064
		trap "rm -f $coverProfileInteg" EXIT

		for testEntry in "${integrationTests[@]}"; do
			IFS=":" read -r testFunc coverPkg <<<"$testEntry"

			echo -e "[*] Running Integration Test ${BLUE}$testFunc${RESET}"
			set +e
			go -C "$repoRoot/$SRCdir" test -p 1 "${testArgs[@]}" \
				-covermode=atomic \
				-coverpkg="$coverPkg" \
				-coverprofile="$coverProfileInteg" \
				-run "^$testFunc$" \
				"$integrationDir" \
				| grep -Ev "\[no test files\]|^PASS$|^goos: |^goarch: |^cpu: |coverage: 0.0% of statements$|^coverage: "
			exitCode=${PIPESTATUS[0]}

			if [[ $exitCode != 0 ]]; then
				echo -e "${RED}[-] FAILED TEST: $testFunc${RESET}"
				return 1
			fi

			coverPercent=$(go tool cover -func="$coverProfileInteg" | grep "^total:" | awk '{print $3}')
			echo -e "   [*] Integ Coverage: ${BOLD}$coverPercent${RESET}"
			rm -f "$coverProfileInteg"

			set -e

			echo -e "${GREEN}[+] DONE${RESET}"
		done
	fi
}
