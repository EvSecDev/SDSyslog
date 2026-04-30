#!/bin/bash
command -v jq >/dev/null
command -v awk >/dev/null

function store_current_coverage_percent() {
	local storeFile currentPercent
	storeFile=$1
	currentPercent=$2

	currentPercent="${currentPercent%\%}"

	if [[ -z $repoRoot ]]; then
		echo "missing environment variable repoRoot"
		return 1
	fi

	local covDir covPath
	covDir="$repoRoot/.test-coverage"
	covPath="$covDir/$storeFile"

	if ! [[ -f $covPath ]]; then
		echo '{
    "lastCommitPct": 0,
    "lastBuildPct": 0
}
' >"$covPath"
	fi

	jq --argjson cov "$currentPercent" '.lastBuildPct = $cov' "$covPath" >"$covPath.tmp"
	mv "$covPath.tmp" "$covPath"
}

function store_commit_coverage_percent() {
	local storeFile currentPercent
	storeFile=$1
	currentPercent=$2

	currentPercent="${currentPercent%\%}"

	if [[ -z $repoRoot ]]; then
		echo "missing environment variable repoRoot"
		return 1
	fi

	local covDir covPath
	covDir="$repoRoot/.test-coverage"
	covPath="$covDir/$storeFile"

	if ! [[ -f $covPath ]]; then
		echo '{
    "lastCommitPct": 0,
    "lastBuildPct": 0
}
' >"$covPath"
	fi

	jq --argjson cov "$currentPercent" '.lastCommitPct = $cov' "$covPath" >"$covPath.tmp"
	mv "$covPath.tmp" "$covPath"
}

function get_last_coverage_percent() {
	local storeFile
	storeFile=$1

	if [[ -z $repoRoot ]]; then
		echo "missing environment variable repoRoot"
		return 1
	fi

	local covDir covPath
	covDir="$repoRoot/.test-coverage"
	covPath="$covDir/$storeFile"

	if ! [[ -f $covPath ]]; then
		echo "0"
		return 0
	fi

	local lastPercent
	lastPercent=$(jq '.lastBuildPct' <"$covPath" 2>&1)
	if [[ -z $lastPercent ]]; then
		echo "0"
		return 0
	fi

	echo "$lastPercent"
	return 0
}

function get_commit_coverage_percent() {
	local storeFile
	storeFile=$1

	if [[ -z $repoRoot ]]; then
		echo "missing environment variable repoRoot"
		return 1
	fi

	local covDir covPath
	covDir="$repoRoot/.test-coverage"
	covPath="$covDir/$storeFile"

	if ! [[ -f $covPath ]]; then
		echo "0"
		return 0
	fi

	local commitPercent
	commitPercent=$(jq '.lastCommitPct' <"$covPath" 2>&1)
	if [[ -z $commitPercent ]]; then
		echo "0"
		return 0
	fi

	echo "$commitPercent"
	return 0
}

diff_test_percent() {
	local lastPercent currentPercent
	lastPercent=$1
	currentPercent=${2%\%}

	awk -v last="$lastPercent" -v curr="$currentPercent" \
		-v bold="$BOLD" -v reset="$RESET" \
		-v green="$GREEN" -v red="$RED" '
    function abs(x) {
        return x < 0 ? -x : x
    }

    BEGIN {
        if (last == 0 || curr == 0) {
            print bold "no change" reset
            exit
        }

        diff = curr - last

        # ignore tiny changes
        if (abs(diff) < 0.2) {
            print bold "no change" reset
            exit
        }

        if (diff > 0) {
            printf "%s+%g%%%s\n", green, diff, reset
        } else {
            printf "%s%g%%%s\n", red, diff, reset
        }
    }
'
}

function create_coverage_diff_string() {
	local storeFile currentPercent
	storeFile=$1
	currentPercent=$2

	currentPercent="${currentPercent%\%}"

	if [[ -z $storeFile ]]; then
		echo "Coverage percent file not supplied"
		return 1
	fi
	if [[ -z $currentPercent ]]; then
		echo "Current coverage percent not supplied"
		return 1
	fi

	local lastPct commitPct lastChange commitChange
	lastPct=$(get_last_coverage_percent "$storeFile")
	if [[ $? != 0 ]]; then
		echo "$lastPct"
		return 1
	fi
	commitPct=$(get_commit_coverage_percent "$storeFile")
	if [[ $? != 0 ]]; then
		echo "$commitPct"
		return 1
	fi

	lastChange=$(diff_test_percent "$lastPct" "$currentPercent")
	commitChange=$(diff_test_percent "$commitPct" "$currentPercent")

	echo "Coverage Diff Since Last: build=$lastChange; commit=$commitChange"
	return 0
}

function run_tests() {
	local intenseTests preCommitMode pkgExitCode internalExitCode
	intenseTests=$1
	preCommitMode=$2

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

	set -e

	if [[ $pkgExitCode != 0 ]]; then
		echo -e "${RED}[-] FAILED TESTS${RESET}"
		return 1
	fi

	set +e

	# shellcheck disable=SC2046
	go -C "$repoRoot/$SRCdir" test "${testArgs[@]}" -coverprofile="$coverProfileOutSrc" \
		$(go list ./... | grep -Ev "internal/tests/integration|pkg/") \
		| grep -Ev "\[no test files\]|^PASS$|^goos: |^goarch: |^cpu: |coverage: 0.0% of statements$|^coverage: "
	internalExitCode=${PIPESTATUS[0]}

	set -e

	if [[ $internalExitCode != 0 ]]; then
		echo -e "${RED}[-] FAILED TESTS${RESET}"
		return 1
	fi

	echo -e "${GREEN}[+] DONE${RESET}"

	local coverPercentPkg coverPercentInt

	echo "[*] Test Coverage Totals:"

	coverPercentPkg=$(go tool cover -func="$coverProfileOutPkg" | grep "^total:" | awk '{print $3}')
	rm "$coverProfileOutPkg"

	coverPercentInt=$(go tool cover -func="$coverProfileOutSrc" | grep "^total:" | awk '{print $3}')
	rm "$coverProfileOutSrc"

	pkgCovDiff=$(create_coverage_diff_string "pkg-coverage.json" "$coverPercentPkg")
	if [[ $? != 0 ]]; then
		echo -e "${RED}[-] Error generating coverage diff for pkg:${RESET} $pkgCovDiff"
		return 1
	fi
	intCovDiff=$(create_coverage_diff_string "internal-coverage.json" "$coverPercentInt")
	if [[ $? != 0 ]]; then
		echo -e "${RED}[-] Error generating coverage diff for internal:${RESET} $intCovDiff"
		return 1
	fi

	echo -e "   [*] Package Coverage: ${BOLD}$coverPercentPkg${RESET} ($pkgCovDiff)"
	echo -e "   [*] Internal Coverage: ${BOLD}$coverPercentInt${RESET} ($intCovDiff)"

	local out
	out=$(store_current_coverage_percent "pkg-coverage.json" "$coverPercentPkg")
	if [[ $? != 0 ]]; then
		echo -e "${RED}[-] Error saving coverage for pkg:${RESET} $out"
		return 1
	fi
	out=$(store_current_coverage_percent "internal-coverage.json" "$coverPercentInt")
	if [[ $? != 0 ]]; then
		echo -e "${RED}[-] Error saving coverage for internal:${RESET} $out"
		return 1
	fi

	if [[ $preCommitMode == true ]]; then
		out=$(store_commit_coverage_percent "pkg-coverage.json" "$coverPercentPkg")
		if [[ $? != 0 ]]; then
			echo -e "${RED}[-] Error saving commit coverage for pkg:${RESET} $out"
			return 1
		fi
		out=$(store_commit_coverage_percent "internal-coverage.json" "$coverPercentInt")
		if [[ $? != 0 ]]; then
			echo -e "${RED}[-] Error saving commit coverage for internal:${RESET} $out"
			return 1
		fi
	fi

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

			set -e

			if [[ $exitCode != 0 ]]; then
				echo -e "${RED}[-] FAILED TEST: $testFunc${RESET}"
				return 1
			fi

			coverPercent=$(go tool cover -func="$coverProfileInteg" | grep "^total:" | awk '{print $3}')
			rm -f "$coverProfileInteg"

			set +e

			integCovDiff=$(create_coverage_diff_string "$testFunc-coverage.json" "$coverPercent")
			if [[ $? != 0 ]]; then
				echo -e "${RED}[-] Error generating coverage diff for integ test $testFunc:${RESET} $integCovDiff"
				return 1
			fi

			echo -e "   [*] Integ Coverage: ${BOLD}$coverPercent${RESET} ($pkgCovDiff)"

			out=$(store_current_coverage_percent "$testFunc-coverage.json" "$coverPercent")
			if [[ $? != 0 ]]; then
				echo -e "${RED}[-] Error saving coverage for integ test $testFunc:${RESET} $out"
				return 1
			fi

			if [[ $preCommitMode == true ]]; then
				out=$(store_commit_coverage_percent "$testFunc-coverage.json" "$coverPercent")
				if [[ $? != 0 ]]; then
					echo -e "${RED}[-] Error saving commit coverage for integ test $testFunc:${RESET} $out"
					return 1
				fi
			fi

			set -e

			echo -e "${GREEN}[+] DONE${RESET}"
		done
	fi
}
