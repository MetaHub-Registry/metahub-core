package magazine

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/backends/minio"
	"github.com/metahub-registry/metahub-core/pkg/backends/sqlite"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"github.com/metahub-registry/metahub-core/pkg/yaml/config"
	"golang.org/x/crypto/bcrypt"
)

var mag Magazine

var (
	featureTestGenericCfgDigest = "sha256:b109e1dac65af00234a8a7f817dcb1e18bc18ca97a5b805283995ebf1137e471"
	featureTestGenericCfg       = []byte(`{
		"architecture":"amd64",
		"config":{
			"Env":[
				"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
				"SKIP_ENTRYPOINTS=true",
				"PLATFORM_FEATURES=arch:cascadelake"
			],
			"Cmd":["/usr/bin/entry.sh"],
			"Labels":{"platform.features":"arch:cascadelake"},
			"ArgsEscaped":true,
			"OnBuild":null
		},
		"created":"2022-05-05T11:21:27.783635915Z",
		"history":[
			{"created":"2021-11-24T20:19:40.199700946Z","created_by":"/bin/sh -c #(nop) ADD file:9233f6f2237d79659a9521f7e390df217cec49f1a8aa3a12147bbca1956acdb9 in / "},
			{"created":"2021-11-24T20:19:40.483367546Z","created_by":"/bin/sh -c #(nop)  CMD [\"/bin/sh\"]","empty_layer":true},
			{"created":"2022-05-05T11:21:27.783635915Z","created_by":"ARG PLATFORM_FEATURES=test","comment":"buildkit.dockerfile.v0","empty_layer":true},
			{"created":"2022-05-05T11:21:27.783635915Z","created_by":"LABEL platform.features=arch:cascadelake","comment":"buildkit.dockerfile.v0","empty_layer":true},
			{"created":"2022-05-05T11:21:27.783635915Z","created_by":"COPY bin/entry.sh /usr/bin/entry.sh # buildkit","comment":"buildkit.dockerfile.v0"},
			{"created":"2022-05-05T11:21:27.783635915Z","created_by":"CMD [\"/usr/bin/entry.sh\"]","comment":"buildkit.dockerfile.v0","empty_layer":true},
			{"created":"2022-05-05T11:21:27.783635915Z","created_by":"ENV SKIP_ENTRYPOINTS=true","comment":"buildkit.dockerfile.v0","empty_layer":true},
			{"created":"2022-05-05T11:21:27.783635915Z","created_by":"ENV PLATFORM_FEATURES=arch:cascadelake","comment":"buildkit.dockerfile.v0","empty_layer":true}
		],
		"os":"linux",
		"rootfs":{
			"type":"layers",
			"diff_ids":["sha256:8d3ac3489996423f53d6087c81180006263b79f206d3fdec9e66f0e27ceb8759","sha256:aff8eba572d7cf1d51ae8245ac42189a896b0d0f069ad5f130795916ade7587a"]}}`)
	BlobUp_FeaturetestGeneric = types.OCIBlobUpload{
		Digest:     featureTestGenericCfgDigest,
		UUID:       uuid.New(),
		UploadedBy: "test",
		Repo:       "qnib/featuretest",
		BlobType:   "config",
		Data:       featureTestGenericCfg,
	}
)

func TestMain(m *testing.M) {
	logrus.Info("Start TestMain")
	ctx := context.Background()
	/// Minio
	minioC, err := minio.SetupMinio(ctx)
	if err != nil {
		logrus.Fatal(err)
	}
	defer minioC.Terminate(ctx)
	configsBucketKey := minio.GetConfigsBucketKey()
	layersBucketKey := minio.GetLayersBucketKey()
	manifestsBucketKey := minio.GetManifestsBucketKey()
	bc := map[string]minio.BucketConfig{
		configsBucketKey:   minio.NewBucketConfig(configsBucketKey, minioC.HttpURI),
		layersBucketKey:    minio.NewBucketConfig(layersBucketKey, minioC.HttpURI),
		manifestsBucketKey: minio.NewBucketConfig(manifestsBucketKey, minioC.HttpURI),
	}
	min, err := minio.NewMiniWithCfg(bc)
	if err != nil {
		logrus.Fatal(err)
	}
	err = min.ConnectClients()
	if err != nil {
		logrus.Fatal(err)
	}
	min.BucketCheck()
	/// SQLite
	f, err := os.CreateTemp("", "sqlite-test")
	if err != nil {
		logrus.Fatal(err)
	}

	logrus.Infof("Start TestMain: %s", f.Name())
	defer os.Remove(f.Name())
	sq, err := sqlite.New(f.Name(), "")
	if err != nil {
		logrus.Fatal(err)
	}
	mag = Magazine{
		Minio:  min,
		SQlite: sq,
	}
	code := m.Run()
	os.Exit(code)
	logrus.Info("End TestMain")
}

func TeardownTest(bs utils.Bootstrap) {
	logrus.Info("> TeardownTest")
	if bs.CopySqlite != "" {
		logrus.Infof("Copy SQlite DB to '%s'", bs.CopySqlite)
		mag.SQlite.Close()
		utils.CopyFile(mag.SQlite.MhFile, bs.CopySqlite)
		err := mag.SQlite.ReOpenDB()
		if err != nil {
			logrus.Fatal(err)
		}
	}
	err := mag.Minio.EmptyAll()
	if err != nil {
		logrus.Fatal(err)
	}
	err = mag.SQlite.EmptyData(bs.SqlEmptyAll)
	if err != nil {
		logrus.Fatal(err)
	}
}

func SetupTest(bs utils.Bootstrap) (err error) {
	logrus.Info("> SetupTest")
	for _, b := range bs.Bootstraps {
		logrus.Errorf("SetupTest: %s", b)
		switch b {
		case "":
		case "collection:qnib/featuretest":
			Setup_Collection_QNIB_FeatureTest()
		case "users":
			Setup_Profiles()
		case "profiles":
			Setup_Profiles()
		case "cfg:generic":
			Setup_FTarm_Config()
		default:
			err = fmt.Errorf("No teardown for '%s'", b)
			return err
		}
	}
	return
}

func Setup_FTarm_Config() {
	bu, b := types.FT_arm_BlobUpload_Config()
	err := mag.StoreBlobUpload(ctx, &bu, b)
	if err != nil {
		logrus.Fatal(err)
	}
}

func Setup_Users() {
	logrus.Warn("Setup Users")
	usrs, err := config.FindUsers(ctx, "../../../misc/yaml/users")
	if err != nil {
		logrus.Fatal(err)
	}
	for _, user := range usrs {
		u := types.User{
			Username:  user.Username,
			FirstName: user.FirstName,
			LastName:  user.LastName,
			Email:     user.Email,
		}
		switch {
		case (user.Hash != "") && (user.Password != ""):
			logrus.Errorf("User '%s' has both 'hash' and 'password' set. Please remove one of them.", user.Username)
			continue
		case user.Hash != "":
			u.PasswordHash = user.Hash
		case user.Password != "":
			hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), 12)
			if err != nil {
				logrus.Errorf("Error while hashing password for user '%s': %s", user.Username, err)
				continue
			}
			u.PasswordHash = string(hashedPassword)
		}
		err := mag.InsertUser(ctx, u)
		if err != nil {
			logrus.Error(err)
		}
	}
}

func Setup_Profiles() {
	logrus.Warn("Setup_Profiles")
	prfs, err := config.FindProfiles(ctx, "../../../misc/yaml/profiles")
	if err != nil {
		logrus.Fatal(err)
	}
	for _, prf := range prfs {
		f := types.Filter{
			Label: prf.Name,
		}
		feats := []string{}
		for _, act := range prf.Actions {
			for k, v := range act.Args {
				if k == "value" {
					feats = append(feats, v.(string))
				}
			}
		}
		f.Feature = strings.Join(feats, ",")
		msg := fmt.Sprintf("# Profile: %s", f.Label)
		if len(f.Feature) > 0 {
			msg += fmt.Sprintf(" [%s]", f.Feature)
		}
		logrus.Info(msg)
		fID, err := mag.SQliteInsertFilter(ctx, f)
		if err != nil {
			logrus.Fatal(err)
		}
		logrus.Infof("Filter '%s' created with ID: %d", f.Label, fID)
		// Profile
		p := types.Profile{
			Name: prf.Name,
		}
		prfId, err := mag.SQliteInsertProfile(ctx, p)
		if err != nil {
			logrus.Fatal(err)
		}
		logrus.Infof("Profile '%s' created with ID: %d", p.Name, prfId)
		err = mag.SQliteLinkProfileToFilter(ctx, prfId, fID)
		if err != nil {
			logrus.Error(err)
			continue
		}
	}
}

func Setup_Collection_QNIB_FeatureTest() {
	logrus.Warn("Setup_Collection_QNIB_FeatureTest")

}
