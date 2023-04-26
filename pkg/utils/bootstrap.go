package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/sirupsen/logrus"
)

type Bootstrap struct {
	CopySqlite    string
	Bootstraps    []string
	EmptyAll      bool
	PostTestSleep int
	YAMLPrefix    string
}

func NewBootstrap() Bootstrap {
	return Bootstrap{
		CopySqlite:    "",
		Bootstraps:    []string{},
		EmptyAll:      false,
		PostTestSleep: 0,
	}
}

func (bs *Bootstrap) SetEmptyAll() {
	bs.EmptyAll = true
}

func (bs *Bootstrap) SetSqlite(p string) {
	h, err := os.UserHomeDir()
	if err != nil {
		logrus.Fatal(err)
	}
	bs.CopySqlite = filepath.Join(h, p)
}

func (bs *Bootstrap) AddBootstrap(b string) {
	bs.Bootstraps = append(bs.Bootstraps, b)
}

func (bs *Bootstrap) SetYAMLPrefix(pre string) {
	bs.YAMLPrefix = pre
}

// / Result
type BootstrapResult struct {
	Dict map[string]string
}

func (br BootstrapResult) PrintDict() {
	for _, l := range br.ToLines() {
		logrus.Info(l)
	}
}

func (br BootstrapResult) ToLines() []string {
	keys := []string{}
	for k := range br.Dict {
		keys = append(keys, k)
	}
	res := []string{}
	sort.Strings(keys)
	for _, k := range keys {
		res = append(res, fmt.Sprintf("'%-20s' : '%s'", k, br.Dict[k]))
	}
	return res
}

func NewBootstrapResult() BootstrapResult {
	return BootstrapResult{
		Dict: map[string]string{},
	}
}

func (bs *BootstrapResult) Add(k, v string) {
	//logrus.Infof("Add %-40s: '%s'", k, v)
	if _, ok := bs.Dict[k]; ok {
		if bs.Dict[k] != v {
			logrus.Warnf("Overwrite key '%s' : '%s' with '%s'", k, bs.Dict[k], v)
			bs.Dict[k] = v
		}
		// Else is skipped as the value is the same
	} else {
		bs.Dict[k] = v
	}
}

func (bs *BootstrapResult) Get(k string) (string, error) {
	v, ok := bs.Dict[k]
	if !ok {
		return "", fmt.Errorf("key '%s' not found", k)
	}
	return v, nil
}

func (bs *BootstrapResult) GetInt(k string) (int, error) {
	v, err := bs.Get(k)
	if err != nil {
		return 0, err
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		logrus.Warnf("Failed to convert key '%s' : '%s' to int: %v", k, v, err)
		return i, err
	}
	return i, nil
}

func (bs *BootstrapResult) GetInt64(k string) (int64, error) {
	i, err := bs.GetInt(k)
	if err != nil {
		return 0, err
	}
	return int64(i), nil
}
