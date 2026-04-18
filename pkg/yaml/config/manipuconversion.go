package config

import "github.com/metahub-registry/metahub-core/pkg/types"

// ToTypesManipuCommand converts a config ManipuCommand into its types
// equivalent. Returns nil when the input is nil or zero-valued.
func (c *ManipuCommand) ToTypesManipuCommand() *types.ManipuCommand {
	if c == nil || c.IsZero() {
		return nil
	}
	v := make([]string, len(c.Value))
	copy(v, c.Value)
	return &types.ManipuCommand{
		Op:    c.Op,
		Value: v,
	}
}

// ToTypesManipuConfig converts a config ManipuConfig into its types
// equivalent. Zero values produce a zero types.ManipuConfig.
func (c ManipuConfig) ToTypesManipuConfig() types.ManipuConfig {
	// Deep-copy the map and slices so the caller can mutate freely.
	var env map[string]string
	if len(c.Env) != 0 {
		env = make(map[string]string, len(c.Env))
		for k, v := range c.Env {
			env[k] = v
		}
	}
	var volumes []string
	if len(c.Volumes) != 0 {
		volumes = make([]string, len(c.Volumes))
		copy(volumes, c.Volumes)
	}
	var layers []types.ManipuLayer
	if len(c.Layers) != 0 {
		layers = make([]types.ManipuLayer, 0, len(c.Layers))
		for _, lyr := range c.Layers {
			layers = append(layers, types.ManipuLayer{
				Rule: lyr.Rule,
				// Image lives on the Upstream; caller resolves via NewUpstreamByName.
			})
			// The caller is responsible for populating the remaining ManipuLayer
			// fields (Image upstream, Cmd, Env). We only copy the rule and leave
			// a placeholder so order is preserved.
		}
	}
	return types.ManipuConfig{
		Env:        env,
		Workdir:    c.Workdir,
		User:       c.User,
		Volumes:    volumes,
		Entrypoint: c.Entrypoint.ToTypesManipuCommand(),
		Cmd:        c.Cmd.ToTypesManipuCommand(),
		Layers:     layers,
	}
}
