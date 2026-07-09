package types

import "testing"

func gguf(title string) OrasBlob {
	return OrasBlob{MediaType: "application/octet-stream", Title: title}
}

func TestPlanModelRepackage_SingleFile(t *testing.T) {
	plans, err := PlanModelRepackage([]OrasBlob{gguf("llama3.2-Q4_K_M.gguf")}, "llama3.2", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 1 {
		t.Fatalf("want 1 plan, got %d", len(plans))
	}
	if plans[0].Entry != "model/llama3.2/llama3.2-Q4_K_M.gguf" {
		t.Fatalf("entry=%q", plans[0].Entry)
	}
}

func TestPlanModelRepackage_ShardsPreserveOrder(t *testing.T) {
	blobs := []OrasBlob{
		gguf("Qwen-Q8_K_XL-00001-of-00003.gguf"),
		gguf("Qwen-Q8_K_XL-00002-of-00003.gguf"),
		gguf("Qwen-Q8_K_XL-00003-of-00003.gguf"),
	}
	plans, err := PlanModelRepackage(blobs, "qwen", "")
	if err != nil {
		t.Fatalf("sharded same-quant should plan: %v", err)
	}
	if len(plans) != 3 {
		t.Fatalf("want 3, got %d", len(plans))
	}
	for i, p := range plans {
		if p.File != blobs[i].Title {
			t.Fatalf("order not preserved at %d: %q", i, p.File)
		}
	}
}

func TestPlanModelRepackage_SkipsNonModel(t *testing.T) {
	blobs := []OrasBlob{
		{MediaType: "application/vnd.oci.image.manifest.v1+json"}, // config-ish, skipped
		gguf("model-Q4_K_M.gguf"),
		{MediaType: "application/octet-stream", Title: "README.md"}, // skipped
	}
	plans, err := PlanModelRepackage(blobs, "m", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 1 || plans[0].File != "model-Q4_K_M.gguf" {
		t.Fatalf("expected only the gguf, got %+v", plans)
	}
}

func TestPlanModelRepackage_RejectsMixedQuant(t *testing.T) {
	blobs := []OrasBlob{
		gguf("m-Q4_K_M.gguf"),
		gguf("m-Q8_0.gguf"),
	}
	_, err := PlanModelRepackage(blobs, "m", "")
	if err == nil {
		t.Fatal("expected mixed-quant rejection")
	}
}

func TestPlanModelRepackage_NoModel(t *testing.T) {
	blobs := []OrasBlob{{MediaType: "application/octet-stream", Title: "notes.txt"}}
	if _, err := PlanModelRepackage(blobs, "m", ""); err == nil {
		t.Fatal("expected no-model error")
	}
}

func TestExtractQuant(t *testing.T) {
	cases := map[string]string{
		"Qwen-Q8_K_XL-00001-of-00004.gguf": "Q8_K_XL",
		"llama3.2-Q4_K_M.gguf":             "Q4_K_M",
		"phi-IQ3_XXS.gguf":                 "IQ3_XXS",
		"model-Q8_0.gguf":                  "Q8_0",
		"plain.gguf":                       "",
	}
	for in, want := range cases {
		if got := extractQuant(in); got != want {
			t.Errorf("extractQuant(%q)=%q want %q", in, got, want)
		}
	}
}
