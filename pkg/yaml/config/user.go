package config

import (
	"context"
	"io/ioutil"
	"log"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"gopkg.in/yaml.v3"
)

type User struct {
	Username  string `yaml:"username"`
	FirstName string `yaml:"first_name"`
	LastName  string `yaml:"last_name"`
	Email     string `yaml:"email"`
	Password  string `yaml:"password"`
	Hash      string `yaml:"hash"`
}

// readUser reads a user config
func readUser(yamlStr []byte) User {

	// First unmarshall into generic structure
	u := User{}

	err := yaml.Unmarshal(yamlStr, &u)
	if err != nil {
		log.Fatalf("Unmarshal: %v\n", err)
	}
	if err != nil {
		log.Fatalf("Parsing Config: %v\n", err)
	}
	return u
}

func LoadUser(yamlfile string) User {
	yamlContent, err := ioutil.ReadFile(yamlfile)
	if err != nil {
		logrus.Errorf("Cannot load user #%v ", err)
	}
	return readUser(yamlContent)
}

func FindUsers(ctx context.Context, cacheDir string) (users []User, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package":  "config",
		"func":     "FindUsers",
		"cacheDir": cacheDir,
	})
	tr := otel.Tracer("config")
	ctx, span := tr.Start(ctx, "FindUsers")
	_ = ctx
	span.SetAttributes(attribute.Key("path").String(cacheDir))
	defer span.End()
	users = make([]User, 0)
	files := []string{}

	for _, ext := range []string{"*.yml", "*.yaml"} {
		newFiles, err := utils.RecursiveFind(cacheDir, ext, false)
		if err != nil {
			l.Errorf("Warning, issue looking for profiles %s: %x\n", ext, err)
			continue
		}
		//l.Debugf("Add %d files with extension %s\n", len(files), newFiles)
		files = append(files, newFiles...)
	}
	// Read in each manifest config
	for _, ufile := range files {
		u := LoadUser(ufile)
		fname := filepath.Base(ufile)
		fext := filepath.Ext(fname)
		bname := strings.TrimSuffix(fname, fext)
		if bname != u.Username {
			logrus.Errorf("User file '%s' does not match username %s", ufile, u.Username)
			continue
		}
		users = append(users, u)
	}
	return
}
