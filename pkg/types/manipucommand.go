package types

// ManipuCmdOp enumerates the valid operations for a ManipuCommand.
const (
	ManipuCmdOpReplace = "replace"
	ManipuCmdOpPrepend = "prepend"
	ManipuCmdOpAppend  = "append"
)

// ValidManipuCmdOps lists every accepted value of ManipuCommand.Op.
var ValidManipuCmdOps = []string{
	ManipuCmdOpReplace,
	ManipuCmdOpPrepend,
	ManipuCmdOpAppend,
}

// ManipuCommand represents a CMD or ENTRYPOINT mutation on an OCI image
// config. It is intentionally separate from the legacy `ManipuCmd` type
// which only carried a single-string value and a rule.
type ManipuCommand struct {
	ID    int64    `json:"id,omitempty"`
	Op    string   `json:"op"`
	Value []string `json:"value"`
}

// IsZero reports whether the command carries no data. Zero-valued commands
// are skipped by the parser and the graph writer.
func (c ManipuCommand) IsZero() bool {
	return c.Op == "" && len(c.Value) == 0
}

// IsValid reports whether Op is one of the allowed constants.
func (c ManipuCommand) IsValid() bool {
	for _, op := range ValidManipuCmdOps {
		if c.Op == op {
			return true
		}
	}
	return false
}
