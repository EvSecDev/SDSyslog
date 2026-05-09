package build

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sdsyslog/cmd/builder/build/helpers"
	"strings"
)

// Custom steps to perform prior to build
func preCompile(ctx *context) (err error) {
	printInfo(0, "Compiling eBPF bytecode...")

	_, err = os.Stat(btfHeader)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		err = fmt.Errorf("could not determine if system supports eBPF: %w", err)
		return
	} else if err != nil && errors.Is(err, os.ErrNotExist) {
		err = fmt.Errorf("cannot compile on system that does not support eBPF")
		return
	}

	info, err := os.Stat(bpfIncludes)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		err = fmt.Errorf("could not determine if system has eBPF headers: %w", err)
		return
	} else if err != nil && errors.Is(err, os.ErrNotExist) {
		err = fmt.Errorf("missing bpf header files, please install libbpf-dev")
		return
	}
	if !info.IsDir() {
		err = fmt.Errorf("bpf header files directory not a directory")
		return
	}

	cSrcDir := filepath.Join(ctx.repositoryRoot, "ebpf")

	headerFile := filepath.Join(cSrcDir, "include", "vmlinux.h")

	_, err = os.Stat(headerFile)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		err = fmt.Errorf("could not determine if header file exists: %w", err)
		return
	} else if err != nil && errors.Is(err, os.ErrNotExist) {
		cmd := exec.Command("bpftool", "btf",
			"dump", "file",
			btfHeader, "format", "c",
		)
		var out []byte
		out, err = cmd.CombinedOutput()
		if err != nil {
			err = fmt.Errorf("bpftool: %w: %s", err, string(out))
			return
		}
		err = os.WriteFile(headerFile, out, 0600)
		if err != nil {
			err = fmt.Errorf("failed to write new header file to repo: %w", err)
			return
		}
	}

	outputDir := filepath.Join(ctx.repositoryRoot, "internal", "ebpf", "static-files")

	found, err := helpers.ScanRepo(filepath.Join(ctx.repositoryRoot, "internal", "ebpf"), false, func(path, line string) (matches bool) {
		if !strings.HasSuffix(path, ".go") {
			return
		}
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "byteCodeFS.ReadFile") {
			return
		}
		if !strings.Contains(line, "static-files/") {
			return
		}
		matches = true
		return
	})
	if err != nil {
		err = fmt.Errorf("scan repo (ebpf static file name): %w", err)
		return
	}
	if len(found) == 0 {
		err = fmt.Errorf("could not find bytecode embed fs file name in eBPF go source dir")
		return
	}
	result := found[0] // pick first

	// Extract file name from line. Example:
	//  ebpfByteCode, err := byteCodeFS.ReadFile("static-files/socket.o")
	parts := strings.Split(result.Line, "static-files/")
	if len(parts) != 2 {
		err = fmt.Errorf("failed parsing ebpf embed fs line '%s'in file '%s'", result.Line, result.Path)
		return
	}
	partialName := parts[1]
	outputFileName := strings.TrimRight(partialName, ")")
	outputFileName = strings.TrimRight(outputFileName, `"`)
	outputFilePath := filepath.Join(outputDir, outputFileName)

	// Compile to bpf bytecode (not setting all problems to fail it, gets in the way of ebpf)
	cmd := exec.Command("clang", "-target", "bpf", "-O2", "-g", "-D__TARGET_ARCH_x86",
		"-std=gnu11",
		"-fdebug-prefix-map="+ctx.repositoryRoot+"=.",
		"-isystem", filepath.Join(cSrcDir, "include"),
		"-isystem", "/usr/include",
		"-Wall",
		"-Wextra",
		"-Werror",
		"-Wshadow",
		"-Wundef",
		"-Wpointer-arith",
		"-Wcast-align",
		"-Wstrict-prototypes",
		"-Wmissing-prototypes",
		"-Wmissing-declarations",
		"-Wredundant-decls",
		"-Wwrite-strings",
		"-Wformat=2",
		"-Wformat-security",
		"-Wnull-dereference",
		"-Wimplicit-fallthrough",
		"-Wswitch-enum",
		"-Wconversion",
		"-Wsign-conversion",
		"-fno-strict-aliasing",
		"-fwrapv",
		"-fno-delete-null-pointer-checks",
		"-Wno-padded",
		"-Wno-packed",
		"-Wno-missing-field-initializers",
		"-Wno-unused-parameter",
		"-Wno-unused-macros",
		"-Wno-disabled-macro-expansion",
		"-Wno-reserved-id-macro",
		"-Wno-gnu-anonymous-struct",
		"-Wno-nested-anon-types",
		"-Wno-language-extension-token",
		"-Wno-vla",
		"-c", filepath.Join(cSrcDir, "reuseport_drain.bpf.c"),
		"-o", outputFilePath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		err = fmt.Errorf("failed testing byte code with llvm-objdump: %w: %s", err, string(out))
		return
	}

	// Sanity test
	cmd = exec.Command("llvm-objdump", "-d", outputFilePath)
	out, err = cmd.CombinedOutput()
	if err != nil {
		err = fmt.Errorf("failed testing byte code with llvm-objdump: %w: %s", err, string(out))
		return
	}

	printSuccess(0, "Done")
	return
}

// Compile executable binary
func compile(ctx *context, longName bool) (err error) {
	err = preCompile(ctx)
	if err != nil {
		err = fmt.Errorf("failed pre-compile step(s): %w", err)
		return
	}

	printInfo(0, "Compiling program binary...")

	// Extract program version from source
	mainConstsFile := filepath.Join(ctx.repositoryRoot, globalConstsFile)
	constsFile, err := os.ReadFile(mainConstsFile)
	if err != nil {
		err = fmt.Errorf("failed to read global consts: %w", err)
		return
	}
	progVersion, err := helpers.GetProgVersion(constsFile, versionVariableName)
	if err != nil {
		return
	}

	// Compile command
	cmd := exec.Command("go", "build",
		"-trimpath",
		"-o", ctx.repositoryRoot+"/",
		"-a",
		"-ldflags", `-s -w -buildid= -extldflags "-static"`,
		ctx.repositoryRoot+"/cmd/"+ctx.cfg.ProgramOutputName,
	)

	// Set env vars
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env,
		"CGO_ENABLED=0",
		"GOARCH="+ctx.cliOpts.Architecture,
		"GOOS="+ctx.cliOpts.OperatingSystem,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		err = fmt.Errorf("go build: %w: %s", err, string(out))
		return
	}

	var outputFileName string
	if longName {
		oldBinaryFile := filepath.Join(ctx.repositoryRoot, ctx.cfg.ProgramOutputName)
		newBinaryFile := fmt.Sprintf("%s_%s_%s-%s-static",
			ctx.cfg.ProgramOutputName,
			progVersion,
			ctx.cliOpts.OperatingSystem,
			ctx.cliOpts.Architecture,
		)
		err = os.Rename(oldBinaryFile, newBinaryFile)
		if err != nil {
			err = fmt.Errorf("failed to rename binary to full name: %w", err)
			return
		}
		outputFileName = newBinaryFile

		var binaryContents []byte
		binaryContents, err = os.ReadFile(newBinaryFile)
		if err != nil {
			err = fmt.Errorf("failed to read new binary file for hashing: %w", err)
			return
		}

		hash := helpers.Hash(binaryContents)
		err = os.WriteFile(newBinaryFile+".sha256", []byte(hash), 0600)
		if err != nil {
			err = fmt.Errorf("failed to write binary hash to file: %w", err)
			return
		}
	} else {
		outputFileName = ctx.cfg.ProgramOutputName
	}

	printInfo(4, "Built version %s%s%s%s", colorBold, colorBlue, progVersion, noColor)
	printSuccess(0, "Done")

	err = postCompile(ctx, outputFileName)
	if err != nil {
		err = fmt.Errorf("failed post-compile step(s): %w", err)
		return
	}

	return
}

// Custom steps to perform after the build
func postCompile(ctx *context, outFileName string) (err error) {
	printInfo(0, "Copying program help menu from source file to README...")

	readmePath := filepath.Join(ctx.repositoryRoot, mainREADME)
	editStartDelimiter := "```bash"
	editEndDelimiter := "```"

	// Retrieve current help menu
	cmd := exec.Command(filepath.Join(ctx.repositoryRoot, outFileName), "-h")
	helpMenu, err := cmd.CombinedOutput()
	if err != nil {
		err = fmt.Errorf("failed testing output binary: %w: %s", err, string(helpMenu))
		return
	}
	if len(helpMenu) == 0 {
		err = fmt.Errorf("no help menu retrieved from compiled binary, cannot update readme")
		return
	}

	readme, err := os.ReadFile(readmePath)
	if err != nil {
		err = fmt.Errorf("failed reading README file: %w", err)
		return
	}

	lines := bytes.Split(readme, []byte("\n"))
	var updatedLines [][]byte

	newHelpMenuLines := bytes.Split(helpMenu, []byte("\n"))

	// Edit state tracking
	inTargetSection := false
	inCodeBlock := false
	replaced := false

	for _, line := range lines {
		if !inTargetSection && bytes.HasPrefix(line, []byte(ctx.cfg.ReadmeHelpMenuStartDelimiter)) {
			// Target markdown section reached, begin search for code block start
			inTargetSection = true
		}

		if inTargetSection && bytes.HasPrefix(line, []byte(editStartDelimiter)) {
			// Code block start found
			inCodeBlock = true
			inTargetSection = false // Short circuit this conditional on later loops

			// Keep the current code block start line though
			updatedLines = append(updatedLines, line)

			if replaced {
				// Hard fail if duplicates
				err = fmt.Errorf("found duplicate readme section and code block")
				return
			}

			// Insert full help menu here
			updatedLines = append(updatedLines, newHelpMenuLines...)
			replaced = true
		}

		if inCodeBlock && bytes.Equal(line, []byte(editEndDelimiter)) {
			inCodeBlock = false
		}

		if !inCodeBlock {
			updatedLines = append(updatedLines, line)
		}
	}
	if inCodeBlock {
		err = fmt.Errorf("missing end code block delimiter for help menu in README, not updating README")
		return
	}

	newReadme := bytes.Join(updatedLines, []byte("\n"))
	err = os.WriteFile(readmePath, newReadme, 0600)
	if err != nil {
		err = fmt.Errorf("failed to write updated README file: %w", err)
		return
	}

	printSuccess(0, "Done")
	return
}
