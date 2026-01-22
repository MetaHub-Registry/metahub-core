package types

// func TestManifestListFromData(t *testing.T) {
// 	mlBytes := FTamdManifestlist().ToOCIBytes()
// 	ml, err := NewManifestListFromData(mlBytes)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	t.Log(string(mlBytes))
// 	if ml.Digest != "sha256:3f0c6c8a4f6f5e2f3e4e5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b3c4d5e6f" {
// 		t.Fatalf("Unexpected digest: %s", ml.Digest)
// 	}
// 	if len(ml.Manifests) != 2 {
// 		t.Fatalf("Unexpected number of manifests: %d", len(ml.Manifests))
// 	}
// }
