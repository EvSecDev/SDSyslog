_sdsyslog() {
    local cur prev words cword
    _init_completion || return

    # Main config of options
    declare -A COMMANDS=(
        [root_sub]="configure receive send version"
        [root_opts]="-c --config -v --verbosity"

        [configure_opts]="--create-keys --recv-config-template --send-config-template -c --config -v --verbosity --uninstall-sender --uninstall-receiver --install-sender --install-receiver"
        [receive_opts]="__inherit__"
        [send_opts]="__inherit__"
        [version_opts]="__inherit__"
    )

    # Special completion options
    case "$prev" in
    --verbosity | -v | --verbose)
        mapfile -t COMPREPLY < <(compgen -W "0 1 2 3 4 5" -- "$cur")
        return 0
        ;;
    esac

    # Walk commands
    local path="root"
    for ((i = 1; i < COMP_CWORD; i++)); do
        local word="${COMP_WORDS[i]}"
        local subs="${COMMANDS[${path}_sub]}"

        if [[ -n "$subs" && " $subs " == *" $word "* ]]; then
            # descend into this subcommand
            [[ "$path" == "root" ]] && path="$word" || path="$path:$word"
        else
            # no deeper subcommand match, stop here
            break
        fi
    done

    # Main suggestions
    local subs="${COMMANDS[${path}_sub]}"
    local opts="${COMMANDS[${path}_opts]}"

    # Handle flag to use parent args instead of explicit ones
    if [[ "$opts" == "__inherit__" ]]; then
        # Only inherit from immediate parent
        if [[ "$path" == *:* ]]; then
            local parent="${path%:*}"
        else
            local parent="root"
        fi
        opts="${COMMANDS[${parent}_opts]}"
    fi

    local globals="${COMMANDS[root_opts]}"

    local suggestions="$subs $opts $globals"

    if [[ "$cur" != -* ]]; then
        # Base completions
        mapfile -t COMPREPLY < <(compgen -W "$suggestions" -- "$cur")

        # Append file/dir matches
        local files
        mapfile -t files < <(compgen -f -- "$cur")
        COMPREPLY+=("${files[@]}")

        # If current word resolves to a directory then no space after completion
        for i in "${!COMPREPLY[@]}"; do
            if [[ -d "${COMPREPLY[$i]}" ]]; then
                compopt -o nospace
                # shellcheck disable=SC2004
                COMPREPLY[$i]="${COMPREPLY[$i]}/"
            fi
        done
    else
        # Options only
        mapfile -t COMPREPLY < <(compgen -W "$suggestions" -- "$cur")
    fi
}
complete -F _sdsyslog sdsyslog
