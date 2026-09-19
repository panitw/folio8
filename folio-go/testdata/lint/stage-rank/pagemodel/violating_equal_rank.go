// Package pagemodel is the retained violating fixture for the EQUAL-rank
// half of the rule, which a strictly-lower table has and a simple
// "forward only" reading does not: pagemodel and diag are both rank 1,
// so neither may import the other and the two stay independent. Without
// this fixture the equal-rank branch of ScanStageRank would never be
// exercised by any caller.
package pagemodel

import "github.com/panitw/folio8/folio-go/internal/diag"

func Report() { _ = diag.Note{} }
