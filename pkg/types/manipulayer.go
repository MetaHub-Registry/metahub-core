package types

type ManipuLayer struct {
	ID          int64
	Label       string
	Image       Manifest
	Rule        string
	Cmd         []ManipuCmd
	Environment []ManipuEnv
}
