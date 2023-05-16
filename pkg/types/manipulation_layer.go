package types

type ManipuLayer struct {
	ID     int64
	Label  string
	Image  Upstream
	Layers []Layer
	Rule   string
}
