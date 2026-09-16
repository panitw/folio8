package designer

import "fmt"

// ComponentCommandError is the stable, bounded diagnostic seam for component
// mutations. It deliberately names only a paint-safe id and command field.
type ComponentCommandError struct {
	error
	ElementID string
	DataPath  string
	Message   string
}

// NewComponentCommandError is ComponentCommandError's only constructor. The
// embedded error is unexported, so the engine's componentFailure — which the
// module root still owns, with every call site — builds the value through here.
func NewComponentCommandError(id, path, message string) error {
	return &ComponentCommandError{error: fmt.Errorf("folio8: %s", message), ElementID: id, DataPath: path, Message: message}
}
