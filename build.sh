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
	# Check for things not supposed to be in a release
	if type -t check_for_dev_artifacts &>/dev/null; then
		check_for_dev_artifacts "$repoRoot"
	fi
}

function compile_program() {
	GOARCH=$1
	GOOS=$2
	buildFull=$3

	# eBPF program
	if type -t compile_ebpf_c &>/dev/null; then
		compile_ebpf_c "$repoRoot/ebpf" "$SRCdir/ebpf/static-files"
	fi

	echo "[*] Compiling program binary..."

	# Vars for build
	export CGO_ENABLED=0
	export GOARCH
	export GOOS

	# Build binary
	go build -trimpath -o "$repoRoot/" -a -ldflags '-s -w -buildid= -extldflags "-static"' "$repoRoot/cmd/..."

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

	echo -e "   [*] Built version ${BOLD}${BLUE}$progVersion${RESET}"
	echo -e "${GREEN}[+] DONE${RESET}"
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
  -o <os>      Which operating system to build for (linux, freebsd) [default: linux]
  -u           Update go packages for program
  -D           Print dependency tree
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
while getopts 'a:o:P:bfuDnph' opt; do
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
		'D')
			printDepTree='true'
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
	intenseTests=true
	buildFull=true

	compile_program_prechecks
	check_package_licenses 'false'
	run_tests "$intenseTests"
	compile_program "$architecture" "$os" "$buildFull"

	tempReleaseDir=$(prepare_github_release_files "$fullNameProgramPrefix")
	generate_third_party_licenses "$tempReleaseDir/THIRD_PARTY_LICENSES.txt"
	create_release_notes "$tempReleaseDir"
elif [[ -n $publishVersion ]]; then
	create_github_release "$githubUser" "$githubRepoName" "$publishVersion"
elif [[ $updatepackages == true ]]; then
	update_go_packages
	check_package_licenses 'true'
elif [[ $printDepTree == true ]]; then
	print_dependency_tree
elif [[ $buildmode == true ]]; then
	buildFull=false

	compile_program_prechecks
	if [[ $skipTests == false ]]; then
		run_tests "$intenseTests"
	fi
	compile_program "$architecture" "$os" "$buildFull"
else
	echo -e "${RED}ERROR${RESET}: Unknown option or combination of options" >&2
	exit 1
fi

exit 0
