#!/bin/bash
command -v jq >/dev/null

PERMITTED_LICENSES=(
	"MIT"
	"BSD"
	"BSD-2-Clause"
	"BSD-3-Clause"
	"Apache-2.0"
	"ISC"
)
DISALLOWED_LICENSES=(
	"GPL"
	"AGPL"
	"LGPL"
	"UNKNOWN"
	"MPL"
)

retrieve_license_name() {
	local licenseText="$1"

	case "$licenseText" in
		*MIT*) echo "MIT" ;;
		*BSD*2*) echo "BSD-2-Clause" ;;
		*BSD*3*) echo "BSD-3-Clause" ;;
		*BSD*) echo "BSD" ;;
		*Apache*) echo "Apache-2.0" ;;
		*LESSER*GENERAL*PUBLIC*) echo "LGPL" ;;
		*GENERAL*PUBLIC*LICENSE*) echo "GPL" ;;
		*AFFERO*) echo "AGPL" ;;
		*Mozilla*Public*) echo "MPL" ;;
		*ISC\ License*) echo "ISC" ;;
		*) echo "UNKNOWN" ;;
	esac
}

find_license_file() {
	local dir="$1"
	find "$dir" -maxdepth 2 -type f \( \
		-iname "LICENSE*" -o \
		-iname "COPYING*" \) \
		| head -n 1
}

in_array() {
	local val="$1"
	shift
	for item in "$@"; do
		if [[ "$item" == "$val" ]]; then
			return 0
		fi
	done
	return 1
}

function retrieve_modules() {
	local modules
	modules=$(go list -m -json all | jq -r 'select(.Main != true) | .Path + "@" + .Version')
	if [[ -z "$modules" ]]; then
		echo -e "[+] No dependencies found"
		return 0
	fi
	echo "$modules"
}

function retrieve_module_license() {
	local module json dir license_file
	module=$1

	if [[ -z "$module" ]]; then
		return 2
	fi

	json=$(go mod download -json "$module" 2>/dev/null || true)
	dir=$(jq -r '.Dir' <<<"$json")

	if [[ -z "$dir" || ! -d "$dir" ]]; then
		echo -e "${YELLOW}[?] WARNING${RESET}: Could not download $module"
		return 255
	fi

	license_file=$(find_license_file "$dir")
	if [[ -z "$license_file" ]]; then
		echo -e "${YELLOW}[?] WARNING${RESET}: $module: NO LICENSE FOUND"
		return 1
	fi
	if ! [[ -s $license_file ]]; then
		echo -e "${YELLOW}[?] WARNING${RESET}: $module: LICENSE FILE IS EMPTY"
		return 1
	fi

	cat "$license_file"
}

function check_package_licenses() {
	local verboseMode FAIL license exitCode licenseName
	verboseMode=$1

	echo -e "[*] Checking licenses for Go dependencies..."

	FAIL=0
	while read -r module; do
		license=$(retrieve_module_license "$module")
		exitCode=$?
		if [[ $exitCode == 2 ]]; then
			continue
		elif [[ $exitCode -gt 1 ]]; then
			echo -e "$license"
			licenseName="UNKNOWN"
		else
			licenseName=$(retrieve_license_name "$license")
		fi

		# Policy enforcement
		if in_array "$licenseName" "${DISALLOWED_LICENSES[@]}"; then
			echo -e "  ${RED}[-] ERROR${RESET}: $module uses disallowed license: $licenseName"
			FAIL=1
		elif in_array "$licenseName" "${PERMITTED_LICENSES[@]}"; then
			if [[ $verboseMode == true ]]; then
				printf "  ${GREEN}[+] VALID${RESET}: %-75s - %s\n" "$module" "$licenseName"
			fi
			:
		else
			echo -e "  ${RED}[-] ERROR${RESET}: $module has unclassified license: $licenseName"
			FAIL=1
		fi
	done < <(retrieve_modules)

	if [[ "$FAIL" -eq 1 ]]; then
		echo -e "${RED}[-] ERROR${RESET}: License compliance check FAILED"
		return 1
	else
		echo -e "${GREEN}[+] DONE${RESET}"
	fi
}

function generate_third_party_licenses() {
	local output FAIL license exitCode licenseName name version hash
	output=$1
	if [[ -z $output ]]; then
		echo "No output file supplied"
		return 1
	fi

	echo "[*] Generating third party license file in \"$output\"..."

	echo -ne "THIRD PARTY LICENSES\n\n" >"$output"
	echo -ne "This distribution includes third-party software components.\n\n" >>"$output"

	declare -A LICENSE_DEDUP

	FAIL=0
	while read -r module; do
		license=$(retrieve_module_license "$module")
		exitCode=$?
		if [[ $exitCode -gt 1 ]]; then
			FAIL=1
			continue
		fi

		licenseName=$(retrieve_license_name "$license")

		name="${module%@*}"
		version="${module#*@}"

		hash=$(printf "%s" "$license" | sha256sum | awk '{print $1}')

		{
			echo "------------------------------------------------------------"
			echo "Module: $name"
			echo "Version: $version"
			echo "License: $licenseName"
			echo "Source: https://$name"
			echo "------------------------------------------------------------"
			echo ""

			if [[ -z "${LICENSE_DEDUP[$hash]}" ]]; then
				# First time seeing this license
				LICENSE_DEDUP["$hash"]="$name@$version"

				echo "$license"
				echo ""
				echo "[License ID: $hash]"
			else
				# Duplicate license
				echo "License text identical to:"
				echo "  ${LICENSE_DEDUP[$hash]}"
				echo "[License ID: $hash]"
			fi

			echo ""
			echo ""
		} >>"$output"
	done < <(retrieve_modules)

	if [[ "$FAIL" -eq 1 ]]; then
		echo -e "${RED}[-] ERROR: Some dependencies were skipped due to license issues"
		return 1
	fi

	echo -e "${GREEN}[+] DONE${RESET}"
}
