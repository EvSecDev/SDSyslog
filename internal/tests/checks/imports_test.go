package checks

import (
	"sdsyslog/internal/global"
	"slices"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

// Validate that certain internal packages are only imported by a set of allowed internal packages
func TestImportRestrictions(t *testing.T) {
	type rule struct {
		targetPrefix    string   // applies recursively
		allowedToImport []string // exact or prefix matches
	}

	// Restrict imports of certain packages
	rules := []rule{
		// Algorithm-specific crypto functions should only be used from the registry
		{
			targetPrefix:    global.ProgBaseName + "/internal/crypto/ecdh",
			allowedToImport: []string{global.ProgBaseName + "/pkg/crypto/registry"},
		},
		{
			targetPrefix:    global.ProgBaseName + "/internal/crypto/aead",
			allowedToImport: []string{global.ProgBaseName + "/pkg/crypto/registry"},
		},
		// Only main should call the CLI package
		{
			targetPrefix:    global.ProgBaseName + "/internal/cli",
			allowedToImport: []string{global.ProgBaseName + "/cmd/sdsyslog"},
		},
		// No internal logic should be calling the installation logic
		{
			targetPrefix:    global.ProgBaseName + "/internal/install",
			allowedToImport: []string{global.ProgBaseName + "/internal/cli"},
		},
		// IOModules are meant for pipeline and integ tests only
		{
			targetPrefix: global.ProgBaseName + "/internal/iomodules",
			allowedToImport: []string{
				global.ProgBaseName + "/internal/receiver",
				global.ProgBaseName + "/internal/sender",
				global.ProgBaseName + "/tests/integration",
				global.ProgBaseName + "/internal/install",
				global.ProgBaseName + "/cmd/sdsyslog", // Specifically for import-only init call
			},
		},
	}

	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedImports |
			packages.NeedModule,
	}

	packages, err := packages.Load(cfg, global.ProgBaseName+"/...")
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	for _, pkg := range packages {
		if strings.HasSuffix(pkg.PkgPath, "_test") {
			continue
		}

		for importPath := range pkg.Imports {
			for _, rule := range rules {
				if importPath != rule.targetPrefix && !strings.HasPrefix(importPath, rule.targetPrefix+"/") {
					continue
				}

				if (pkg.PkgPath == rule.targetPrefix || strings.HasPrefix(pkg.PkgPath, rule.targetPrefix+"/")) &&
					(importPath == rule.targetPrefix || strings.HasPrefix(importPath, rule.targetPrefix+"/")) {
					continue
				}

				allowed := false

				for _, allowedImportPath := range rule.allowedToImport {
					if pkg.PkgPath == allowedImportPath ||
						strings.HasPrefix(pkg.PkgPath, allowedImportPath+"/") {
						allowed = true
						break
					}
				}

				if !allowed {
					t.Errorf(
						"%s (and subpackages) may only be imported by %v (used in %s)",
						rule.targetPrefix,
						rule.allowedToImport,
						pkg.PkgPath,
					)
				}
			}
		}
	}
}

// Check internal packages are not imported by any other packages
func TestNoImports(t *testing.T) {
	// Exact package paths with no internal imports
	assertNotImported := []string{
		global.ProgBaseName + "/internal/parsing",
		global.ProgBaseName + "/internal/logctx",
		global.ProgBaseName + "/internal/filtering",
		global.ProgBaseName + "/internal/atomics",
		global.ProgBaseName + "/internal/calc",
	}

	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedImports |
			packages.NeedModule,
	}

	packages, err := packages.Load(cfg, global.ProgBaseName+"/...")
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	for _, pkg := range packages {
		if strings.HasSuffix(pkg.PkgPath, "_test") {
			continue
		}

		if !slices.Contains(assertNotImported, pkg.PkgPath) {
			continue
		}

		for importPath := range pkg.Imports {
			if !strings.HasPrefix(importPath, global.ProgBaseName+"/") {
				continue
			}

			t.Errorf("%s should not import any other internal packages (importing %s)",
				pkg.PkgPath, importPath)
		}
	}
}
