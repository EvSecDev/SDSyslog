package setup

type Step interface {
	Name() string

	// Non-mutating actions - reports if apply steps have already been done
	NeedsApply(ctx *context) (alreadyDone bool, err error)

	// Mutating-actions - conducts apply steps, records steps in self struct
	Apply(ctx *context) (err error)

	// Mutating-actions - conducts undo steps in case of apply failure using records in self struct
	Rollback(ctx *context)

	// Mutating-actions - conducts cleanup actions after all install steps have succeeded (will not trigger rollback when failures occur)
	PostApply(ctx *context)

	// Mutating-actions - conducts full removal of all related files/state
	Uninstall(ctx *context) (err error)
}

type context struct {
	mode    string  // send / receive
	dryRun  bool    // Enable/Disable mutating actions
	logger  *logger // Centralized print to stdout handler
	suiteID uint8   // Encryption suite identifier
}

type Installer struct {
	ctx   *context
	steps []Step // Ordered list of installation steps (reverse for uninstall)
}
