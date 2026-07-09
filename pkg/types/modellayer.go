package types

import (
	"archive/tar"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/opencontainers/go-digest"
)

// ModelLayerMediaType is the uncompressed OCI layer mediaType used for
// synthesised model layers. Uncompressed ⇒ the layer digest equals the DiffID,
// which keeps the manipulated image config's RootFS.DiffIDs trivially in sync.
const ModelLayerMediaType = "application/vnd.oci.image.layer.v1.tar"

// ModelConfigMediaType is the dedicated mediaType for a runtime configuration
// blob the publisher pushes alongside the model in the same ORAS artifact.
// repackage identifies it by this mediaType (the mirror of IdentifyModelBlob) and
// injects it as a per-profile config layer. Its bytes are opaque to metahub.
const ModelConfigMediaType = "application/vnd.qnib.model.config.v1+json"

// knownModelMediaTypes maps model-artifact blob mediaTypes to a canonical file
// extension. Highest-precedence identification signal (a GGUF pushed via
// `docker model push` / Ollama carries these; a plain `oras push` does not).
var knownModelMediaTypes = map[string]string{
	"application/vnd.docker.ai.gguf.v3":  ".gguf",
	"application/vnd.ollama.image.model": ".gguf",
}

// blobTitle returns the org.opencontainers.image.title for a blob, preferring
// the dedicated Title field and falling back to the annotation.
func blobTitle(b OrasBlob) string {
	if b.Title != "" {
		return b.Title
	}
	return b.Annotations["org.opencontainers.image.title"]
}

// IdentifyModelBlob decides whether an ORAS blob is a model file and returns the
// filename to place inside the layer. Precedence (mirrors the design):
//  1. known model mediaType,
//  2. org.opencontainers.image.title with a .gguf extension.
//
// The magic-byte fallback (LooksLikeGGUF) is separate because it needs the blob
// bytes, which the caller streams. Returns isModel=false when nothing matches.
func IdentifyModelBlob(b OrasBlob) (fileName string, isModel bool) {
	title := blobTitle(b)
	if ext, ok := knownModelMediaTypes[b.MediaType]; ok {
		if title != "" {
			return title, true
		}
		return "model" + ext, true
	}
	if title != "" && strings.EqualFold(path.Ext(title), ".gguf") {
		return title, true
	}
	return "", false
}

// IdentifyConfigBlob decides whether an ORAS blob is the runtime configuration
// blob — identified solely by the dedicated ModelConfigMediaType, which the
// publisher tags explicitly (the mirror of IdentifyModelBlob) — and returns the
// filename to place inside the config layer (the org.opencontainers.image.title,
// defaulting to "config.json"). Returns isConfig=false for any other blob.
func IdentifyConfigBlob(b OrasBlob) (fileName string, isConfig bool) {
	if b.MediaType != ModelConfigMediaType {
		return "", false
	}
	if title := blobTitle(b); title != "" {
		return title, true
	}
	return "config.json", true
}

// LooksLikeGGUF is the magic-byte fallback: GGUF files begin with "GGUF".
func LooksLikeGGUF(peek []byte) bool {
	return len(peek) >= 4 && string(peek[:4]) == "GGUF"
}

// ModelLayerTarget returns the tar entry path (relative, no leading slash) for a
// model file: <targetPath>/<fileName>, defaulting targetPath to /model/<name>/.
func ModelLayerTarget(targetPath, name, fileName string) string {
	if targetPath == "" {
		targetPath = "/model/" + name + "/"
	}
	return path.Join(strings.TrimPrefix(targetPath, "/"), fileName)
}

// ModelConfigTarget returns the tar entry path (relative) for a config file:
// model/<name>/config/<fileName> — the config analogue of ModelLayerTarget,
// co-located under the model dir so a base image can hard-code the path.
func ModelConfigTarget(name, fileName string) string {
	return path.Join("model", name, "config", fileName)
}

// epoch is the pinned timestamp for every tar entry (determinism).
var epoch = time.Unix(0, 0).UTC()

// countWriter counts bytes written through it.
type countWriter struct {
	w io.Writer
	n int64
}

func (c *countWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}

// StreamModelLayer wraps src (exactly size bytes) into a deterministic
// uncompressed tar placing the file at entry (relative path, e.g.
// "model/llama3.2/llama3.2.gguf"), writes the tar to dst, and returns the layer
// digest (sha256 over the tar bytes == DiffID) and the tar size.
//
// It streams with a fixed 1 MiB buffer and never buffers the whole blob, so peak
// memory is independent of model size (validated by spike 2.3). Determinism:
// fixed mtime/uid/gid/mode, PAX format, stable parent-dir + file order.
func StreamModelLayer(dst io.Writer, src io.Reader, size int64, entry string) (dig digest.Digest, tarSize int64, err error) {
	digester := digest.SHA256.Digester()
	cw := &countWriter{w: dst}
	mw := io.MultiWriter(cw, digester.Hash())
	tw := tar.NewWriter(mw)

	// Emit parent directory entries top-down for robustness.
	for _, d := range parentDirs(entry) {
		if e := tw.WriteHeader(&tar.Header{
			Name: d + "/", Typeflag: tar.TypeDir, Mode: 0o755,
			ModTime: epoch, Format: tar.FormatPAX,
		}); e != nil {
			return "", 0, e
		}
	}
	if e := tw.WriteHeader(&tar.Header{
		Name: entry, Typeflag: tar.TypeReg, Mode: 0o444,
		Size: size, ModTime: epoch, Format: tar.FormatPAX,
	}); e != nil {
		return "", 0, e
	}
	buf := make([]byte, 1<<20)
	written, err := io.CopyBuffer(tw, io.LimitReader(src, size), buf)
	if err != nil {
		return "", 0, err
	}
	if written != size {
		return "", 0, fmt.Errorf("short read: got %d bytes, want %d", written, size)
	}
	if err = tw.Close(); err != nil {
		return "", 0, err
	}
	return digester.Digest(), cw.n, nil
}

// parentDirs returns the ancestor directories of a relative file path, top-down
// (e.g. "model/llama3.2/f.gguf" → ["model", "model/llama3.2"]).
func parentDirs(entry string) []string {
	var dirs []string
	for d := path.Dir(entry); d != "." && d != "/" && d != ""; d = path.Dir(d) {
		dirs = append([]string{d}, dirs...)
	}
	return dirs
}
