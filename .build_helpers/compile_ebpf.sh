#!/bin/bash
command -v bpftool >/dev/null
command -v clang >/dev/null
command -v llvm-objdump >/dev/null

function compile_ebpf_c() {
    local cSrcDir outDir outputFileName
    cSrcDir=$1
    outDir=$2

    if [[ -z $SRCdir ]]; then
        return 1
    fi

    echo "[*] Compiling eBPF bytecode... "

    if ! [[ -f /sys/kernel/btf/vmlinux ]]; then
        echo "Cannot compile on system that does not support eBPF" >&2
        return 1
    fi

    if ! [[ -d /usr/include/bpf ]]; then
        echo "Missing bpf header files, please install libbpf-dev" >&2
        return 1
    fi

    if ! [[ -f $cSrcDir/include/vmlinux.h ]]; then
        bpftool btf dump file /sys/kernel/btf/vmlinux format c >"$cSrcDir/include/vmlinux.h"
    fi

    outputFileName=$(grep -hoPm1 "(?<=byteCodeFS.ReadFile\(\"static-files/)[a-zA-Z\.]+(?=\"\))" "$SRCdir/ebpf/"*.go | head -n1)

    # Compile to bpf bytecode (not setting all problems to fail it, gets in the way of ebpf)
    clang -target bpf -O2 -g -D__TARGET_ARCH_x86 \
        -std=gnu11 \
        -fdebug-prefix-map="$(pwd)"=. \
        -isystem "$cSrcDir/include" \
        -isystem /usr/include \
        -Wall \
        -Wextra \
        -Werror \
        -Wshadow \
        -Wundef \
        -Wpointer-arith \
        -Wcast-align \
        -Wstrict-prototypes \
        -Wmissing-prototypes \
        -Wmissing-declarations \
        -Wredundant-decls \
        -Wwrite-strings \
        -Wformat=2 \
        -Wformat-security \
        -Wnull-dereference \
        -Wimplicit-fallthrough \
        -Wswitch-enum \
        -Wconversion \
        -Wsign-conversion \
        -fno-strict-aliasing \
        -fwrapv \
        -fno-delete-null-pointer-checks \
        -Wno-padded \
        -Wno-packed \
        -Wno-missing-field-initializers \
        -Wno-unused-parameter \
        -Wno-unused-macros \
        -Wno-disabled-macro-expansion \
        -Wno-reserved-id-macro \
        -Wno-gnu-anonymous-struct \
        -Wno-nested-anon-types \
        -Wno-language-extension-token \
        -Wno-vla \
        -c "$cSrcDir/reuseport_drain.bpf.c" \
        -o "$outDir/$outputFileName"

    # Test
    llvm-objdump -d "$outDir/$outputFileName" &>/dev/null

    echo -e "   ${GREEN}[+] DONE${RESET}"
}
