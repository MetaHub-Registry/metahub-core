package neo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

const (
	pkgNameManipuConfig           = "neo/manipuconfig.go"
	FailManipuConfigCreate        = "failed to create ManipuConfig"
	FailManipuConfigGetByID       = "failed to get ManipuConfig by ID"
	FailManipuConfigFromResult    = "failed to build ManipuConfig from result"
	FailManipuConfigLink          = "failed to link Manipulation to ManipuConfig"
	FailManipuCommandCreate       = "failed to create ManipuCommand"
	FailManipuCommandFromResult   = "failed to build ManipuCommand from result"
	FailManipuConfigGetByManipID  = "failed to get ManipuConfig list for Manipulation"
	FailManipuConfigCmdLinkFailed = "failed to link ManipuConfig to ManipuCommand"
)

const (
	relHasDefault    = "HAS_DEFAULT"
	relHasVariant    = "HAS_VARIANT"
	relHasEntrypoint = "HAS_ENTRYPOINT"
	relHasCmd        = "HAS_CMD"
)

// manipuConfigCreate creates a `:ManipuConfig` node for the given config.
// All complex properties (env map, volumes list) are persisted as JSON
// strings so they round-trip through the project's generic Result helper
// which only understands scalars.
//
// Entrypoint/Cmd `ManipuCommand` children are persisted and linked via
// `HAS_ENTRYPOINT` / `HAS_CMD` edges when present. Layers are NOT created
// here — the caller is responsible for linking already-created ManipuLayer
// nodes via `(cfg)-[:HAS_LAYER {order}]->(:ManipuLayer)`.
func (n Neo) manipuConfigCreate(ctx context.Context, cfg *types.ManipuConfig) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipuConfig,
		"func":    "manipuConfigCreate",
	})
	defer span.End()

	envStr, e := marshalJSONField(cfg.Env)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "failed to marshal env")
	}
	volumesStr, e := marshalJSONField(cfg.Volumes)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "failed to marshal volumes")
	}

	q := fmt.Sprintf(
		`CREATE (cfg:ManipuConfig {env: %s, workdir: %s, user: %s, volumes: %s}) RETURN id(cfg) AS cfgID`,
		cypherQuote(envStr), cypherQuote(cfg.Workdir), cypherQuote(cfg.User), cypherQuote(volumesStr),
	)
	utils.Debug(l, span, q)
	res, e := n.Execute(q)
	if e != nil {
		return utils.ErrorHandler(l, span, e, FailNeoExecute)
	}
	cfgID, e := res.GetInt64("cfgID")
	if e != nil {
		return utils.ErrorHandler(l, span, e, types.FailBrGet, "cfgID")
	}
	cfg.ID = cfgID

	// Commands.
	if cfg.Entrypoint != nil && !cfg.Entrypoint.IsZero() {
		if e := n.manipuCommandCreateAndLink(ctx, cfgID, relHasEntrypoint, cfg.Entrypoint); e != nil {
			return utils.ErrorHandler(l, span, e, FailManipuCommandCreate)
		}
	}
	if cfg.Cmd != nil && !cfg.Cmd.IsZero() {
		if e := n.manipuCommandCreateAndLink(ctx, cfgID, relHasCmd, cfg.Cmd); e != nil {
			return utils.ErrorHandler(l, span, e, FailManipuCommandCreate)
		}
	}
	return nil
}

// manipuCommandCreateAndLink creates a `:ManipuCommand` node and links it
// to the parent `ManipuConfig` via the given relationship.
func (n Neo) manipuCommandCreateAndLink(ctx context.Context, cfgID int64, rel string, cmd *types.ManipuCommand) error {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipuConfig,
		"func":    "manipuCommandCreateAndLink",
	})
	defer span.End()
	_ = ctx
	valuesStr, e := marshalJSONField(cmd.Value)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "failed to marshal command value")
	}
	q := fmt.Sprintf(
		`MATCH (cfg:ManipuConfig) WHERE id(cfg) = %d CREATE (mc:ManipuCommand {op: %s, value: %s}) CREATE (cfg)-[:%s]->(mc) RETURN id(mc) AS mcID`,
		cfgID, cypherQuote(cmd.Op), cypherQuote(valuesStr), rel,
	)
	utils.Debug(l, span, q)
	res, err := n.Execute(q)
	if err != nil {
		return utils.ErrorHandler(l, span, err, FailNeoExecute)
	}
	mcID, err := res.GetInt64("mcID")
	if err != nil {
		return utils.ErrorHandler(l, span, err, types.FailBrGet, "mcID")
	}
	cmd.ID = mcID
	return nil
}

// manipuConfigLink attaches a `ManipuConfig` node to its owning
// `Manipulation` node using either `HAS_DEFAULT` (for the default block)
// or `HAS_VARIANT {rule}` (for a rule-keyed variant).
func (n Neo) manipuConfigLink(ctx context.Context, mpID, cfgID int64, rel string, ruleKey string) error {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipuConfig,
		"func":    "manipuConfigLink",
	})
	defer span.End()
	_ = ctx
	props := ""
	if rel == relHasVariant {
		props = fmt.Sprintf(` {rule: %s}`, cypherQuote(ruleKey))
	}
	q := fmt.Sprintf(
		`MATCH (mp:Manipulation), (cfg:ManipuConfig) WHERE id(mp) = %d AND id(cfg) = %d MERGE (mp)-[:%s%s]->(cfg)`,
		mpID, cfgID, rel, props,
	)
	utils.Debug(l, span, q)
	if _, err := n.Execute(q); err != nil {
		return utils.ErrorHandler(l, span, err, FailNeoExecute)
	}
	return nil
}

// manipuLayerCreateForConfig creates a bare `:ManipuLayer` node linked to
// its Upstream + Manifest but *not* to a Rule. Variant layers inherit
// their rule from the `HAS_VARIANT {rule}` edge on the owning
// `ManipuConfig`, so there is no per-layer Rule linkage to establish.
//
// The returned layer has its `ID` populated so the caller can link it
// via `manipuConfigLinkLayer`.
func (n Neo) manipuLayerCreateForConfig(ctx context.Context, up types.Upstream) (types.ManipuLayer, error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipuConfig,
		"func":    "manipuLayerCreateForConfig",
		"image":   up.Reg + "/" + up.Repo + ":" + up.Tag,
	})
	defer span.End()

	// Resolve the existing Upstream to its ID.
	upID, found, err := n.UpstreamGet(ctx, up)
	if err != nil {
		return types.ManipuLayer{}, utils.ErrorHandler(l, span, err, FailUpstreamGet)
	}
	if !found {
		return types.ManipuLayer{}, utils.NewError(l, span, FailUpstreamNotFound)
	}
	up.ID = upID
	md, found, err := n.ManifestGetByUpstream(ctx, up)
	if err != nil {
		return types.ManipuLayer{}, utils.ErrorHandler(l, span, err, FailManifestGetByUpstream)
	}
	if !found {
		return types.ManipuLayer{}, utils.NewError(l, span, FailManifestNotFound)
	}

	label := up.GetLabel()
	// ManipuLayer uniqueness is (label, rule). Variant layers use empty
	// rule because the rule lives on the variant edge.
	if existing, found, gErr := n.ManipuLayerGetByLabel(ctx, label); gErr == nil && found {
		return existing, nil
	}
	q := fmt.Sprintf(
		`CREATE (mpl:ManipuLayer {label: %s, rule: ""}) RETURN id(mpl) AS mplID`,
		cypherQuote(label),
	)
	utils.Debug(l, span, q)
	res, err := n.Execute(q)
	if err != nil {
		return types.ManipuLayer{}, utils.ErrorHandler(l, span, err, FailNeoExecute)
	}
	mplID, err := res.GetInt64("mplID")
	if err != nil {
		return types.ManipuLayer{}, utils.ErrorHandler(l, span, err, types.FailBrGet, "mplID")
	}
	// Link to Manifest + Upstream.
	if err := n.ManifestLinkToManipulayer(ctx, md.ID, mplID); err != nil {
		return types.ManipuLayer{}, utils.ErrorHandler(l, span, err, FailManipuLayerLinkToManifest)
	}
	if err := n.ManipuLayerLinkToUpstream(ctx, mplID, upID); err != nil {
		return types.ManipuLayer{}, utils.ErrorHandler(l, span, err, FailManipuLayerLinkToUpstream)
	}
	return types.ManipuLayer{
		ID:    mplID,
		Label: label,
		Image: md,
	}, nil
}

// manipuConfigLinkLayer links a ManipuLayer to a ManipuConfig while
// preserving the layer's order within the config's layer list.
func (n Neo) manipuConfigLinkLayer(ctx context.Context, cfgID, mplID int64, order int) error {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipuConfig,
		"func":    "manipuConfigLinkLayer",
	})
	defer span.End()
	_ = ctx
	q := fmt.Sprintf(
		`MATCH (cfg:ManipuConfig), (mpl:ManipuLayer) WHERE id(cfg) = %d AND id(mpl) = %d MERGE (cfg)-[:HAS_LAYER {order: %d}]->(mpl)`,
		cfgID, mplID, order,
	)
	utils.Debug(l, span, q)
	if _, err := n.Execute(q); err != nil {
		return utils.ErrorHandler(l, span, err, FailNeoExecute)
	}
	return nil
}

// manipuConfigsByManipulationID returns every ManipuConfig linked from the
// given Manipulation node, distinguishing HAS_DEFAULT vs HAS_VARIANT and
// carrying the variant rule when present.
type configLink struct {
	CfgID int64
	Rel   string
	Rule  string
}

func (n Neo) manipuConfigsByManipulationID(ctx context.Context, mpID int64) (links []configLink, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipuConfig,
		"func":    "manipuConfigsByManipulationID",
		"mpID":    mpID,
	})
	defer span.End()
	_ = ctx
	q := fmt.Sprintf(
		`MATCH (mp:Manipulation)-[r]->(cfg:ManipuConfig) WHERE id(mp) = %d AND type(r) IN ["HAS_DEFAULT","HAS_VARIANT"] RETURN id(cfg) AS cfgID, type(r) AS rel, coalesce(r.rule, "") AS rule`,
		mpID,
	)
	utils.Debug(l, span, q)
	res, err := n.Query(q)
	if err != nil {
		return links, utils.ErrorHandler(l, span, err, FailNeoExecute)
	}
	for _, r := range res {
		id, _ := r.GetInt64("cfgID")
		rel, _ := r.GetStr("rel")
		rule, _ := r.GetStr("rule")
		links = append(links, configLink{CfgID: id, Rel: rel, Rule: rule})
	}
	return links, nil
}

// manipuConfigHydrate loads a ManipuConfig node (properties + commands +
// layers) from the graph.
func (n Neo) manipuConfigHydrate(ctx context.Context, cfgID int64) (cfg types.ManipuConfig, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipuConfig,
		"func":    "manipuConfigHydrate",
		"cfgID":   cfgID,
	})
	defer span.End()
	found, err := n.manipuConfigHydrateProps(ctx, cfgID, &cfg)
	if err != nil || !found {
		return cfg, err
	}
	if err = n.manipuConfigHydrateCommands(ctx, cfgID, &cfg); err != nil {
		return cfg, utils.ErrorHandler(l, span, err, FailManipuCommandFromResult)
	}
	layers, err := n.manipuConfigLayers(ctx, cfgID)
	if err != nil {
		return cfg, utils.ErrorHandler(l, span, err, FailManipuLayersGetByManipulationID)
	}
	cfg.Layers = layers
	return cfg, nil
}

// manipuConfigHydrateProps loads the scalar properties of a ManipuConfig
// (ID, env, workdir, user, volumes) into cfg. Returns found=false when the
// node does not exist.
func (n Neo) manipuConfigHydrateProps(ctx context.Context, cfgID int64, cfg *types.ManipuConfig) (bool, error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipuConfig,
		"func":    "manipuConfigHydrateProps",
		"cfgID":   cfgID,
	})
	defer span.End()
	_ = ctx
	q := fmt.Sprintf(
		`MATCH (cfg:ManipuConfig) WHERE id(cfg) = %d RETURN id(cfg) AS cfgID, coalesce(cfg.env, "") AS env, coalesce(cfg.workdir, "") AS workdir, coalesce(cfg.user, "") AS user, coalesce(cfg.volumes, "") AS volumes`,
		cfgID,
	)
	utils.Debug(l, span, q)
	res, found, err := n.QueryOne(q)
	if err != nil {
		return false, utils.ErrorHandler(l, span, err, FailNeoExecute)
	}
	if !found {
		return false, nil
	}
	cfg.ID, _ = res.GetInt64("cfgID")
	cfg.Workdir, _ = res.GetStr("workdir")
	cfg.User, _ = res.GetStr("user")
	envStr, _ := res.GetStr("env")
	if err := unmarshalOptionalJSON(envStr, &cfg.Env); err != nil {
		return true, utils.ErrorHandler(l, span, err, "failed to unmarshal env")
	}
	volumesStr, _ := res.GetStr("volumes")
	if err := unmarshalOptionalJSON(volumesStr, &cfg.Volumes); err != nil {
		return true, utils.ErrorHandler(l, span, err, "failed to unmarshal volumes")
	}
	return true, nil
}

// manipuConfigHydrateCommands populates cfg.Entrypoint and cfg.Cmd from
// linked `ManipuCommand` nodes when present.
func (n Neo) manipuConfigHydrateCommands(ctx context.Context, cfgID int64, cfg *types.ManipuConfig) error {
	ep, err := n.manipuCommandByRel(ctx, cfgID, relHasEntrypoint)
	if err != nil {
		return err
	}
	cfg.Entrypoint = ep
	cmd, err := n.manipuCommandByRel(ctx, cfgID, relHasCmd)
	if err != nil {
		return err
	}
	cfg.Cmd = cmd
	return nil
}

// unmarshalOptionalJSON decodes s into out. Empty / "null" strings are
// treated as absent and leave out untouched.
func unmarshalOptionalJSON(s string, out any) error {
	if s == "" || s == "null" {
		return nil
	}
	return json.Unmarshal([]byte(s), out)
}

func (n Neo) manipuCommandByRel(ctx context.Context, cfgID int64, rel string) (*types.ManipuCommand, error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipuConfig,
		"func":    "manipuCommandByRel",
		"cfgID":   cfgID,
		"rel":     rel,
	})
	defer span.End()
	_ = ctx
	q := fmt.Sprintf(
		`MATCH (cfg:ManipuConfig)-[:%s]->(mc:ManipuCommand) WHERE id(cfg) = %d RETURN id(mc) AS mcID, coalesce(mc.op, "") AS op, coalesce(mc.value, "") AS value LIMIT 1`,
		rel, cfgID,
	)
	utils.Debug(l, span, q)
	res, found, err := n.QueryOne(q)
	if err != nil {
		return nil, utils.ErrorHandler(l, span, err, FailNeoExecute)
	}
	if !found {
		return nil, nil
	}
	id, _ := res.GetInt64("mcID")
	op, _ := res.GetStr("op")
	valueStr, _ := res.GetStr("value")
	var value []string
	if valueStr != "" && valueStr != "null" {
		if e := json.Unmarshal([]byte(valueStr), &value); e != nil {
			return nil, utils.ErrorHandler(l, span, e, "failed to unmarshal command value")
		}
	}
	cmd := &types.ManipuCommand{ID: id, Op: op, Value: value}
	return cmd, nil
}

func (n Neo) manipuConfigLayers(ctx context.Context, cfgID int64) ([]types.ManipuLayer, error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipuConfig,
		"func":    "manipuConfigLayers",
		"cfgID":   cfgID,
	})
	defer span.End()
	q := fmt.Sprintf(
		`MATCH (cfg:ManipuConfig)-[r:HAS_LAYER]->(mpl:ManipuLayer) WHERE id(cfg) = %d RETURN %s, coalesce(r.order, 0) AS layerOrder ORDER BY layerOrder`,
		cfgID, ManipuLayerReturn,
	)
	utils.Debug(l, span, q)
	res, err := n.Query(q)
	if err != nil {
		return nil, utils.ErrorHandler(l, span, err, FailNeoExecute)
	}
	layers := make([]types.ManipuLayer, 0, len(res))
	for _, r := range res {
		mpl, e := n.ManipuLayerFromResult(ctx, r)
		if e != nil {
			return nil, utils.ErrorHandler(l, span, e, FailManipuLayerFromResult)
		}
		// Attach the ManipuLayer's Upstream onto its Image so the UI
		// can render the source image URL without a separate lookup.
		if up, found, uErr := n.UpstreamGetByManipuLayerID(ctx, mpl.ID); uErr == nil && found {
			mpl.Image.Upstream = up
		}
		layers = append(layers, mpl)
	}
	return layers, nil
}

// cypherQuote wraps a string in double quotes and escapes double quotes +
// backslashes, matching the other string-literal uses in this package. It
// intentionally produces Cypher-literal syntax rather than using
// parameterised queries because the rest of the codebase builds queries
// that way.
func cypherQuote(s string) string {
	escaped := strings.ReplaceAll(s, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `"` + escaped + `"`
}

// marshalJSONField marshals any value to a JSON string; nil/empty values
// produce "".
func marshalJSONField(v any) (string, error) {
	// Treat nil and empty collections as empty-string for Cypher friendliness.
	switch x := v.(type) {
	case nil:
		return "", nil
	case map[string]string:
		if len(x) == 0 {
			return "", nil
		}
	case []string:
		if len(x) == 0 {
			return "", nil
		}
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
