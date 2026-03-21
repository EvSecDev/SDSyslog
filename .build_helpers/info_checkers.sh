#!/bin/bash

print_dependency_tree() {
	declare -A children
	declare -A visited

	while read -r parent child; do
		children["$parent"]+="$child "
	done < <(go mod graph)

	root=$(go list -m)

	print_tree() {
		local node=$1
		local prefix=$2

		echo "${prefix}${node}"

		visited["$node"]=1

		for child in ${children[$node]}; do
			if [[ -z "${visited[$child]}" ]]; then
				print_tree "$child" "  ${prefix}"
			else
				echo "${prefix}  ${child} (already shown)"
			fi
		done
	}

	print_tree "$root" ""
}
