package types

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

/*
   PRETTY_NAME="Ubuntu 22.04.1 LTS"
   NAME="Ubuntu"
   VERSION_ID="22.04"
   VERSION="22.04.1 LTS (Jammy Jellyfish)"
   VERSION_CODENAME=jammy
   ID=ubuntu
   ID_LIKE=debian
   HOME_URL="https://www.ubuntu.com/"
   SUPPORT_URL="https://help.ubuntu.com/"
   BUG_REPORT_URL="https://bugs.launchpad.net/ubuntu/"
   PRIVACY_POLICY_URL="https://www.ubuntu.com/legal/terms-and-policies/privacy-policy"
   UBUNTU_CODENAME=jammy
*/

type OS struct {
	Major    int
	Minor    int
	Patch    int
	Name     string
	ID       string
	IDLike   string
	Codename string
}

func NewOS() OS {
	return OS{
		Major:    0,
		Minor:    0,
		Patch:    0,
		Name:     "",
		ID:       "",
		IDLike:   "",
		Codename: "",
	}
}
func ParseOS(fcont string) (os OS, err error) {
	os = NewOS()
	for _, l := range strings.Split(fcont, "\n") {
		spl := strings.Split(l, "=")
		if len(spl) != 2 {
			continue
		}
		v := strings.Trim(spl[1], "\"")
		switch spl[0] {
		case "NAME":
			os.Name = v
		case "ID":
			os.ID = v
		case "ID_LIKE":
			os.IDLike = v
		case "VERSION_ID":
			os.Major, os.Minor, os.Patch, err = ParseVersion(v)
			if err != nil {
				err = errors.Wrap(err, fmt.Sprintf("failed to parse version %s", v))
				return
			}
		case "VERSION_CODENAME":
			os.Codename = v
		}

	}
	return
}

func ParseVersion(v string) (major, minor, patch int, err error) {
	major = 0
	minor = 0
	patch = 0
	spl := strings.Split(v, ".")
	if len(spl) > 0 {
		major, err = strconv.Atoi(spl[0])
		if err != nil {
			err = errors.Wrap(err, fmt.Sprintf("failed to parse major %s", v))
			return
		}
	}
	if len(spl) > 1 {
		minor, err = strconv.Atoi(spl[1])
		if err != nil {
			err = errors.Wrap(err, fmt.Sprintf("failed to parse minor %s", v))
			return
		}
	}
	if len(spl) > 2 {
		patch, err = strconv.Atoi(spl[2])
		if err != nil {
			err = errors.Wrap(err, fmt.Sprintf("failed to parse patch %s", v))
			return
		}
	}
	return
}

func (os OS) GetVersion() string {
	res := fmt.Sprintf("%d", os.Major)
	if os.Minor > 0 {
		res = fmt.Sprintf("%s.%d", res, os.Minor)
	}
	if os.Patch > 0 {
		res = fmt.Sprintf("%s.%d", res, os.Patch)
	}
	return res
}
