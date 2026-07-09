package types

import "testing"

func TestIdentifyConfigBlob(t *testing.T) {
	// config mediaType + title → identified, title used as filename
	if f, ok := IdentifyConfigBlob(OrasBlob{MediaType: ModelConfigMediaType, Title: "serve.args"}); !ok || f != "serve.args" {
		t.Fatalf("titled config: got (%q, %v)", f, ok)
	}
	// config mediaType, no title → default filename
	if f, ok := IdentifyConfigBlob(OrasBlob{MediaType: ModelConfigMediaType}); !ok || f != "config.json" {
		t.Fatalf("default config: got (%q, %v)", f, ok)
	}
	// a model/other blob is never a config blob (identified only by mediaType)
	if _, ok := IdentifyConfigBlob(OrasBlob{MediaType: "application/octet-stream", Title: "x.gguf"}); ok {
		t.Fatal("octet-stream must not be identified as config")
	}
}

func TestModelConfigTarget(t *testing.T) {
	if got := ModelConfigTarget("qwen3-8b", "serve.args"); got != "model/qwen3-8b/config/serve.args" {
		t.Fatalf("target: %q", got)
	}
}

func TestPlanConfigRepackage_SplitsByMediaType(t *testing.T) {
	blobs := []OrasBlob{
		{Digest: "sha256:model", MediaType: "application/octet-stream", Title: "qwen3-8b-Q8_K_XL.gguf", Size: 100},
		{Digest: "sha256:cfg", MediaType: ModelConfigMediaType, Title: "serve.args", Size: 12},
	}

	cfgPlans := PlanConfigRepackage(blobs, "qwen3-8b")
	if len(cfgPlans) != 1 {
		t.Fatalf("want 1 config plan, got %d", len(cfgPlans))
	}
	if cfgPlans[0].File != "serve.args" || cfgPlans[0].Entry != "model/qwen3-8b/config/serve.args" || cfgPlans[0].Blob.Digest != "sha256:cfg" {
		t.Fatalf("config plan: %+v", cfgPlans[0])
	}

	// The model plan must NOT include the config blob (no contamination).
	mPlans, err := PlanModelRepackage(blobs, "qwen3-8b", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(mPlans) != 1 || mPlans[0].Blob.Digest != "sha256:model" {
		t.Fatalf("model plan must be the single GGUF blob, got %+v", mPlans)
	}

	// Model-only artifact → empty config plan, no error.
	if only := PlanConfigRepackage(blobs[:1], "qwen3-8b"); len(only) != 0 {
		t.Fatalf("want 0 config plans for a model-only artifact, got %d", len(only))
	}
}
