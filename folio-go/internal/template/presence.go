package template

// Presence distinguishes a field that is absent, a field that is
// explicitly `null`, and a field that is present with a value
// (D-1.4.8, corrected). A bare pointer cannot do this — `*int` is nil
// for both `{}` and `{"a":null}` (measured, M-2) — so this wrapper
// carries its own Set/Null flags instead of relying on the zero value
// of Value to mean "absent".
//
// Story 1.4's own decoder never runs encoding/json's UnmarshalJSON
// machinery on Presence directly — every Presence field is populated by
// hand in parse.go, which already knows, from a map[string]json.RawMessage
// lookup, whether the key was present at all and whether its raw bytes
// are the literal `null` (D-1.4.8: "only the unmarshaller knows it was
// called" — here, decodeObject IS that unmarshaller).
type Presence[T any] struct {
	Set   bool // the key was present in the object at all
	Null  bool // the key was present and its value was JSON null
	Value T    // meaningful only when Set && !Null
}

// present builds a Presence carrying a real value.
func present[T any](v T) Presence[T] {
	return Presence[T]{Set: true, Value: v}
}

// presentNull builds a Presence recording an explicit JSON null.
func presentNull[T any]() Presence[T] {
	return Presence[T]{Set: true, Null: true}
}

// absent is the zero value: Presence[T]{}. Named for readability at call
// sites; it is not otherwise distinct from the zero value.
func absent[T any]() Presence[T] {
	return Presence[T]{}
}
