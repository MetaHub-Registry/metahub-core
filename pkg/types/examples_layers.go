package types

import (
	"github.com/docker/distribution"
	"github.com/opencontainers/go-digest"
)

var (
	Layer0Glow150ARM64DT = NewDataTuple("data/layer/0ff856ae13827b74e79413147bb2ae1869a47877741c9a10fd70bea36d6f160c.dat")
	Layer0Glow150ARM64   = Layer{
		Digest:    Layer0Glow150ARM64DT.Digest.String(),
		Size:      5833119,
		MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
		Data:      Layer0Glow150ARM64DT.Data,
	}
	Layer1Glow150ARM64DT = NewDataTuple("data/layer/3571972328e9561a0c1f7e6fa608edf090eacc168ac29d0a9f234ed797708d54.dat")
	Layer1Glow150ARM64   = Layer{
		Digest:    Layer1Glow150ARM64DT.Digest.String(),
		Size:      3072,
		MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
		Data:      Layer1Glow150ARM64DT.Data,
	}
	Layer2Glow150ARM64DT = NewDataTuple("data/layer/d54e369a89429ce6226ed2c646caecdaf7538180c2e26507472ff7188738313f.dat")
	Layer2Glow150ARM64   = Layer{
		Digest:    Layer2Glow150ARM64DT.Digest.String(),
		Size:      3584,
		MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
		Data:      Layer2Glow150ARM64DT.Data,
	}
	Layer0Glow151ARM64DT = NewDataTuple("data/layer/7c574a510ef9d3b6b3a5bcd60886da206f81277f1b646acd3f11ee9ff31adaa7.dat")
	Layer0Glow151ARM64   = Layer{
		Digest:    Layer0Glow151ARM64DT.Digest.String(),
		Size:      5877038,
		MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
		Data:      Layer0Glow151ARM64DT.Data,
	}
	Layer1Glow151ARM64DT = NewDataTuple("data/layer/8a170a58bdf6977be6f8b7652ba4373dbeb140c5c8173785297117900d13b968.dat")
	Layer1Glow151ARM64   = Layer{
		Digest:    Layer1Glow151ARM64DT.Digest.String(),
		Size:      221,
		MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
		Data:      Layer1Glow151ARM64DT.Data,
	}
	Layer2Glow151ARM64DT = NewDataTuple("data/layer/a24ce36aa43c36ba448b33f11e76ee61a4f8df83d31532558cac872a908e3525.dat")
	Layer2Glow151ARM64   = Layer{
		Digest:    Layer2Glow151ARM64DT.Digest.String(),
		Size:      309,
		MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
		Data:      Layer2Glow151ARM64DT.Data,
	}
	/// ARM64
	Layer0FTARM64DT = NewDataTuple("data/layer/6ec0f660682c2f5b391af1d81d3b4839da06f050269546177b28e4b51d8d4c30.dat")
	Layer0FTARM64   = Layer{
		Digest:    Layer0FTARM64DT.Digest.String(),
		Size:      2693535,
		MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
		Data:      Layer0FTARM64DT.Data,
	}
	Layer1FTARM64DT = NewDataTuple("data/layer/297a529dab3ab8530adef75e34219e97ec0f3cb2fd39efdb13a1cc0a70ece15a.dat")
	Layer1FTARM64   = Layer{
		Digest:    Layer1FTARM64DT.Digest.String(),
		Size:      304,
		MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
		Data:      Layer1FTARM64DT.Data,
	}
)

const ()

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

func FT_zen3_Layer0() Layer {
	ref := FT_zen3_Layer0Reference()
	return Layer{
		Digest:    ref.Digest.String(),
		Size:      ref.Size,
		MediaType: ref.MediaType,
		Data:      FT_amd_Layer0Byte(),
	}
}

func FT_zen3_Layer1() Layer {
	ref := FT_zen3_Layer1Reference()
	return Layer{
		Digest:    ref.Digest.String(),
		Size:      ref.Size,
		MediaType: ref.MediaType,
		Data:      FT_amd_Layer1Byte(),
	}
}

func newLayerFromDT(dt DataTuple) Layer {
	return Layer{
		Digest:    string(digest.FromBytes(dt.Data)),
		Size:      int64(len(dt.Data)),
		MediaType: MediaTypeLayer,
		Data:      dt.Data,
	}
}

func LayerDTFeaturetestLf144() DataTuple {
	return NewDataTuple("data/layer/f1417ff83b319fbdae6dd9cd6d8c9c88002dcd75ecf6ec201c8c6894681cf2b5.dat")
}

func LayerFeaturetestLf144() Layer {
	return newLayerFromDT(LayerDTFeaturetestLf144())
}

func LayerDTFeaturetestL4c85() DataTuple {
	return NewDataTuple("data/layer/4c85de3e6e41d6a5fc11e77e87cd57cddcfa84003e12c93fd62075d924da2636.dat")
}
func LayerFeaturetestL4c85() Layer {
	return newLayerFromDT(LayerDTFeaturetestL4c85())
}

func FT_amd_Layer0Reference() distribution.Descriptor {
	return distribution.Descriptor{
		MediaType: MediaTypeLayer,
		Size:      2796496,
		Digest:    "sha256:88ecf269dec31566a8e6b05147732fe34d32bc608de0d636dffaba659230a515",
	}
}

func FT_zen3_Layer1Reference() distribution.Descriptor {
	return distribution.Descriptor{
		MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
		Size:      364,
		Digest:    "sha256:40a6b36cce2bd722e56eb5afc76842d2600de949b417307051d38c5a5654a708",
	}
}

func FT_zen3_Layer0Reference() distribution.Descriptor {
	return distribution.Descriptor{
		MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
		Size:      3374563,
		Digest:    "sha256:f56be85fc22e46face30e2c3de3f7fe7c15f8fd7c4e5add29d7f64b87abdaa09",
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
		UploadUser:    "test",
		UploadProfile: "generic",
		BlobType:      "layer",
		BlobID:        1,
		Repo:          "qnib/featuretest",
		Digest:        r.Digest.String(),
	}
	return bu, FT_arm_Layer0Byte()
}

func FT_arm_BlobUpload_Config() (OCIBlobUpload, []byte) {
	cfg := FT_arm_Config()
	bu := OCIBlobUpload{
		UploadUser:    "test",
		UploadProfile: "generic",
		BlobType:      "config",
		BlobID:        1,
		MediaType:     cfg.MediaType,
		Repo:          "qnib/featuretest",
		Digest:        cfg.Digest,
	}
	return bu, ConfigByteFtArm64()
}
