package types

import (
	"github.com/docker/distribution"
)

func FT_arm64_Layer1() Layer {
	ref := FT_arm_Layer1Reference()
	return Layer{
		Digest:    ref.Digest.String(),
		Size:      ref.Size,
		MediaType: ref.MediaType,
		Data:      FT_arm_Layer1Byte(),
	}
}

func FT_arm64_Layer0() Layer {
	l0ref := FT_arm_Layer0Reference()
	return Layer{
		Digest:    l0ref.Digest.String(),
		Size:      l0ref.Size,
		MediaType: l0ref.MediaType,
		Data:      FT_arm_Layer0Byte(),
	}
}

func FT_amd64_Layer0() Layer {
	ref := FT_amd_Layer0Reference()
	return Layer{
		Digest:    ref.Digest.String(),
		Size:      ref.Size,
		MediaType: ref.MediaType,
		Data:      FT_amd_Layer0Byte(),
	}
}

func FT_amd64_Layer1() Layer {
	ref := FT_amd_Layer1Reference()
	return Layer{
		Digest:    ref.Digest.String(),
		Size:      ref.Size,
		MediaType: ref.MediaType,
		Data:      FT_amd_Layer1Byte(),
	}
}

func FT_amd_Layer0Reference() distribution.Descriptor {
	return distribution.Descriptor{
		MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
		Size:      2796496,
		Digest:    "sha256:88ecf269dec31566a8e6b05147732fe34d32bc608de0d636dffaba659230a515",
	}
}

func FT_amd_Layer1Reference() distribution.Descriptor {
	return distribution.Descriptor{
		MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
		Size:      304,
		Digest:    "sha256:a2f2473b6a7e9b3d17c9873e41d04808b4c0310e0d4f7c13e03fa3f9a3a14bcc",
	}
}

// Layer
func FT_arm_Layer0Reference() distribution.Descriptor {
	return distribution.Descriptor{
		MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
		Size:      2693535,
		Digest:    "sha256:6ec0f660682c2f5b391af1d81d3b4839da06f050269546177b28e4b51d8d4c30",
	}
}

func FT_arm_Layer1Reference() distribution.Descriptor {
	return distribution.Descriptor{
		MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
		Size:      304,
		Digest:    "sha256:297a529dab3ab8530adef75e34219e97ec0f3cb2fd39efdb13a1cc0a70ece15a",
	}
}

func FT_arm_Layer0Byte() []byte {
	// read file and return bytes
	dat, err := f.ReadFile("data/layer/6ec0f660682c2f5b391af1d81d3b4839da06f050269546177b28e4b51d8d4c30.dat")
	if err != nil {
		panic(err)
	}
	return dat
}

func FT_arm_Layer1Byte() []byte {
	// read file and return bytes
	dat, err := f.ReadFile("data/layer/297a529dab3ab8530adef75e34219e97ec0f3cb2fd39efdb13a1cc0a70ece15a.dat")
	if err != nil {
		panic(err)
	}
	return dat
}

func FT_amd_Layer0Byte() []byte {
	// read file and return bytes
	dat, err := f.ReadFile("data/layer/88ecf269dec31566a8e6b05147732fe34d32bc608de0d636dffaba659230a515.dat")
	if err != nil {
		panic(err)
	}
	return dat
}

func FT_amd_Layer1Byte() []byte {
	// read file and return bytes
	dat, err := f.ReadFile("data/layer/a2f2473b6a7e9b3d17c9873e41d04808b4c0310e0d4f7c13e03fa3f9a3a14bcc.dat")
	if err != nil {
		panic(err)
	}
	return dat
}

func FT_arm_BlobUpload_Layer0() (OCIBlobUpload, []byte) {
	r := FT_arm_Layer0Reference()
	bu := OCIBlobUpload{
		UploadedBy: "test",
		BlobType:   "layer",
		BlobID:     1,
		Repo:       "qnib/featuretest",
		Digest:     r.Digest.String(),
	}
	return bu, FT_arm_Layer0Byte()
}

func FT_arm_BlobUpload_Config() (OCIBlobUpload, []byte) {
	cfg := FT_arm_Config()
	bu := OCIBlobUpload{
		UploadedBy: "test",
		BlobType:   "config",
		BlobID:     1,
		Repo:       "qnib/featuretest",
		Digest:     cfg.Digest,
	}
	return bu, FT_arm_ConfigByte()
}
