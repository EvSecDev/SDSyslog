#!/bin/bash
if [ -z "$BASH_VERSION" ]; then
	echo "This script must be run in BASH." >&2
	exit 1
fi

# Define colors - unsupported terminals fail safe
if [ -t 1 ] && { [[ "$TERM" =~ "xterm" ]] || [[ "$COLORTERM" == "truecolor" ]] || tput setaf 1 &>/dev/null; }; then
	readonly RED='\033[31m'
	readonly GREEN='\033[32m'
	readonly YELLOW='\033[33m'
	readonly BLUE='\033[34m'
	readonly RESET='\033[0m'
	readonly BOLD='\033[1m'
fi

readonly configFile="build.conf"
# shellcheck source=./build.conf
source "$configFile"
if [[ $? != 0 ]]; then
	echo -e "${RED}[-] ERROR${RESET}: Failed to import build config variables in $configFile" >&2
	exit 1
fi

# Bail on any failure
set -e

# Check for required commands
command -v go >/dev/null
command -v sha256sum >/dev/null

# Variables
repoRoot=$(pwd)
progVersion=$(grep -E "^\s*ProgVersion\s*string" "$SRCdir/global/consts.go" | grep -oPm1 "(?<=\")\S+(?=\")")
if [[ -z $progVersion ]]; then
	echo "Failed to retrieve version from source code" >&2
	exit 1
fi

# Check for required external variables
if [[ -z $HOME ]]; then
	echo -e "${RED}[-] ERROR${RESET}: Missing HOME variable" >&2
	exit 1
fi
if [[ -z $repoRoot ]]; then
	echo -e "${RED}[-] ERROR${RESET}: Failed to determine current directory" >&2
	exit 1
fi

# Load external functions
while IFS= read -r -d '' helperFunction; do
	source "$helperFunction"
	if [[ $? != 0 ]]; then
		echo -e "${RED}[-] ERROR${RESET}: Failed to import build helper functions" >&2
		exit 1
	fi
done < <(find .build_helpers/ -maxdepth 1 -type f -iname "*.sh" -print0)

##################################
# MAIN BUILD
##################################

function compile_program_prechecks() {
	# Always ensure we start in the root of the repository
	cd "$repoRoot"/

	# Check for things not supposed to be in a release
	if type -t check_for_dev_artifacts &>/dev/null; then
		check_for_dev_artifacts "$SRCdir" "$repoRoot"
		check_for_dev_artifacts "$repoRoot/pkg" "$repoRoot"
	fi
}

function compile_program() {
	local GOARCH GOOS buildFull deployedBinaryPath buildVersion skipTests intenseTests
	GOARCH=$1
	GOOS=$2
	buildFull=$3
	skipTests=$4
	intenseTests=$5

	# eBPF program
	if type -t compile_ebpf_c &>/dev/null; then
		compile_ebpf_c "$repoRoot/ebpf" "$SRCdir/ebpf/static-files"
	fi

	if [[ $skipTests != 'true' ]]; then
		# Run tests (excludes unimportant info)
		echo "[*] Running all tests..."

		local testArgs
		testArgs=(
			"-timeout=4m"
			"-count=1"
		)

		if [[ $intenseTests == 'true' ]]; then
			testArgs+=(
				"-race"
				"-bench=."
			)
		fi

		local coverProfileOutPkg coverProfileOutSrc coverPercent pkgExitCode internalExitCode
		coverProfileOutPkg="$repoRoot/coverprofile_pkg.out"
		coverProfileOutSrc="$repoRoot/coverprofile_src.out"

		# shellcheck disable=SC2064
		trap "rm -f $coverProfileOutPkg $coverProfileOutSrc" EXIT

		set +e

		go -C "$repoRoot/pkg" test "${testArgs[@]}" -coverprofile="$coverProfileOutPkg" ./... |
			grep -Ev "\[no test files\]|^PASS$|coverage: 0.0% of statements$|^coverage: "
		pkgExitCode=${PIPESTATUS[0]}

		if [[ $pkgExitCode != 0 ]]; then
			echo -e "   ${RED}[-] FAILED TESTS${RESET}"
			exit 1
		fi

		go -C "$repoRoot/$SRCdir" test "${testArgs[@]}" -coverprofile="$coverProfileOutSrc" ./... |
			grep -Ev "\[no test files\]|^PASS$|^goos: |^goarch: |^cpu: |coverage: 0.0% of statements$|^coverage: "
		internalExitCode=${PIPESTATUS[0]}

		if [[ $internalExitCode != 0 ]]; then
			echo -e "   ${RED}[-] FAILED TESTS${RESET}"
			exit 1
		fi

		set -e
		echo -e "   ${GREEN}[+] DONE${RESET}"
		echo "[*] Test Coverage Totals:"

		coverPercent=$(go tool cover -func="$coverProfileOutPkg" | grep "^total:" | awk '{print $3}')
		echo " Protocol Coverage: $coverPercent"
		rm "$coverProfileOutPkg"

		coverPercent=$(go tool cover -func="$coverProfileOutSrc" | grep "^total:" | awk '{print $3}')
		echo " Internal Coverage: $coverPercent"
		rm "$coverProfileOutSrc"

		echo -e "   ${GREEN}[+] DONE${RESET}"
	fi

	echo "[*] Compiling program binary..."

	# Vars for build
	export CGO_ENABLED=0
	export GOARCH
	export GOOS

	# Build binary
	go build -C "$repoRoot/cmd/sdsyslog" -trimpath -o "$repoRoot/$outputEXE" -a -ldflags '-s -w -buildid= -extldflags "-static"'

	# Get help menu
	helpMenu=$(./$outputEXE -h)

	# Rename to more descriptive if full build was requested
	if [[ $buildFull == true ]]; then
		local fullNameEXE

		# Rename with version
		fullNameEXE="${outputEXE}_${progVersion}_${GOOS}-${GOARCH}-static"
		mv "$outputEXE" "$fullNameEXE"

		# Create hash for built binary
		sha256sum "$fullNameEXE" >"$fullNameEXE".sha256
	fi

	# Ensure readme has updated code blocks
	if type -t update_readme &>/dev/null; then
		update_readme "$helpMenu" "$srcHelpMenuStartDelimiter" "$readmeHelpMenuStartDelimiter"
	fi

	echo -e "   ${GREEN}[+] DONE${RESET}: Built version ${BOLD}${BLUE}$progVersion${RESET}"
}

##################################
# START
##################################

function usage {
	echo "Usage $0
Program Build Script and Helpers

Options:
  -b           Build the program using defaults
  -n           Skip tests
  -f           Run intense tests (-race -bench)
  -a <arch>    Architecture of compiled binary (amd64, arm64) [default: amd64]
  -o <os>      Which operating system to build for (linux, windows) [default: linux]
  -u           Update go packages for program
  -p           Prepare release notes and attachments
  -P <version> Publish release to github
  -h           Print this help menu
"
}

# DEFAULTS
architecture="amd64"
os="linux"
skipTests='false'
intenseTests='false'

# Argument parsing
while getopts 'a:o:P:bfunph' opt; do
	case "$opt" in
	'a')
		architecture="$OPTARG"
		;;
	'b')
		buildmode='true'
		;;
	'n')
		skipTests='true'
		;;
	'f')
		intenseTests='true'
		;;
	'o')
		os="$OPTARG"
		;;
	'u')
		updatepackages='true'
		;;
	'p')
		prepareRelease='true'
		;;
	'P')
		publishVersion="$OPTARG"
		;;
	'h')
		usage
		exit 0
		;;
	*)
		usage
		exit 0
		;;
	esac
done

if [[ $prepareRelease == true ]]; then
	compile_program_prechecks
	compile_program "$architecture" "$os" 'true' 'true'
	tempReleaseDir=$(prepare_github_release_files "$fullNameProgramPrefix")
	create_release_notes "$repoRoot" "$tempReleaseDir"
elif [[ -n $publishVersion ]]; then
	create_github_release "$githubUser" "$githubRepoName" "$publishVersion"
elif [[ $updatepackages == true ]]; then
	update_go_packages "$repoRoot" "$SRCdir"
elif [[ $buildmode == true ]]; then
	compile_program_prechecks
	compile_program "$architecture" "$os" 'false' "$skipTests" "$intenseTests"
else
	echo -e "${RED}ERROR${RESET}: Unknown option or combination of options" >&2
	exit 1
fi

exit 0
