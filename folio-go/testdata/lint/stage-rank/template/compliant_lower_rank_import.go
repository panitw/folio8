// Package template is a COMPLIANT fixture: template(2) importing
// geom(0) is a strictly-lower rank and must NOT be reported. Its
// presence is what makes the fixture assertion two-sided — an
// over-eager scanner that flagged every internal import would fail here
// rather than passing the violating half and looking correct.
package template

import "github.com/panitw/folio8/folio-go/internal/geom"

func Width() geom.Length { return 0 }
