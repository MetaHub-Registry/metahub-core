package types

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// ModelFilePlan is one model file to be repackaged into its own layer, in ORAS
// blob order.
type ModelFilePlan struct {
	Blob  OrasBlob // source blob (streamed from the blobs bucket by Digest)
	File  string   // filename inside the layer (from the identifier)
	Entry string   // full tar entry path, relative (e.g. model/<name>/<file>)
}

// quantRe matches a GGUF quantization token such as Q4_K_M, Q8_0, IQ3_XXS,
// Q8_K_XL. Used to detect (and reject) a single artifact that mixes multiple
// quantizations — the MVP handles one quantization per push.
var quantRe = regexp.MustCompile(`(?i)\b(IQ|Q)\d+(_[A-Z0-9]+)*`)

// extractQuant returns the normalized (upper-case) quantization token found in a
// model filename, or "" when none is present. Shard files of the same model
// share the same token (e.g. "...-Q8_K_XL-00001-of-00004.gguf" → "Q8_K_XL").
func extractQuant(fileName string) string {
	m := quantRe.FindString(fileName)
	return strings.ToUpper(m)
}

// PlanModelRepackage inspects an artifact's blobs and returns the ordered set of
// model files to repackage, one layer each, preserving ORAS blob order. It is
// the pure decision core of `metahub-admin repackage` (no I/O).
//
// Errors:
//   - no blob identifies as a model,
//   - the artifact mixes more than one quantization (MVP is one quant per push).
func PlanModelRepackage(blobs []OrasBlob, name, targetPath string) ([]ModelFilePlan, error) {
	plans := make([]ModelFilePlan, 0, len(blobs))
	quants := map[string]struct{}{}
	for _, b := range blobs {
		file, ok := IdentifyModelBlob(b)
		if !ok {
			continue
		}
		if q := extractQuant(file); q != "" {
			quants[q] = struct{}{}
		}
		plans = append(plans, ModelFilePlan{
			Blob:  b,
			File:  file,
			Entry: ModelLayerTarget(targetPath, name, file),
		})
	}
	if len(plans) == 0 {
		return nil, fmt.Errorf("no model blobs found in artifact %q", name)
	}
	if len(quants) > 1 {
		return nil, fmt.Errorf("artifact %q mixes %d quantizations (%s); MVP handles one quantization per push",
			name, len(quants), strings.Join(sortedKeys(quants), ", "))
	}
	return plans, nil
}

// PlanConfigRepackage returns the config-file layers to inject for a model, one
// per configuration blob (identified by ModelConfigMediaType), placing each at
// model/<name>/config/<file>. Config is optional: an artifact with no config blob
// yields an empty plan (a model-only push). Pure decision core, no I/O.
func PlanConfigRepackage(blobs []OrasBlob, name string) []ModelFilePlan {
	plans := make([]ModelFilePlan, 0)
	for _, b := range blobs {
		file, ok := IdentifyConfigBlob(b)
		if !ok {
			continue
		}
		plans = append(plans, ModelFilePlan{
			Blob:  b,
			File:  file,
			Entry: ModelConfigTarget(name, file),
		})
	}
	return plans
}

func sortedKeys(m map[string]struct{}) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}
