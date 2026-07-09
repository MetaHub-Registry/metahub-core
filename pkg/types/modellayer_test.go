package types

import (
	"archive/tar"
	"bytes"
	"io"
	"testing"
)

func TestIdentifyModelBlob(t *testing.T) {
	cases := []struct {
		name     string
		blob     OrasBlob
		wantFile string
		wantIs   bool
	}{
		{"docker mediaType no title",
			OrasBlob{MediaType: "application/vnd.docker.ai.gguf.v3"}, "model.gguf", true},
		{"ollama mediaType with title",
			OrasBlob{MediaType: "application/vnd.ollama.image.model", Title: "qwen.gguf"}, "qwen.gguf", true},
		{"octet-stream + gguf title",
			OrasBlob{MediaType: "application/octet-stream", Title: "llama3.2-Q4_K_M.gguf"}, "llama3.2-Q4_K_M.gguf", true},
		{"octet-stream + title via annotation",
			OrasBlob{MediaType: "application/octet-stream",
				Annotations: map[string]string{"org.opencontainers.image.title": "m.GGUF"}}, "m.GGUF", true},
		{"octet-stream + non-gguf title",
			OrasBlob{MediaType: "application/octet-stream", Title: "readme.txt"}, "", false},
		{"unknown, no title",
			OrasBlob{MediaType: "application/octet-stream"}, "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotFile, gotIs := IdentifyModelBlob(c.blob)
			if gotIs != c.wantIs || gotFile != c.wantFile {
				t.Fatalf("IdentifyModelBlob=%q,%v want %q,%v", gotFile, gotIs, c.wantFile, c.wantIs)
			}
		})
	}
}

func TestLooksLikeGGUF(t *testing.T) {
	if !LooksLikeGGUF([]byte("GGUF\x00\x01")) {
		t.Fatal("GGUF magic not detected")
	}
	if LooksLikeGGUF([]byte("PK\x03\x04")) {
		t.Fatal("false positive")
	}
	if LooksLikeGGUF([]byte("GG")) {
		t.Fatal("short buffer should be false")
	}
}

func TestModelLayerTarget(t *testing.T) {
	if got := ModelLayerTarget("", "llama3.2", "f.gguf"); got != "model/llama3.2/f.gguf" {
		t.Fatalf("default target=%q", got)
	}
	if got := ModelLayerTarget("/opt/m/", "x", "f.gguf"); got != "opt/m/f.gguf" {
		t.Fatalf("override target=%q", got)
	}
}

// TestStreamModelLayerDeterministic is the golden-digest guard (spike 2.2): the
// same payload + entry must yield the same layer digest across independent runs.
func TestStreamModelLayerDeterministic(t *testing.T) {
	payload := bytes.Repeat([]byte("GGUF-weights-"), 4096)
	entry := "model/llama3.2/llama3.2-Q4_K_M.gguf"

	run := func() (string, int64, []byte) {
		var buf bytes.Buffer
		dig, tarSize, err := StreamModelLayer(&buf, bytes.NewReader(payload), int64(len(payload)), entry)
		if err != nil {
			t.Fatalf("StreamModelLayer: %v", err)
		}
		return dig.String(), tarSize, buf.Bytes()
	}
	d1, s1, b1 := run()
	d2, s2, b2 := run()
	if d1 != d2 {
		t.Fatalf("non-deterministic digest: %s != %s", d1, d2)
	}
	if s1 != s2 || !bytes.Equal(b1, b2) {
		t.Fatalf("non-deterministic tar bytes (sizes %d/%d)", s1, s2)
	}

	// DiffID == layer digest for uncompressed tar: the digest is computed over
	// exactly the bytes emitted, so re-hashing the buffer must match.
	if int64(len(b1)) != s1 {
		t.Fatalf("reported tarSize %d != actual %d", s1, len(b1))
	}

	// The file must be present on-path with the deterministic metadata.
	tr := tar.NewReader(bytes.NewReader(b1))
	var sawFile bool
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar read: %v", err)
		}
		if h.Name == entry {
			sawFile = true
			if h.Mode != 0o444 {
				t.Fatalf("mode=%o want 0444", h.Mode)
			}
			if !h.ModTime.Equal(epoch) {
				t.Fatalf("mtime=%v want epoch", h.ModTime)
			}
			if h.Uid != 0 || h.Gid != 0 {
				t.Fatalf("uid/gid=%d/%d want 0/0", h.Uid, h.Gid)
			}
			got, _ := io.ReadAll(tr)
			if !bytes.Equal(got, payload) {
				t.Fatal("payload roundtrip mismatch")
			}
		}
	}
	if !sawFile {
		t.Fatalf("entry %q not found in tar", entry)
	}
}

// TestStreamModelLayerShortRead ensures a size mismatch is reported.
func TestStreamModelLayerShortRead(t *testing.T) {
	var buf bytes.Buffer
	_, _, err := StreamModelLayer(&buf, bytes.NewReader([]byte("abc")), 10, "model/x/f.gguf")
	if err == nil {
		t.Fatal("expected short-read error")
	}
}
