package folio8

import (
	"fmt"

	"github.com/panitw/folio8/folio-go/internal/designer"
)

// The designer surface is unexported here and reached only through
// internal/designer's function variables, which this init assigns. See that
// package's doc comment for why the implementations stay in this package.
func init() {
	designer.Canvas = func(tpl any) (designer.CanvasProjection, error) {
		return canvas(designerTemplate(tpl))
	}
	designer.CanvasWithTextPaint = func(tpl any, fonts map[string][]byte) (designer.CanvasProjection, error) {
		return canvasWithTextPaint(designerTemplate(tpl), fonts)
	}
	designer.ApplyComponentCommand = func(tpl any, command []byte, fonts ...map[string][]byte) (designer.CanvasProjection, error) {
		return applyComponentCommand(designerTemplate(tpl), command, designerFontSets(fonts)...)
	}
	designer.ApplyPageSetupCommand = func(tpl any, command []byte) (designer.CanvasProjection, error) {
		return applyPageSetupCommand(designerTemplate(tpl), command)
	}
	designer.PreviewComponentMove = func(tpl any, command []byte, fonts ...map[string][]byte) (designer.ComponentMove, error) {
		return previewComponentMove(designerTemplate(tpl), command, designerFontSets(fonts)...)
	}
	designer.TableColumns = func(tpl any, tableID string) (designer.TableColumnsProjection, error) {
		return tableColumns(designerTemplate(tpl), tableID)
	}
	designer.PreviewIdentity = func(template, data, params []byte, fontSet map[string][]byte) string {
		return previewIdentity(template, data, params, fontSet)
	}
	designer.AssetBytes = func(tpl any, key string) ([]byte, string, error) {
		return assetBytes(designerTemplate(tpl), key)
	}
	designer.StandInData = func(tpl any) ([]byte, error) {
		return standInData(designerTemplate(tpl))
	}
	designer.CarriedCommands = func(command []byte) ([][]byte, bool) {
		members, unit, err := carriedCommands(command)
		if err != nil {
			// A unit whose member list cannot be read carries no members a
			// caller can inspect — but it is still a unit, and saying so is
			// what stops the caller reporting the door's refusal in its own
			// words.
			return nil, true
		}
		return members, unit
	}
}

// designerTemplate recovers the *Template a designer caller holds. A nil
// handle — untyped or a nil *Template — passes through as a nil *Template, so
// each function keeps its own nil refusal.
func designerTemplate(tpl any) *Template {
	if tpl == nil {
		return nil
	}
	t, ok := tpl.(*Template)
	if !ok {
		panic(fmt.Sprintf("folio8: designer bridge expects a *folio8.Template, got %T", tpl))
	}
	return t
}

// designerFontSets converts the bridge's variadic font maps to FontSets
// without copying any map.
func designerFontSets(fonts []map[string][]byte) []FontSet {
	if fonts == nil {
		return nil
	}
	out := make([]FontSet, len(fonts))
	for i, f := range fonts {
		out[i] = f
	}
	return out
}
