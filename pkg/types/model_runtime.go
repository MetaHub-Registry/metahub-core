package types

import "strings"

// Runtime is a registered application container that can serve a model — the
// other half of a composition (runtime + resolved quant + resolved config). It is
// named by a config artefact's `runtime` annotation and carries coarse
// compatibility metadata: the model formats it can run and the config contract it
// honors. Resolving a config's runtime *name* to this record (with its image ref)
// is what turns a composition into a runnable bundle.
type Runtime struct {
	Name           string   `json:"name"`                      // the name a config's runtime annotation references
	Image          string   `json:"image"`                     // the application container image ref
	Formats        []string `json:"formats"`                   // model formats it can run (e.g. gguf)
	ConfigContract string   `json:"config_contract,omitempty"` // config contract it honors; empty = any
}

// RuntimeCompatible reports whether the runtime can serve a model of modelFormat
// with a config of configContract: the format must be in the runtime's formats
// (case-insensitive) and, unless the runtime's config-contract is empty (wildcard),
// it must equal the config's contract.
func RuntimeCompatible(rt Runtime, modelFormat, configContract string) bool {
	formatOK := modelFormat == ""
	for _, f := range rt.Formats {
		if strings.EqualFold(strings.TrimSpace(f), modelFormat) {
			formatOK = true
			break
		}
	}
	if !formatOK {
		return false
	}
	if rt.ConfigContract == "" {
		return true
	}
	return rt.ConfigContract == configContract
}

// FormatsCSV joins the runtime's formats for scalar storage in the graph.
func (rt Runtime) FormatsCSV() string {
	return strings.Join(rt.Formats, ",")
}

// FormatsFromCSV parses a comma-separated formats string back into a slice,
// dropping empties/whitespace.
func FormatsFromCSV(csv string) []string {
	out := make([]string, 0)
	for _, f := range strings.Split(csv, ",") {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}
