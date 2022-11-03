package types

import "fmt"

type Platform struct {
	NeoID    int64  `json:"neo_id"`
	SqlID    int64  `json:"sql_id"`
	Arch     string `json:"architecture"`
	OS       string `json:"os"`
	Variant  string `json:"variant"`
	Features string `json:"features"`
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
