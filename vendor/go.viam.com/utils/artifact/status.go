package artifact

// Status describes how the tree has changed. We don't show removed because
// a removal is a direct operation done on the tree that is immediately reflected
// in source control.
type Status struct {
	Modified []string
	Unstored []string
}
