package types

type ManipuLayer struct {
	ID          int64
	Label       string
	Image       ManifestData
	Rule        string
	Cmd         []ManipuCmd
	Environment []ManipuEnv
}
