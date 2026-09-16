package magazine

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildSymlinkTar creates a (optionally gzip-compressed) tar with the alpine
// >=3.21 symlink structure:
//
//	./etc/os-release  → /usr/lib/os-release (symlink)
//	./usr/lib/os-release (regular file with os-release content)
func buildSymlinkTar(t *testing.T, withGzip bool) []byte {
	t.Helper()
	content := "ID=alpine\nVERSION_ID=3.21.0\nNAME=\"Alpine Linux\"\nPRETTY_NAME=\"Alpine Linux v3.21\"\n"

	var buf bytes.Buffer
	var tw *tar.Writer
	var gw *gzip.Writer
	if withGzip {
		gw = gzip.NewWriter(&buf)
		tw = tar.NewWriter(gw)
	} else {
		tw = tar.NewWriter(&buf)
	}

	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     "etc/os-release",
		Typeflag: tar.TypeSymlink,
		Linkname: "/usr/lib/os-release",
	}))
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     "usr/lib/os-release",
		Typeflag: tar.TypeReg,
		Mode:     0644,
		Size:     int64(len(content)),
	}))
	_, err := tw.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	if gw != nil {
		require.NoError(t, gw.Close())
	}
	return buf.Bytes()
}

// TestScanGzipTarLinks_SymlinkPlainTar verifies symlink detection in a plain
// tar stream (the format used by test fixtures).
func TestScanGzipTarLinks_SymlinkPlainTar(t *testing.T) {
	buf := buildSymlinkTar(t, false)
	links, err := scanGzipTarLinks(bytes.NewReader(buf), "etc/os-release")
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, "usr/lib/os-release", links["etc/os-release"],
		"absolute symlink target must be stripped of leading /")
}

// TestScanGzipTarLinks_SymlinkGzipTar verifies symlink detection in a
// gzip-compressed tar stream (the format used by real OCI/Docker layers).
func TestScanGzipTarLinks_SymlinkGzipTar(t *testing.T) {
	buf := buildSymlinkTar(t, true)
	links, err := scanGzipTarLinks(bytes.NewReader(buf), "etc/os-release")
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, "usr/lib/os-release", links["etc/os-release"])
}

// TestScanGzipTarLinks_NoMatch verifies that no entries are returned when the
// layer contains no symlink for the requested name.
func TestScanGzipTarLinks_NoMatch(t *testing.T) {
	// Build a tar with only a regular file — no symlinks.
	content := "ID=alpine\nVERSION_ID=3.17.3\n"
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     "etc/os-release",
		Typeflag: tar.TypeReg,
		Mode:     0644,
		Size:     int64(len(content)),
	}))
	_, err := tw.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, tw.Close())

	links, err := scanGzipTarLinks(bytes.NewReader(buf.Bytes()), "etc/os-release")
	require.NoError(t, err)
	assert.Empty(t, links, "regular file must not appear as a symlink")
}

// TestReadGzipTarFiles_SymlinkTarget verifies that the symlink target
// /usr/lib/os-release is extracted correctly when requested.
func TestReadGzipTarFiles_SymlinkTarget(t *testing.T) {
	buf := buildSymlinkTar(t, false)
	wanted := map[string]bool{
		"usr/lib/os-release": true,
	}
	files, err := readGzipTarFiles(bytes.NewReader(buf), wanted)
	require.NoError(t, err)
	require.Contains(t, files, "usr/lib/os-release")
	assert.Contains(t, string(files["usr/lib/os-release"]), "ID=alpine")
	assert.Contains(t, string(files["usr/lib/os-release"]), "VERSION_ID=3.21.0")
}

// TestResolveOSReleaseContent_Direct verifies the no-symlink case (alpine <3.21
// where /etc/os-release is a plain file).
func TestResolveOSReleaseContent_Direct(t *testing.T) {
	links := map[string]string{}
	contents := map[string][]byte{
		"etc/os-release": []byte("ID=alpine\nVERSION_ID=3.17.3\n"),
	}
	got, ok := resolveOSReleaseContent("etc/os-release", links, contents)
	require.True(t, ok)
	assert.Contains(t, got, "ID=alpine")
}

// TestResolveOSReleaseContent_Symlink verifies the two-hop resolution for
// alpine >=3.21 where /etc/os-release → /usr/lib/os-release.
func TestResolveOSReleaseContent_Symlink(t *testing.T) {
	links := map[string]string{
		"etc/os-release": "usr/lib/os-release",
	}
	contents := map[string][]byte{
		"usr/lib/os-release": []byte("ID=alpine\nVERSION_ID=3.21.0\n"),
	}
	got, ok := resolveOSReleaseContent("etc/os-release", links, contents)
	require.True(t, ok, "symlink chain must be resolved")
	assert.Contains(t, got, "ID=alpine")
	assert.Contains(t, got, "VERSION_ID=3.21.0")
}

// TestResolveOSReleaseContent_NotFound verifies that a missing target returns
// found=false rather than panicking or returning stale data.
func TestResolveOSReleaseContent_NotFound(t *testing.T) {
	links := map[string]string{
		"etc/os-release": "usr/lib/os-release",
	}
	contents := map[string][]byte{} // target file not extracted
	_, ok := resolveOSReleaseContent("etc/os-release", links, contents)
	assert.False(t, ok, "missing symlink target must return found=false")
}
