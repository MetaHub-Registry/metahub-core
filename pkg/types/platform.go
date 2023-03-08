package types

import (
	"fmt"

	"github.com/sirupsen/logrus"

	"regexp"
)

type Platform struct {
	ID      int64  `json:"-"`
	Arch    string `json:"architecture"`
	OS      string `json:"os"`
	Variant string `json:"variant"`
}

func (p Platform) GetLabel() string {
	l := fmt.Sprintf("%s/%s", p.OS, p.Arch)
	if p.Variant != "" {
		l = fmt.Sprintf("%s/%s", l, p.Variant)
	}
	return l
}

func (p Platform) GetTag() string {
	l := fmt.Sprintf("%s.%s", p.OS, p.Arch)
	if p.Variant != "" {
		l = fmt.Sprintf("%s.%s", l, p.Variant)
	}
	return l
}

func (p Platform) EvalRule(r Rule) bool {
	return p.evalRule(r.Op, r.Key, r.Value)
}

// TODO: Needs to become an interface with more complex annotations
func (p Platform) evalRule(op, k, v string) bool {
	switch op {
	case "eq":
		switch k {
		case "os":
			return p.OS == v
		case "arch":
			return p.Arch == v
		case "variant":
			return p.Variant == v
		default:
			logrus.Errorf("unknown key '%s' for platform rule", k)
			return false
		}
	case "ne":
		switch k {
		case "os":
			return p.OS != v
		case "arch":
			return p.Arch != v
		case "variant":
			return p.Variant != v
		default:
			logrus.Errorf("unknown key '%s' for platform rule", k)
			return false
		}
	case "re":
		// regex
		re := regexp.MustCompile(v)
		switch k {
		case "os":
			return re.MatchString(p.OS)
		case "arch":
			return re.MatchString(p.Arch)
		case "variant":
			return re.MatchString(p.Variant)
		default:
			logrus.Errorf("unknown key '%s' for platform rule", k)
			return false
		}
	}
	return false
}
