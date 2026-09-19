package designer

// ComponentMove is engine-owned geometry evidence, in document millipoints.
type ComponentMove struct {
	DX int64 `json:"dx"`
	DY int64 `json:"dy"`
}
