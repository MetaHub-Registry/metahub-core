package sqlite

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"github.com/metahub-registry/metahub-core/pkg/yaml/config"
	"golang.org/x/crypto/bcrypt"
)

var (
	sq  Sqlite
	ctx = context.Background()
)

func TestMain(m *testing.M) {
	f, err := os.CreateTemp("", "sqlite-test")
	if err != nil {
		logrus.Fatal(err)
	}

	logrus.Infof("Start TestMain: %s", f.Name())
	defer os.Remove(f.Name())
	sq, err = New(f.Name(), "")
	if err != nil {
		logrus.Fatal(err)
	}
	code := m.Run()
	os.Exit(code)
	logrus.Info("End TestMain")
}

func TeardownTest(bs utils.Bootstrap) {
	logrus.Info("> TeardownTest")
	if bs.CopySqlite != "" {
		logrus.Infof("Copy SQlite DB to '%s'", bs.CopySqlite)
		sq.Close()
		utils.CopyFile(sq.MhFile, bs.CopySqlite)
		err := sq.ReOpenDB()
		if err != nil {
			logrus.Fatal(err)
		}
	}
	err := sq.EmptyData(bs.SqlEmptyAll)
	if err != nil {
		logrus.Fatal(err)
	}
}

func SetupTest(bs utils.Bootstrap) (br utils.BootstrapResult, err error) {
	logrus.Info("> SetupTest")
	br = utils.NewBootstrapResult()
	for _, b := range bs.Bootstraps {
		logrus.Errorf("SetupTest: %s", b)
		switch b {
		case "profiles":
			err = Setup_Profiles(&br)
			if err != nil {
				return
			}
		case "platforms":
			err = Setup_Platforms(&br)
			if err != nil {
				return
			}
		case "users":
			err = Setup_Users(&br)
			if err != nil {
				return
			}
		case "manifest:qnib/featuretest-arm64":
			err = Setup_Manifest_QNIB_FeatureTest_arm(&br)
			if err != nil {
				return
			}
		case "config:qnib/featuretest-arm64":
			err = Setup_Config_FTarm(&br)
			if err != nil {
				return
			}
		case "upstreams:qnib/featuretest-arm64":
			err = Setup_Upstream_FTarm(&br)
			if err != nil {
				return
			}
		case "layers:qnib/featuretest-arm64":
			err = Setup_Layers_FTarm(&br)
			if err != nil {
				return
			}
		case "manifestlist:qnib/featuretest-generic-arm64":
			err = Setup_Manifestlist_FTarm(&br)
			if err != nil {
				return
			}
		case "collection:qnib/featuretest":
			err = Setup_Collection_QNIB_FeatureTest(&br)
			if err != nil {
				return
			}
		case "filter:generic":
			err = Setup_Filter_Generic(&br)
			if err != nil {
				return
			}
		default:
			err = fmt.Errorf("No teardown for '%s'", b)
			return br, err
		}
	}
	return
}

func Setup_Manifestlist_FTarm(br *utils.BootstrapResult) (err error) {
	ml := types.FT_generic_arm_Manifestlist()
	err = sq.ManifestList.Insert(&ml)
	if err != nil {
		return
	}
	br.Add("manifestlist:qnib/featuretest-generic-arm64#id", fmt.Sprint(ml.SqlID))
	return
}

func Setup_Layers_FTarm(br *utils.BootstrapResult) (err error) {
	//logrus.Warn("Setup_Layers_FTarm")
	// Create layers
	l0 := types.FT_arm_Layer0Reference()
	l0ID, err := sq.Layer.InsertDescriptor(l0)
	if err != nil {
		return
	}
	br.Add("layer:qnib/featuretest-arm64#id0", fmt.Sprint(l0ID))
	br.Add("layer:qnib/featuretest-arm64#digest0", l0.Digest.String())
	br.Add("layer:qnib/featuretest-arm64#size0", fmt.Sprint(l0.Size))
	l1 := types.FT_arm_Layer1Reference()
	l1ID, err := sq.Layer.InsertDescriptor(l1)
	if err != nil {
		return
	}
	br.Add("layer:qnib/featuretest-arm64#id1", fmt.Sprint(l1ID))
	br.Add("layer:qnib/featuretest-arm64#digest1", l1.Digest.String())
	br.Add("layer:qnib/featuretest-arm64#size1", fmt.Sprint(l1.Size))
	return
}

func Setup_Platforms(br *utils.BootstrapResult) (err error) {
	//logrus.Info("> Setup_Platforms")
	// Create platforms
	plts := []types.Platform{
		{Arch: "amd64", Variant: "", OS: "linux", Features: ""},
		{Arch: "arm", Variant: "", OS: "linux", Features: ""},
		{Arch: "arm", Variant: "v5", OS: "linux", Features: ""},
		{Arch: "arm", Variant: "v6", OS: "linux", Features: ""},
		{Arch: "arm", Variant: "v7", OS: "linux", Features: ""},
		{Arch: "arm64", Variant: "v8", OS: "linux", Features: ""},
	}
	for _, p := range plts {
		err := sq.Platform.Insert(&p)
		if err != nil {
			return err
		}
		pStr := fmt.Sprintf("%s/%s", p.OS, p.Arch)
		if p.Variant != "" {
			pStr = fmt.Sprintf("%s/%s", pStr, p.Variant)
		}
		k := fmt.Sprintf("platform:%s#id", pStr)
		br.Add(k, fmt.Sprint(p.SqlID))
		// Check if filters are already there, if so link
		f := types.Filter{
			Label: "generic",
		}
		if p.Features != "" {
			f.Label = strings.TrimPrefix(p.Features, "cpu:")
		}
		fID, found, err := sq.Filter.GetFilter(f)
		if err != nil {
			return err
		}
		f.SqlID = fID
		if found {
			pfp := types.ProfileFilterPlatform{
				Platform: p,
				Filter:   f,
			}
			err = sq.Platform.InsertPlatformFilter(pfp)
			if err != nil {
				return err
			}
		} else {
			logrus.Warnf("No filter for platform '%s'", pStr)
		}
	}
	return
}

func Setup_Upstream_FTarm(br *utils.BootstrapResult) (err error) {
	logrus.Warn("Setup_Upstream_FTarm")
	// Create upstream for qnib/featuretest
	up := types.FT_arm_Upstream()
	err = sq.Upstream.Insert(&up)
	if err != nil {
		return
	}
	br.Add("upstream:qnib/featuretest-arm64#id", fmt.Sprint(up.SqlID))
	return
}

func Setup_Filter_Generic(br *utils.BootstrapResult) (err error) {
	//logrus.Info("> Setup_Filter_Generic")
	// Create filter
	f := types.Filter{
		Label: "generic",
	}
	err = sq.Filter.InsertFilter(&f)
	if err != nil {
		return
	}
	br.Add("filter:generic#id", fmt.Sprint(f.SqlID))
	return
}

func Setup_Config_FTarm(br *utils.BootstrapResult) (err error) {
	//logrus.Warn("Setup_Config_FTarm")
	cfg := types.FT_arm_Config()
	err = sq.Config.Insert(&cfg)
	if err != nil {
		return
	}
	br.Add("config:qnib/featuretest-arm64#id", fmt.Sprint(cfg.SqlID))
	br.Add("config:qnib/featuretest-arm64#digest", cfg.Digest)
	br.Add("config:qnib/featuretest-arm64#size", fmt.Sprint(cfg.Size))
	return
}

func Setup_Manifest_QNIB_FeatureTest_arm(br *utils.BootstrapResult) (err error) {
	logrus.Warn("Setup_Maniest_QNIB_FeatureTest_arm")
	// Create cmanifest for qnib/featuretest
	mani := types.FT_arm_Manifest()
	// Upstream ID
	upID, err := br.GetInt64("upstream:qnib/featuretest-arm64#id")
	if err != nil {
		return err
	}
	mani.Upstream.SqlID = upID
	// ConfigID
	cfgID, err := br.GetInt64("config:qnib/featuretest-arm64#id")
	if err != nil {
		return err
	}
	mani.Config.SqlID = cfgID
	// Layers
	/// L0
	l0 := types.Layer{}
	if loID, err := br.GetInt64("layer:qnib/featuretest-arm64#id0"); err != nil {
		return err
	} else {
		l0.SqlID = loID
	}
	if v, err := br.Get("layer:qnib/featuretest-arm64#digest0"); err != nil {
		return err
	} else {
		l0.Digest = v
	}
	if v, err := br.GetInt64("layer:qnib/featuretest-arm64#size0"); err != nil {
		return err
	} else {
		l0.Size = v
	}
	mani.Layers = append(mani.Layers, l0)
	// / L0
	l1 := types.Layer{}
	if v, err := br.GetInt64("layer:qnib/featuretest-arm64#id1"); err != nil {
		return err
	} else {
		l1.SqlID = v
	}
	if v, err := br.Get("layer:qnib/featuretest-arm64#digest1"); err != nil {
		return err
	} else {
		l1.Digest = v
	}
	if v, err := br.GetInt64("layer:qnib/featuretest-arm64#size1"); err != nil {
		return err
	} else {
		l1.Size = v
	}
	mani.Layers = append(mani.Layers, l1)
	// Profile
	mani.Platform = types.Platform{
		Arch:    "arm64",
		OS:      "linux",
		Variant: "v8",
	}
	if v, err := br.GetInt64("platform:linux/arm64/v8#id"); err != nil {
		return err
	} else {
		mani.Platform.SqlID = v
	}
	err = sq.Manifest.Insert(&mani)
	if err != nil {
		return
	}
	br.Add("manifest:qnib/featuretest-arm64#id", fmt.Sprint(mani.SqlID))
	br.Add("manifest:qnib/featuretest-arm64#digest", mani.Digest)
	return
}

func Setup_Profiles(br *utils.BootstrapResult) (err error) {
	logrus.Warn("Setup_Profiles")
	prfs, err := config.FindProfiles(ctx, "../../../misc/yaml/profiles")
	if err != nil {
		return err
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
		//logrus.Info(msg)
		fID, success, err := sq.Filter.GetFilter(f)
		if err != nil {
			return err
		}
		if !success {
			err = sq.Filter.InsertFilter(&f)
			if err != nil {
				return err
			}
			fID = f.SqlID
		}
		br.Add(fmt.Sprintf("filter:%s#id", f.Label), fmt.Sprint(fID))
		//logrus.Infof("Filter '%s' created with ID: %d", f.Label, fID)
		// Profile
		p := types.Profile{
			Name: prf.Name,
		}
		prfId, success, err := sq.Profile.HasProfile(p.Name)
		if err != nil {
			return err
		}
		if !success {
			err = sq.Profile.Insert(&p)
			if err != nil {
				return err
			}
			prfId = p.SqlID
		}
		br.Add(fmt.Sprintf("profile:%s#id", p.Name), fmt.Sprint(prfId))
		//logrus.Infof("Profile '%s' created with ID: %d", p.Name, prfId)
		err = sq.LinkProfileToFilter(ctx, prfId, fID)
		if err != nil {
			return err
		}
	}
	return
}

func Setup_Collection_QNIB_FeatureTest(br *utils.BootstrapResult) (err error) {
	//logrus.Info("> Setup_Collection_QNIB_FeatureTest")
	// Create collection
	c := types.FT_arm_Collection()
	err = sq.Collection.Insert(&c)
	if err != nil {
		return
	}
	br.Add("collection:qnib/featuretest#id", fmt.Sprint(c.SqlID))
	// Link Collection and ManifestList
	mlID, err := br.GetInt64("manifestlist:qnib/featuretest-generic-arm64#id")
	if err != nil {
		return err
	}
	err = sq.Collection.InsertXTable(c.SqlID, mlID)
	if err != nil {
		return
	}
	return
}

func Setup_Users(br *utils.BootstrapResult) (err error) {
	logrus.Warn("Setup Users")
	usrs := []config.User{
		{Username: "test", Email: "test@test.de", FirstName: "Max", LastName: "Mustermann", Password: "test"},
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
			return
		case user.Hash != "":
			u.PasswordHash = user.Hash
		case user.Password != "":
			hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), 12)
			if err != nil {
				logrus.Errorf("Error while hashing password for user '%s': %s", user.Username, err)
				return err
			}
			u.PasswordHash = string(hashedPassword)
		}
		_, err = sq.InsertUser(ctx, u)
		if err != nil {
			return err
		}
	}
	return
}
