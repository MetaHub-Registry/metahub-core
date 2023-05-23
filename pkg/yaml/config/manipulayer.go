package config

type ManipuLayer struct {
	Image       string `yaml:"image"`
	Rule        string `yaml:"rule"`
	Cmd         []Cmd
	Environment []Environment
}
