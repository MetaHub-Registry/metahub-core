package neo

import (
	"context"
	"regexp"
	"strings"

	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

const pkgModelRuntime = "neo/model_runtime.go"

// runtimeNameSafe guards the runtime name inlined into the query.
var runtimeNameSafe = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)

// hasInjectionChars reports whether s contains characters that would break out of
// an inlined Cypher string literal (double quote or backslash).
func hasInjectionChars(s string) bool {
	return strings.ContainsAny(s, "\"\\")
}

// RuntimeUpsert registers (creates or updates) a runtime by name: the application
// container image it maps to and its coarse compatibility (formats + config
// contract). Idempotent on name. Fields are validated because they are inlined
// into the query.
func (n Neo) RuntimeUpsert(ctx context.Context, rt types.Runtime) (rtID int64, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgModelRuntime,
		"func":    "RuntimeUpsert",
		"name":    rt.Name,
	})
	defer span.End()
	_ = ctx

	if !runtimeNameSafe.MatchString(rt.Name) {
		return -1, utils.NewError(l, span, "unsafe runtime name %q", rt.Name)
	}
	formatsCSV := rt.FormatsCSV()
	if hasInjectionChars(rt.Image) || hasInjectionChars(formatsCSV) || hasInjectionChars(rt.ConfigContract) {
		return -1, utils.NewError(l, span, "runtime %q has invalid characters in image/formats/contract", rt.Name)
	}

	q := `MERGE (rt:Runtime {name: "` + rt.Name + `"})`
	q += ` SET rt.image = "` + rt.Image + `",`
	q += ` rt.formats = "` + formatsCSV + `",`
	q += ` rt.configContract = "` + rt.ConfigContract + `"`
	q += ` RETURN id(rt) AS rtID`
	utils.Debug(l, span, "%s", q)

	res, err := n.Execute(q)
	if err != nil {
		return -1, utils.ErrorHandler(l, span, err, "while executing RuntimeUpsert")
	}
	rtID, err = res.GetInt64("rtID")
	if err != nil {
		return -1, utils.ErrorHandler(l, span, err, "while reading rtID")
	}
	return rtID, nil
}

// RuntimesGetAll returns every registered runtime (name, image, formats, config
// contract) for composition matching and the runtimes endpoint.
func (n Neo) RuntimesGetAll(ctx context.Context) (runtimes []types.Runtime, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgModelRuntime,
		"func":    "RuntimesGetAll",
	})
	defer span.End()
	_ = ctx
	runtimes = make([]types.Runtime, 0)

	q := `MATCH (rt:Runtime)`
	q += ` RETURN COALESCE(rt.name, "") AS name,`
	q += ` COALESCE(rt.image, "") AS image,`
	q += ` COALESCE(rt.formats, "") AS formats,`
	q += ` COALESCE(rt.configContract, "") AS contract`
	q += ` ORDER BY name`
	utils.Debug(l, span, "%s", q)

	res, err := n.Query(q)
	if err != nil {
		return runtimes, utils.ErrorHandler(l, span, err, "while executing RuntimesGetAll")
	}
	for _, r := range res {
		name, _ := r.GetStr("name")
		if name == "" {
			continue
		}
		image, _ := r.GetStr("image")
		formats, _ := r.GetStr("formats")
		contract, _ := r.GetStr("contract")
		runtimes = append(runtimes, types.Runtime{
			Name:           name,
			Image:          image,
			Formats:        types.FormatsFromCSV(formats),
			ConfigContract: contract,
		})
	}
	return runtimes, nil
}
