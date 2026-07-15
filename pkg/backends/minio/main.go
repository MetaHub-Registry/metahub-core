package minio

import (
	"context"
	"fmt"
	"log"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/mitchellh/go-homedir"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/metahub-registry/metahub-core/pkg/tracing"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

const (
	blobBucketKey      = "blobs"
	manifestsBucketKey = "manifests"
	pkgName            = "backends/minio"
)

func GetBlobBucketKey() string {
	return blobBucketKey
}

func GetManifestsBucketKey() string {
	return manifestsBucketKey
}

type BucketConfig struct {
	Client     *minio.Client
	ConsoleURL string
	URL        string
	Region     string
	Bucket     string
	User       string
	Password   string
	Redirect   bool
	Ssl        bool
	// PublicURL / PublicSsl address the client-reachable endpoint used only to
	// SIGN presigned URLs; internal object ops use Client (URL/Ssl). Empty
	// PublicURL means "same as internal" (PresignClient == Client).
	PublicURL     string
	PublicSsl     bool
	PresignClient *minio.Client
}

func (m Minio) BucketConfigString() string {
	res := []string{}
	for k, v := range m.BucketConfig {
		if v.Ssl {
			res = append(res, fmt.Sprintf("%s: https://%s / https://%s", k, v.ConsoleURL, v.URL))
		} else {
			res = append(res, fmt.Sprintf("%s: http://%s / http://%s", k, v.ConsoleURL, v.URL))
		}
	}
	return strings.Join(res, " | ")
}

type MinioConfig struct {
	Tracing tracing.TraceCfg
}

type Minio struct {
	BucketConfig map[string]BucketConfig
	Config       MinioConfig
}

func NewMiniWithCfg(cfg map[string]BucketConfig) (m Minio, err error) {
	m = Minio{
		BucketConfig: cfg,
	}
	return
}

// viperStr returns the string at key when set, else fallback. Extracted so the
// per-bucket resolution in NewMinio stays flat (low cyclomatic complexity).
func viperStr(key, fallback string) string {
	if viper.IsSet(key) {
		return viper.GetString(key)
	}
	return fallback
}

// viperBool returns the bool at key when set, else fallback.
func viperBool(key string, fallback bool) bool {
	if viper.IsSet(key) {
		return viper.GetBool(key)
	}
	return fallback
}

func NewMinio(cfgFile string) (m Minio, err error) {
	m = Minio{
		BucketConfig: map[string]BucketConfig{},
	}
	viper.SetConfigType("yaml")
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".cobra-example" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".metahub-admin")
		viper.SetConfigType("yaml")
	}
	m.BucketConfig = map[string]BucketConfig{
		blobBucketKey: {
			Bucket: blobBucketKey,
		},
		manifestsBucketKey: {
			Bucket: manifestsBucketKey,
		},
	}
	bc := BucketConfig{
		URL:       viper.GetString("minio.url"),
		Region:    viper.GetString("minio.region"),
		User:      viper.GetString("minio.user"),
		Password:  viper.GetString("minio.password"),
		Redirect:  viper.GetBool("minio.redirect"),
		Ssl:       viper.GetBool("minio.ssl"),
		PublicURL: viper.GetString("minio.public_url"),
		PublicSsl: viper.GetBool("minio.public_ssl"),
	}

	for _, v := range reflect.ValueOf(m.BucketConfig).MapKeys() {
		b := fmt.Sprintf("%s", v)
		p := "minio." + b + "."
		bCfg := BucketConfig{
			Bucket:   viperStr(p+"bucket", bc.Bucket),
			URL:      viperStr(p+"url", bc.URL),
			Region:   viperStr(p+"region", bc.Region),
			User:     viperStr(p+"user", bc.User),
			Password: viperStr(p+"password", bc.Password),
			Redirect: viperBool(p+"redirect", bc.Redirect),
			Ssl:      viperBool(p+"ssl", bc.Ssl),
		}
		// Public presign endpoint: per-bucket key, then global minio.public_url,
		// then fall back to the internal URL (identity — no separate presign host).
		pubURL := bc.PublicURL
		if pubURL == "" {
			pubURL = bCfg.URL
		}
		bCfg.PublicURL = viperStr(p+"public_url", pubURL)
		pubSsl := bCfg.Ssl
		if viper.IsSet("minio.public_ssl") {
			pubSsl = bc.PublicSsl
		}
		bCfg.PublicSsl = viperBool(p+"public_ssl", pubSsl)
		m.BucketConfig[b] = bCfg
	}
	return
}

func (m Minio) ConnectClients() (err error) {
	for _, v := range reflect.ValueOf(m.BucketConfig).MapKeys() {
		b := fmt.Sprintf("%s", v)
		var bCfg BucketConfig
		if bc, ok := m.BucketConfig[b]; !ok {
			logrus.Errorf("Bucket '%s' not found in config", b)
		} else {
			bCfg = bc
		}

		logrus.Infof("Connecting to bucket '%s' in '%s'", bCfg.Bucket, bCfg.URL)

		cli, e := minio.New(bCfg.URL, &minio.Options{
			Creds:  credentials.NewStaticV4(bCfg.User, bCfg.Password, ""),
			Secure: bCfg.Ssl,
			Region: bCfg.Region,
		})
		if e != nil {
			logrus.Errorf("Error creating minio client: %v\n %s\n", bCfg, e)
			err = e
			return
		}
		bCfg.Client = cli
		// Build a dedicated presign client bound to the PUBLIC endpoint. Presigning
		// is offline (no connection), so this client only needs endpoint+creds+region.
		// When the public endpoint matches the internal one, reuse the same client.
		if bCfg.PublicURL == "" || (bCfg.PublicURL == bCfg.URL && bCfg.PublicSsl == bCfg.Ssl) {
			bCfg.PresignClient = cli
		} else {
			pcli, pe := minio.New(bCfg.PublicURL, &minio.Options{
				Creds:  credentials.NewStaticV4(bCfg.User, bCfg.Password, ""),
				Secure: bCfg.PublicSsl,
				Region: bCfg.Region,
			})
			if pe != nil {
				logrus.Errorf("Error creating minio presign client: %v\n %s\n", bCfg, pe)
				err = pe
				return
			}
			bCfg.PresignClient = pcli
		}
		m.BucketConfig[b] = bCfg
	}
	return
}

func (m Minio) BucketCheck() (err error) {
	ctx := context.Background()
	for _, bc := range m.BucketConfig {
		exists, err := bc.Client.BucketExists(ctx, bc.Bucket)
		if err != nil {
			logrus.Errorf("Error checking bucket %s: %s\n", bc.Bucket, err)
			return err
		}
		if !exists {
			logrus.Infof("Creating bucket '%s'", bc.Bucket)
			err = bc.Client.MakeBucket(ctx, bc.Bucket, minio.MakeBucketOptions{Region: bc.Region})
			if err != nil {
				logrus.Errorf(err.Error())
				return err
			}
		} else {
			logrus.Debugf("Bucket '%s' exists", bc.Bucket)
		}
	}
	return
}

func (m Minio) PreTearDown(bs utils.Bootstrap) {
	if bs.PostTestSleep > 0 {
		logrus.Warnf("Minio: %s", m.BucketConfigString())
	}
}

func (m Minio) EmptyAll() (err error) {
	// INSTRUMENTATION (perf-debug): a bucket-wide wipe is a prime suspect for the
	// re-uploaded model blobs — make it impossible to miss in a run's logs.
	logrus.Warn("EmptyAll CALLED — emptying ALL buckets (bulk object delete)")
	for _, bc := range m.BucketConfig {
		logrus.Warnf(">> EmptyAll emptying bucket '%s'", bc.Bucket)
		objectsCh := make(chan minio.ObjectInfo)

		// Send object names that are needed to be removed to objectsCh
		go func() {
			defer close(objectsCh)
			// List all objects from a bucket-name with a matching prefix.
			for object := range bc.Client.ListObjects(context.Background(), bc.Bucket, minio.ListObjectsOptions{}) {
				if object.Err != nil {
					log.Fatalln(object.Err)
				}
				objectsCh <- object
			}
		}()

		opts := minio.RemoveObjectsOptions{
			GovernanceBypass: true,
		}

		for rErr := range bc.Client.RemoveObjects(context.Background(), bc.Bucket, objectsCh, opts) {
			fmt.Println("Error detected during deletion: ", rErr)
		}
	}
	return
}

func (m Minio) TearDown(bs utils.Bootstrap) (err error) {
	m.PreTearDown(bs)
	if bs.PostTestSleep > 0 {
		time.Sleep(time.Duration(bs.PostTestSleep) * time.Second)
	}
	for _, b := range bs.Bootstraps {
		err = m.TeardownTest(b)
		if err != nil {
			logrus.Error(err)
			return
		}
	}
	return
}

func (min Minio) TeardownTest(bootstrap string) (err error) {
	logrus.Info("> TeardownTest")
	err = min.EmptyAll()
	if err != nil {
		logrus.Fatal(err)
	}
	switch bootstrap {
	case "":
	default:
		logrus.Warnf("no teardown for '%s'", bootstrap)
	}
	return
}
