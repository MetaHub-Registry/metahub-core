package types

// DedupeLatestByUpstreamTagPlatform collapses runtime manifests that resolve to
// the SAME mutable upstream tag on the SAME platform down to a single entry — the
// latest push.
//
// Why: a mutable runtime tag (e.g. `…/ai/llama-cpp:cuda12-6_sm86-real`) that is
// rebuilt and re-pushed with a new digest is registered in the graph as a NEW
// :Manifest and appended to the runtime slot's append-only HAS_PUSHED history
// WITHOUT evicting the prior one. The per-profile index compute then binds BOTH
// the old and new manifest to the profile, and — since they share a platform
// (e.g. linux/amd64) — the resulting OCI index carries two same-platform members.
// docker resolves that nondeterministically and can serve the STALE base (which
// is exactly how a rebuilt runtime image with a new entrypoint failed to take
// effect). Collapsing same-(reg,repo,tag,platform) to the latest keeps a computed
// index to one current manifest per runtime tag+platform.
//
// Grouping key: (Upstream.Reg, Upstream.Repo, Upstream.Tag, Platform label). A
// manifest missing an upstream repo OR tag is passed through UNTOUCHED — it cannot
// be attributed to a mutable tag, so it never participates in a collapse. This
// makes over-collapse impossible: genuinely different runtime images (e.g. a rocm
// and a cuda base that both happen to be linux/amd64) carry different upstream
// tags and are never merged, and anything without an upstream tag is left alone.
// Under-collapse (e.g. if the same image were re-pushed under a drifted registry
// host) is the safe direction — it degrades to today's behaviour, never worse.
//
// "Latest" is the manifest with the highest node ID, a monotonic proxy for push
// recency (a later push creates a higher-id :Manifest node). The append-only
// HAS_PUSHED timestamp is the conceptual signal; node ID tracks it here for the
// currently-live nodes this operates on without a second query.
//
// Order is preserved: the first occurrence of each key keeps its position; a
// later, higher-id collision replaces the retained manifest in place.
func DedupeLatestByUpstreamTagPlatform(mds []Manifest) []Manifest {
	out := make([]Manifest, 0, len(mds))
	pos := make(map[string]int, len(mds)) // grouping key -> index into out
	for _, m := range mds {
		if m.Upstream.Repo == "" || m.Upstream.Tag == "" {
			out = append(out, m) // not attributable to a mutable tag: never collapse
			continue
		}
		key := m.Upstream.Reg + "\x00" + m.Upstream.Repo + "\x00" + m.Upstream.Tag + "\x00" + m.Platform.GetLabel()
		if i, seen := pos[key]; seen {
			if m.ID > out[i].ID {
				out[i] = m // a later (higher-id) push supersedes the retained one
			}
			continue
		}
		pos[key] = len(out)
		out = append(out, m)
	}
	return out
}
