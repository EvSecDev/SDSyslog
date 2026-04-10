#!/bin/bash
command -v rsync >/dev/null
command -v jq >/dev/null
command -v xargs >/dev/null
command -v join >/dev/null
command -v awk >/dev/null

function update_go_packages {
	local repoDir src
	repoDir=$1
	src=$2

	cd "$repoDir/$src" || {
		echo "[-] ERROR: Failed to move into source directory"
		return 1
	}

	# Require clean repo
	if ! git diff --quiet || ! git diff --cached --quiet; then
		echo "[-] ERROR: Working tree not clean. Refusing to update when uncommitted changes exist."
		return 1
	fi

	echo "[*] Capturing current module state..."
	local beforeFile afterFile
	beforeFile=$(mktemp)
	afterFile=$(mktemp)

	go list -m all >"$beforeFile"

	echo "[*] Updating Go packages..."
	go get -u all || return 1
	go mod verify || return 1
	go mod tidy

	echo "[*] Capturing updated module state..."
	go list -m all >"$afterFile"

	echo "[*] Detecting changed modules..."

	# Extract changed modules (old vs new version)
	local changes
	changes=$(join -j1 \
		<(sort "$beforeFile") \
		<(sort "$afterFile") | awk '$2 != $3 {print $1, $2, $3}')

	if [[ -z "$changes" ]]; then
		echo "[+] No module changes detected"
		return 0
	fi

	echo
	echo "=========== MODULE SOURCE DIFFS ==========="

	while read -r module oldVer newVer; do
		echo
		echo "[*] $module"
		echo "    $oldVer -> $newVer"

		local baseTmp moduleSafe oldShort newShort
		baseTmp="$HOME/.local/tmp/gomoddiff"
		mkdir -p "$baseTmp"

		moduleSafe=$(sanitize_module_name "$module")
		oldShort=$(short_version "$oldVer")
		newShort=$(short_version "$newVer")

		local tmpOld tmpNew oldSrc newSrc

		tmpOld="$baseTmp/${moduleSafe}@${oldShort}/old"
		tmpNew="$baseTmp/${moduleSafe}@${newShort}/new"

		oldSrc="$tmpOld/src"
		newSrc="$tmpNew/src"

		# Clean any previous runs
		rm -rf "$tmpOld" "$tmpNew"

		mkdir -p "$oldSrc" "$newSrc"

		echo "    [-] Downloading old version..."
		go mod download -json "$module@$oldVer" \
			| jq -r '.Dir' \
			| xargs -I {} cp -a {} "$oldSrc"

		if [[ $(du -sm "$oldSrc" | cut -f1) -gt 100 ]]; then
			echo "    [!] Skipping large module (>100MB)"
			continue
		fi

		echo "    [+] Downloading new version..."
		go mod download -json "$module@$newVer" \
			| jq -r '.Dir' \
			| xargs -I {} cp -a {} "$newSrc"

		mkdir -p "$tmpOld/filtered" "$tmpNew/filtered"

		echo "    [*] Preparing filtered trees..."

		rsync -a --delete \
			--exclude='*_test.go' --exclude='testdata/' \
			"$oldSrc/" "$tmpOld/filtered/"

		rsync -a --delete \
			--exclude='*_test.go' --exclude='testdata/' \
			"$newSrc/" "$tmpNew/filtered/"

		echo "    [*] Diff (excluding tests)..."

		local diffOutput diffLines

		diffOutput=$(git -c color.ui=always diff --no-index \
			"$tmpOld/filtered" "$tmpNew/filtered")

		diffLines=$(printf "%s\n" "$diffOutput" | wc -l)

		if ((diffLines < 20)); then
			printf "%s\n" "$diffOutput"
		else
			printf "%s\n" "$diffOutput" | less -R
		fi

		# Ensure writable before cleanup (handles chmod 400 files)
		chmod -R u+w "$tmpOld" "$tmpNew" 2>/dev/null

		rm -rf "$tmpOld" "$tmpNew"
	done <<<"$changes"

	echo "==========================================="
	echo

	read -r -p "[?] Accept these changes? (y/N): " confirm
	if [[ "$confirm" != "y" && "$confirm" != "Y" ]]; then
		echo "[-] Reverting changes..."
		git reset --hard HEAD
		return 1
	fi

	echo "[+] DONE"
}

sanitize_module_name() {
	echo "$1" | sed 's|/|_|g'
}

short_version() {
	local v="$1"
	# Trim long pseudo-versions: v0.0.0-20240101-abcdef123456 → v0.0.0-abcdef
	if [[ "$v" =~ ^v[0-9]+\.[0-9]+\.[0-9]+- ]]; then
		sed -E 's/(v[0-9]+\.[0-9]+\.[0-9]+-[0-9]+)-([a-f0-9]{6}).*/\1-\2/' <<<"$v"
	else
		echo "$v"
	fi
}
