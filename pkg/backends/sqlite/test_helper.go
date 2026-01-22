package sqlite

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"github.com/metahub-registry/metahub-core/pkg/yaml/config"
)

func Setup_Manifestlist_FTarm(sq *Sqlite, br *utils.BootstrapResult) (err error) {
	ml := types.FTarmManifestlist()
	err = sq.InsertManifestList(&ml)
	if err != nil {
		return
	}
	br.Add("manifestlist:qnib/featuretest:arm64#id", fmt.Sprint(ml.ID))
	return
}

func Setup_Manifestlist_FT_generic(sq *Sqlite, br *utils.BootstrapResult) (err error) {
	ml := types.FTgenericManifestlist()
	err = sq.InsertManifestList(&ml)
	if err != nil {
		return
	}
	br.Add("manifestlist:qnib/featuretest:arm64#id", fmt.Sprint(ml.ID))
	return
}

func Setup_Manifestlist_FTamd64(sq *Sqlite, br *utils.BootstrapResult) (err error) {
	ml := types.FTamdManifestlist()
	err = sq.InsertManifestList(&ml)
	if err != nil {
		return
	}
	br.Add("manifestlist:qnib/featuretest:amd64#id", fmt.Sprint(ml.ID))
	return
}

func Setup_Layers_FT_amd64(sq *Sqlite, br *utils.BootstrapResult) (err error) {
	//logrus.Warn("Setup_Layers_FT_amd64")
	// Create layers
	l0 := types.FTamdLayer0Reference()
	l0ID, err := sq.Layer.InsertDescriptor(l0)
	if err != nil {
		return
	}
	br.Add("layer:qnib/featuretest:amd64#id0", fmt.Sprint(l0ID))
	br.Add("layer:qnib/featuretest:amd64#digest0", l0.Digest.String())
	br.Add("layer:qnib/featuretest:amd64#size0", fmt.Sprint(l0.Size))
	l1 := types.FTamdLayer1Reference()
	l1ID, err := sq.Layer.InsertDescriptor(l1)
	if err != nil {
		return
	}
	br.Add("layer:qnib/featuretest:amd64#id1", fmt.Sprint(l1ID))
	br.Add("layer:qnib/featuretest:amd64#digest1", l1.Digest.String())
	br.Add("layer:qnib/featuretest:amd64#size1", fmt.Sprint(l1.Size))
	return
}

func Setup_Layers_FTarm(sq *Sqlite, br *utils.BootstrapResult) (err error) {
	//logrus.Warn("Setup_Layers_FTarm")
	// Create layers
	l0 := types.FTarmLayer0Reference()
	l0ID, err := sq.Layer.InsertDescriptor(l0)
	if err != nil {
		return
	}
	br.Add("layer:qnib/featuretest:arm64#id0", fmt.Sprint(l0ID))
	br.Add("layer:qnib/featuretest:arm64#digest0", l0.Digest.String())
	br.Add("layer:qnib/featuretest:arm64#size0", fmt.Sprint(l0.Size))
	l1 := types.FTarmLayer1Reference()
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
	logrus.Warn("> Setup_Platforms")
	/*
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
			br.Add(k, fmt.Sprint(p.ID))
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
			f.ID = fID
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
	*/
	return
}

func Setup_Upstream_FT_amd64(sq *Sqlite, br *utils.BootstrapResult) (err error) {
	//logrus.Warn("Setup_Upstream_FT_amd64")
	// Create upstream for qnib/featuretest
	up := types.FTamd64Upstream()
	err = sq.Upstream.Insert(&up)
	if err != nil {
		return
	}
	br.Add("upstream:qnib/featuretest:amd64#id", fmt.Sprint(up.ID))
	return
}

func Setup_Upstream_FTarm(sq *Sqlite, br *utils.BootstrapResult) (err error) {
	//logrus.Warn("Setup_Upstream_FTarm")
	// Create upstream for qnib/featuretest
	up := types.FTarm64Upstream()
	err = sq.Upstream.Insert(&up)
	if err != nil {
		return
	}
	br.Add("upstream:qnib/featuretest:arm64#id", fmt.Sprint(up.ID))
	return
}

func Setup_Filter_Generic(sq *Sqlite, br *utils.BootstrapResult) (err error) {
	logrus.Warn("> Setup_Filter_Generic")
	/*
		// Create filter
		f := types.Filter{
			Label: "generic",
		}
		err = sq.Filter.InsertFilter(&f)
		if err != nil {
			return
		}
		br.Add("filter:generic#id", fmt.Sprint(f.ID))
	*/
	return
}

func Setup_Config_FT_amd64(sq *Sqlite, br *utils.BootstrapResult) (err error) {
	//logrus.Warn("Setup_Config_FTarm")
	cfg := types.FTamd64Config()
	err = sq.Config.Insert(&cfg)
	if err != nil {
		return
	}
	br.Add("config:qnib/featuretest:amd64#id", fmt.Sprint(cfg.ID))
	br.Add("config:qnib/featuretest:amd64#digest", cfg.Digest)
	br.Add("config:qnib/featuretest:amd64#size", fmt.Sprint(cfg.Size))
	return

}
func Setup_Config_FTarm(sq *Sqlite, br *utils.BootstrapResult) (err error) {
	//logrus.Warn("Setup_Config_FTarm")
	cfg := types.FTarmConfig()
	err = sq.Config.Insert(&cfg)
	if err != nil {
		return
	}
	br.Add("config:qnib/featuretest:arm64#id", fmt.Sprint(cfg.ID))
	br.Add("config:qnib/featuretest:arm64#digest", cfg.Digest)
	br.Add("config:qnib/featuretest:arm64#size", fmt.Sprint(cfg.Size))
	return
}

func Setup_Manifest_QNIB_FeatureTest_amd(sq *Sqlite, br *utils.BootstrapResult) (err error) {
	//logrus.Warn("Setup_Maniest_QNIB_FeatureTest_arm")
	// Create cmanifest for qnib/featuretest
	mani := types.FTamdManifestData()
	// Upstream ID
	upID, err := br.GetInt64("upstream:qnib/featuretest:amd64#id")
	if err != nil {
		return err
	}
	mani.Upstream.ID = upID
	// ConfigID
	cfgID, err := br.GetInt64("config:qnib/featuretest:amd64#id")
	if err != nil {
		return err
	}
	mani.Config.ID = cfgID
	// Layers
	/// L0
	l0 := types.Layer{}
	if loID, err := br.GetInt64("layer:qnib/featuretest:amd64#id0"); err != nil {
		return err
	} else {
		l0.SqlID = loID
	}
	if v, err := br.Get("layer:qnib/featuretest:amd64#digest0"); err != nil {
		return err
	} else {
		l0.Digest = v
	}
	if v, err := br.GetInt64("layer:qnib/featuretest:amd64#size0"); err != nil {
		return err
	} else {
		l0.Size = v
	}
	mani.Layers = append(mani.Layers, l0)
	// / L0
	l1 := types.Layer{}
	if v, err := br.GetInt64("layer:qnib/featuretest:amd64#id1"); err != nil {
		return err
	} else {
		l1.SqlID = v
	}
	if v, err := br.Get("layer:qnib/featuretest:amd64#digest1"); err != nil {
		return err
	} else {
		l1.Digest = v
	}
	if v, err := br.GetInt64("layer:qnib/featuretest:amd64#size1"); err != nil {
		return err
	} else {
		l1.Size = v
	}
	mani.Layers = append(mani.Layers, l1)
	// Profile
	mani.Platform = types.Platform{
		Arch:    "amd64",
		OS:      "linux",
		Variant: "",
	}
	if v, err := br.GetInt64("platform:linux/amd64#id"); err != nil {
		return err
	} else {
		mani.Platform.ID = v
	}
	err = sq.Manifest.Insert(&mani)
	if err != nil {
		return
	}
	br.Add("manifest:qnib/featuretest:amd64#id", fmt.Sprint(mani.ID))
	br.Add("manifest:qnib/featuretest:amd64#digest", mani.Digest)
	return
}

func Setup_Manifest_QNIB_FeatureTest_arm(sq *Sqlite, br *utils.BootstrapResult) (err error) {
	//logrus.Warn("Setup_Maniest_QNIB_FeatureTest_arm")
	// Create cmanifest for qnib/featuretest
	mani := types.FTarmManifestData()
	// Upstream ID
	upID, err := br.GetInt64("upstream:qnib/featuretest:arm64#id")
	if err != nil {
		return err
	}
	mani.Upstream.ID = upID
	// ConfigID
	cfgID, err := br.GetInt64("config:qnib/featuretest:arm64#id")
	if err != nil {
		return err
	}
	mani.Config.ID = cfgID
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
		mani.Platform.ID = v
	}
	err = sq.Manifest.Insert(&mani)
	if err != nil {
		return
	}
	br.Add("manifest:qnib/featuretest:arm64#id", fmt.Sprint(mani.ID))
	br.Add("manifest:qnib/featuretest:arm64#digest", mani.Digest)
	return
}

func Setup_Profiles(sqObj *Sqlite, br *utils.BootstrapResult, d string) (err error) {
	logrus.Warn("Setup_Profiles")
	/*
		ctx := context.Background()
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
			//logrus.Info(msg)
			fID, success, err := sqObj.Filter.GetFilter(f)
			if err != nil {
				return err
			}
			if !success {
				err = sqObj.Filter.InsertFilter(&f)
				if err != nil {
					return err
				}
				fID = f.ID
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
				prfId = p.ID
			}
			br.Add(fmt.Sprintf("profile:%s#id", p.Name), fmt.Sprint(prfId))
			//logrus.Infof("Profile '%s' created with ID: %d", p.Name, prfId)
			err = sqObj.LinkProfileToFilter(ctx, prfId, fID)
			if err != nil {
				return err
			}
		}
	*/
	return
}

func Setup_Collection_QNIB_FeatureTest_Generic(sq *Sqlite, br *utils.BootstrapResult) (err error) {
	cTag := types.CollectionTagGeneric()
	err = sq.CollectionTag.Insert(&cTag)
	if err != nil {
		return
	}
	br.Add("collection_tag:generic#id", fmt.Sprint(cTag.ID))
	// Create collection
	c := types.FTGenericCollection()
	c.ColTag.ID = cTag.ID
	err = sq.Collection.Insert(&c)
	if err != nil {
		return
	}
	br.Add("collection:qnib/featuretest:generic#id", fmt.Sprint(c.ID))

	// Link Collection and ManifestList
	mlID, err := br.GetInt64("manifestlist:qnib/featuretest:arm64#id")
	if err != nil {
		return err
	}
	err = sq.Collection.InsertXTable(c.ID, cTag.ID, mlID)
	if err != nil {
		return
	}
	return
}

/*
	func Setup_Collection_QNIB_FeatureTestLatest(sq *Sqlite, br *utils.BootstrapResult) (err error) {
		//logrus.Info("> Setup_Collection_QNIB_FeatureTest")
		cTag := types.CollectionTagLatest()
		err = sq.CollectionTag.Insert(&cTag)
		if err != nil {
			return
		}
		br.Add("collection_tag:latest#id", fmt.Sprint(cTag.SqlID))
		// Create collection
		c := types.FTarmCollection()
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
	cTag := types.CollectionTagarm()
	err = sq.CollectionTag.Insert(&cTag)
	if err != nil {
		return
	}
	br.Add("collection_tag:arm64#id", fmt.Sprint(cTag.ID))
	// Create collection
	c := types.FTarmCollection()
	col, err := sq.GetInsertCollectionByRepoTag(c.Repo, cTag.Tag)
	if err != nil {
		return
	}
	br.Add("collection:qnib/featuretest:arm64#id", fmt.Sprint(col.ID))

	// Link Collection and ManifestList
	mlID, err := br.GetInt64("manifestlist:qnib/featuretest:arm64#id")
	if err != nil {
		return err
	}
	err = sq.Collection.InsertXTable(col.ID, cTag.ID, mlID)
	if err != nil {
		return
	}
	return
}

func Setup_Collection_QNIB_FeatureTest_amd64(sq *Sqlite, br *utils.BootstrapResult) (err error) {
	cTag := types.CollectionTagamd64()
	err = sq.CollectionTag.Insert(&cTag)
	if err != nil {
		return
	}
	br.Add("collection_tag:amd64#id", fmt.Sprint(cTag.ID))
	// Create collection
	c := types.FTamd64Collection()
	col, err := sq.GetInsertCollectionByRepoTag(c.Repo, cTag.Tag)
	if err != nil {
		return
	}
	br.Add("collection:qnib/featuretest:amd64#id", fmt.Sprint(col.ID))

	// Link Collection and ManifestList
	mlID, err := br.GetInt64("manifestlist:qnib/featuretest:amd64#id")
	if err != nil {
		return err
	}
	err = sq.Collection.InsertXTable(col.ID, cTag.ID, mlID)
	if err != nil {
		return
	}
	return
}

func Setup_Users(ctx context.Context, sqObj *Sqlite, br *utils.BootstrapResult) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func": "Setup_Users",
	})
	_ = l
	defer span.End()
	usrs := []config.User{
		{Username: "test", Email: "test@test.de", FirstName: "Max", LastName: "Mustermann", Password: "test", Role: "user"},
		{Username: "admin", Email: "admin@test.de", FirstName: "Admin", LastName: "Mustermann", Password: "admin", Role: "admin"},
	}
	for _, user := range usrs {
		scrt := types.Secret{
			Key:       user.Password,
			Name:      "test0",
			CreatedAt: time.Now(),
			IsCopied:  false,
		}

		u := types.User{
			Username:  user.Username,
			FirstName: user.FirstName,
			LastName:  user.LastName,
			Email:     user.Email,
			Role:      user.Role,
			Secrets: []types.Secret{
				scrt,
			},
		}
		if uID, err := sqObj.InsertUser(ctx, u); err != nil {
			return err
		} else {
			br.Add(fmt.Sprintf("user:%s#id", u.Username), fmt.Sprintf("%d", uID))
		}
		bearerToken := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", u.Username, user.Password)))
		br.Add(fmt.Sprintf("user:%s#bearer", u.Username), bearerToken)
		br.Add(fmt.Sprintf("user:%s#password", u.Username), user.Password)
	}
	return
}

func (s Sqlite) BootstrapTest(ctx context.Context, bs utils.Bootstrap) (br utils.BootstrapResult, err error) {
	br = utils.NewBootstrapResult()
	for _, b := range bs.Bootstraps {
		switch b {
		case "profiles":
			err = Setup_Profiles(&s, &br, "../../../misc/yaml/profiles")
			if err != nil {
				err = errors.Wrap(err, "Setup_Profiles")
				return
			}
		case "platforms":
			err = Setup_Platforms(&s, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Platforms")
				return
			}
		case "users":
			err = Setup_Users(ctx, &s, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Users")
				return
			}
		case "manifest:qnib/featuretest:arm64":
			err = Setup_Manifest_QNIB_FeatureTest_arm(&s, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Manifest_QNIB_FeatureTest_arm")
				return
			}
		case "manifest:qnib/featuretest:amd64":
			err = Setup_Manifest_QNIB_FeatureTest_amd(&s, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Manifest_QNIB_FeatureTest_amd")
				return
			}
		case "config:qnib/featuretest:arm64":
			err = Setup_Config_FTarm(&s, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Config_FTarm")
				return
			}
		case "config:qnib/featuretest:amd64":
			err = Setup_Config_FT_amd64(&s, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Config_FT_amd64")
				return
			}

		case "upstreams:qnib/featuretest:arm64":
			err = Setup_Upstream_FTarm(&s, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Upstream_FTarm")
				return
			}
		case "upstreams:qnib/featuretest:amd64":
			err = Setup_Upstream_FT_amd64(&s, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Upstream_FT_amd64")
				return
			}
		case "layers:qnib/featuretest:arm64":
			err = Setup_Layers_FTarm(&s, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Layers_FTarm")
				return
			}
		case "layers:qnib/featuretest:amd64":
			err = Setup_Layers_FT_amd64(&s, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Layers_FT_amd64")
				return
			}
		case "manifestlist:qnib/featuretest:amd64":
			err = Setup_Manifestlist_FTamd64(&s, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Manifestlist_FTamd64")
				return
			}

		case "manifestlist:qnib/featuretest:arm64":
			err = Setup_Manifestlist_FTarm(&s, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Manifestlist_FTarm")
				return
			}
		case "collection:qnib/featuretest:arm64":
			err = Setup_Collection_QNIB_FeatureTest_arm(&s, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Collection_QNIB_FeatureTest_arm")
				return
			}
		case "collection:qnib/featuretest:amd64":
			err = Setup_Collection_QNIB_FeatureTest_amd64(&s, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Collection_QNIB_FeatureTest_amd64")
				return
			}
		case "collection:qnib/featuretest:generic":
			err = Setup_Collection_QNIB_FeatureTest_Generic(&s, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Collection_QNIB_FeatureTest_Generic")
				return
			}
		case "filter:generic":
			err = Setup_Filter_Generic(&s, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Filter_Generic")
				return
			}
		default:
			err = fmt.Errorf("no Setup for '%s'", b)
			return br, err
		}
	}
	return
}
