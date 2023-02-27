package config

import (
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func FindYamlFiles(cacheDir string) (files []string, err error) {
	files = []string{}
	for _, ext := range []string{"*.yml", "*.yaml"} {
		newFiles, err := utils.RecursiveFind(cacheDir, ext, false)
		if err != nil {
			logrus.Errorf("Warning, issue looking for yaml file %s: %x\n", ext, err)
			return files, err
		}
		//l.Debugf("Add %d files with extension %s\n", len(files), newFiles)
		files = append(files, newFiles...)
	}
	return files, nil
}
