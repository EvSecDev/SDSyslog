package ebpf

import (
	"bytes"
	"sdsyslog/internal/global"
	"testing"

	"github.com/cilium/ebpf"
)

func TestLoadProgram(t *testing.T) {
	ebpfByteCode, err := byteCodeFS.ReadFile("static-files/socket.o")
	if err != nil {
		t.Fatalf("failed to read bytecode: %v", err)
		return
	}

	spec, err := ebpf.LoadCollectionSpecFromReader(bytes.NewReader(ebpfByteCode))
	if err != nil {
		t.Fatalf("failed to load eBPF spec: %v", err)
		return
	}

	if len(spec.Programs) == 0 {
		t.Fatalf("no programs found in eBPF object")
	}
	if len(spec.Maps) == 0 {
		t.Fatalf("no maps found in eBPF object")
	}

	prog, ok := spec.Programs[global.DrainFuncName]
	if !ok {
		t.Fatalf("expected program %s not found", global.DrainFuncName)
	}

	if len(prog.Instructions) == 0 {
		t.Fatalf("program has no instructions")
	}
}
