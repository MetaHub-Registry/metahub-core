package sqlite

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"github.com/metahub-registry/metahub-core/pkg/yaml/config"
	"golang.org/x/crypto/bcrypt"
)

func Setup_Manifestlist_FTarm(sq *Sqlite, br *utils.BootstrapResult) (err error) {
	ml := types.FT_arm_Manifestlist()
	err = sq.ManifestList.Insert(&ml)
	if err != nil {
		return
	}
	br.Add("manifestlist:qnib/featuretest:arm64#id", fmt.Sprint(ml.SqlID))
	return
}

func Setup_Manifestlist_FT_generic(sq *Sqlite, br *utils.BootstrapResult) (err error) {
	ml := types.FT_generic_Manifestlist()
	err = sq.ManifestList.Insert(&ml)
	if err != nil {
		return
	}
	br.Add("manifestlist:qnib/featuretest:arm64#id", fmt.Sprint(ml.SqlID))
	return
}

func Setup_Manifestlist_FTamd64(sq *Sqlite, br *utils.BootstrapResult) (err error) {
	ml := types.FT_amd_Manifestlist()
	err = sq.ManifestList.Insert(&ml)
	if err != nil {
		return
	}
	br.Add("manifestlist:qnib/featuretest:amd64#id", fmt.Sprint(ml.SqlID))
	return
}

func Setup_Layers_FTarm(sq *Sqlite, br *utils.BootstrapResult) (err error) {
	//logrus.Warn("Setup_Layers_FTarm")
	// Create layers
	l0 := types.FT_arm_Layer0Reference()
	l0ID, err := sq.Layer.InsertDescriptor(l0)
	if err != nil {
		return
	}
	br.Add("layer:qnib/featuretest:arm64#id0", fmt.Sprint(l0ID))
	br.Add("layer:qnib/featuretest:arm64#digest0", l0.Digest.String())
	br.Add("layer:qnib/featuretest:arm64#size0", fmt.Sprint(l0.Size))
	l1 := types.FT_arm_Layer1Reference()
	l1ID, err := sq.Layer.InsertDescriptor(l1)
	if err != nil {
		return
	}
	br.Add("layer:qnib/featuretest:arm64#id1", fmt.Sprint(l1ID))
	br.Add("layer:qnib/featuretest:arm64#digest1", l1.Digest.String())
	br.Add("layer:qnib/featuretest:arm64#size1", fmt.Sprint(l1.Size))
	return
}

func Setup_Platforms(sqObj *Sqlite, br *utils.BootstrapResult) (err error) {
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
		err := sqObj.Platform.Insert(&p)
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
		fID, found, err := sqObj.Filter.GetFilter(f)
		if err != nil {
			return err
		}
		f.SqlID = fID
		if found {
			pfp := types.ProfileFilterPlatform{
				Platform: p,
				Filter:   f,
			}
			err = sqObj.Platform.InsertPlatformFilter(pfp)
			if err != nil {
				return err
			}
		} else {
			logrus.Warnf("No filter for platform '%s'", pStr)
		}
	}
	return
}

func Setup_Upstream_FTarm(sq *Sqlite, br *utils.BootstrapResult) (err error) {
	logrus.Warn("Setup_Upstream_FTarm")
	// Create upstream for qnib/featuretest
	up := types.FT_arm_Upstream()
	err = sq.Upstream.Insert(&up)
	if err != nil {
		return
	}
	br.Add("upstream:qnib/featuretest:arm64#id", fmt.Sprint(up.SqlID))
	return
}

func Setup_Filter_Generic(sq *Sqlite, br *utils.BootstrapResult) (err error) {
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

func Setup_Config_FTarm(sq *Sqlite, br *utils.BootstrapResult) (err error) {
	//logrus.Warn("Setup_Config_FTarm")
	cfg := types.FT_arm_Config()
	err = sq.Config.Insert(&cfg)
	if err != nil {
		return
	}
	br.Add("config:qnib/featuretest:arm64#id", fmt.Sprint(cfg.SqlID))
	br.Add("config:qnib/featuretest:arm64#digest", cfg.Digest)
	br.Add("config:qnib/featuretest:arm64#size", fmt.Sprint(cfg.Size))
	return
}

func Setup_Manifest_QNIB_FeatureTest_amd(sq *Sqlite, br *utils.BootstrapResult) (err error) {
	logrus.Warn("Setup_Maniest_QNIB_FeatureTest_arm")
	// Create cmanifest for qnib/featuretest
	mani := types.FT_amd_Manifest()
	_ = mani
	return
}

func Setup_Manifest_QNIB_FeatureTest_arm(sq *Sqlite, br *utils.BootstrapResult) (err error) {
	logrus.Warn("Setup_Maniest_QNIB_FeatureTest_arm")
	// Create cmanifest for qnib/featuretest
	mani := types.FT_arm_Manifest()
	// Upstream ID
	upID, err := br.GetInt64("upstream:qnib/featuretest:arm64#id")
	if err != nil {
		return err
	}
	mani.Upstream.SqlID = upID
	// ConfigID
	cfgID, err := br.GetInt64("config:qnib/featuretest:arm64#id")
	if err != nil {
		return err
	}
	mani.Config.SqlID = cfgID
	// Layers
	/// L0
	l0 := types.Layer{}
	if loID, err := br.GetInt64("layer:qnib/featuretest:arm64#id0"); err != nil {
		return err
	} else {
		l0.SqlID = loID
	}
	if v, err := br.Get("layer:qnib/featuretest:arm64#digest0"); err != nil {
		return err
	} else {
		l0.Digest = v
	}
	if v, err := br.GetInt64("layer:qnib/featuretest:arm64#size0"); err != nil {
		return err
	} else {
		l0.Size = v
	}
	mani.Layers = append(mani.Layers, l0)
	// / L0
	l1 := types.Layer{}
	if v, err := br.GetInt64("layer:qnib/featuretest:arm64#id1"); err != nil {
		return err
	} else {
		l1.SqlID = v
	}
	if v, err := br.Get("layer:qnib/featuretest:arm64#digest1"); err != nil {
		return err
	} else {
		l1.Digest = v
	}
	if v, err := br.GetInt64("layer:qnib/featuretest:arm64#size1"); err != nil {
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
	br.Add("manifest:qnib/featuretest:arm64#id", fmt.Sprint(mani.SqlID))
	br.Add("manifest:qnib/featuretest:arm64#digest", mani.Digest)
	return
}

func Setup_Profiles(sqObj *Sqlite, br *utils.BootstrapResult, d string) (err error) {
	ctx := context.Background()
	logrus.Warn("Setup_Profiles")
	prfs, err := config.FindProfiles(ctx, d)
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
		logrus.Info(msg)
		fID, success, err := sqObj.Filter.GetFilter(f)
		if err != nil {
			return err
		}
		if !success {
			err = sqObj.Filter.InsertFilter(&f)
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
		prfId, success, err := sqObj.Profile.HasProfile(p.Name)
		if err != nil {
			return err
		}
		if !success {
			err = sqObj.Profile.Insert(&p)
			if err != nil {
				return err
			}
			prfId = p.SqlID
		}
		br.Add(fmt.Sprintf("profile:%s#id", p.Name), fmt.Sprint(prfId))
		//logrus.Infof("Profile '%s' created with ID: %d", p.Name, prfId)
		err = sqObj.LinkProfileToFilter(ctx, prfId, fID)
		if err != nil {
			return err
		}
	}
	return
}
func Setup_Collection_QNIB_FeatureTest_Generic(sq *Sqlite, br *utils.BootstrapResult) (err error) {
	cTag := types.CollectionTag_Generic()
	err = sq.CollectionTag.Insert(&cTag)
	if err != nil {
		return
	}
	br.Add("collection_tag:generic#id", fmt.Sprint(cTag.SqlID))
	// Create collection
	c := types.FT_generic_Collection()
	c.Tag.SqlID = cTag.SqlID
	err = sq.Collection.Insert(&c)
	if err != nil {
		return
	}
	br.Add("collection:qnib/featuretest:generic#id", fmt.Sprint(c.SqlID))

	// Link Collection and ManifestList
	mlID, err := br.GetInt64("manifestlist:qnib/featuretest:arm64#id")
	if err != nil {
		return err
	}
	err = sq.Collection.InsertXTable(c.SqlID, cTag.SqlID, mlID)
	if err != nil {
		return
	}
	return
}

/*
	func Setup_Collection_QNIB_FeatureTestLatest(sq *Sqlite, br *utils.BootstrapResult) (err error) {
		//logrus.Info("> Setup_Collection_QNIB_FeatureTest")
		cTag := types.CollectionTag_Latest()
		err = sq.CollectionTag.Insert(&cTag)
		if err != nil {
			return
		}
		br.Add("collection_tag:latest#id", fmt.Sprint(cTag.SqlID))
		// Create collection
		c := types.FT_arm_Collection()
		c.Tag.SqlID = cTag.SqlID
		err = sq.Collection.Insert(&c)
		if err != nil {
			return
		}
		br.Add("collection:qnib/featuretest:latest#id", fmt.Sprint(c.SqlID))

		// Link Collection and ManifestList
		mlID, err := br.GetInt64("manifestlist:qnib/featuretest:arm64#id")
		if err != nil {
			return err
		}
		err = sq.Collection.InsertXTable(c.SqlID, cTag.SqlID, mlID)
		if err != nil {
			return
		}
		return
	}
*/

func Setup_Collection_QNIB_FeatureTest_arm(sq *Sqlite, br *utils.BootstrapResult) (err error) {
	//logrus.Info("> Setup_Collection_QNIB_FeatureTest")
	cTag := types.CollectionTag_arm()
	err = sq.CollectionTag.Insert(&cTag)
	if err != nil {
		return
	}
	br.Add("collection_tag:arm64#id", fmt.Sprint(cTag.SqlID))
	// Create collection
	c := types.FT_arm_Collection()
	c.Tag.SqlID = cTag.SqlID
	err = sq.Collection.Insert(&c)
	if err != nil {
		return
	}
	br.Add("collection:qnib/featuretest:arm64#id", fmt.Sprint(c.SqlID))

	// Link Collection and ManifestList
	mlID, err := br.GetInt64("manifestlist:qnib/featuretest:arm64#id")
	if err != nil {
		return err
	}
	err = sq.Collection.InsertXTable(c.SqlID, cTag.SqlID, mlID)
	if err != nil {
		return
	}
	return
}

/*func Setup_Collection_QNIB_FeatureTest_amd64(sq *Sqlite, br *utils.BootstrapResult) (err error) {
	//logrus.Info("> Setup_Collection_QNIB_FeatureTest")
	// Create collection
	c := types.FT_arm_Collection()
	err = sq.Collection.Insert(&c)
	if err != nil {
		return
	}
	br.Add("collection:qnib/featuretest:latest#id", fmt.Sprint(c.SqlID))
	// Link Collection and ManifestList
	mlID, err := br.GetInt64("manifestlist:qnib/featuretest:amd64#id")
	if err != nil {
		return err
	}
	err = sq.Collection.InsertXTable(c.SqlID, mlID)
	if err != nil {
		return
	}
	return
}*/

func Setup_Users(sqObj *Sqlite, br *utils.BootstrapResult) (err error) {
	logrus.Warn("Setup Users")
	ctx := context.Background()
	usrs := []config.User{
		{Username: "test", Email: "test@test.de", FirstName: "Max", LastName: "Mustermann", Password: "test", Role: "user"},
		{Username: "admin", Email: "admin@test.de", FirstName: "Admin", LastName: "Mustermann", Password: "admin", Role: "admin"},
	}
	for _, user := range usrs {
		u := types.User{
			Username:     user.Username,
			FirstName:    user.FirstName,
			LastName:     user.LastName,
			Email:        user.Email,
			PasswordHash: "",
			Role:         user.Role,
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
		_, err = sqObj.InsertUser(ctx, u)
		if err != nil {
			return err
		}
		bearerToken := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", u.Username, user.Password)))
		br.Add(fmt.Sprintf("user:%s#bearer", u.Username), bearerToken)
		br.Add(fmt.Sprintf("user:%s#hash", u.Username), u.PasswordHash)
	}
	return
}
