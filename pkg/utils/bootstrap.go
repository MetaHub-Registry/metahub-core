package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/sirupsen/logrus"
)

type Bootstrap struct {
	CopySqlite  string
	Bootstraps  []string
	SqlEmptyAll bool
}

func NewBootstrap() Bootstrap {
	return Bootstrap{
		CopySqlite:  "",
		Bootstraps:  []string{},
		SqlEmptyAll: false,
	}
}

func (bs *Bootstrap) SetSqlEmptyAll() {
	bs.SqlEmptyAll = true
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

// / Result
type BootstrapResult struct {
	Dict map[string]string
}

func NewBootstrapResult() BootstrapResult {
	return BootstrapResult{
		Dict: map[string]string{},
	}
}

func (bs *BootstrapResult) Add(k, v string) {
	//logrus.Infof("Add %-40s: '%s'", k, v)
	bs.Dict[k] = v
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
