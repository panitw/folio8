package forbiddenimportsfixture

import "runtime"

// compliantRuntimeGOOS references runtime.GOOS — the "runtime" import
// itself is not banned, only the Caller selector (RuleRuntimeCaller);
// this fixture proves the ban is selector-specific, not package-wide.
var compliantRuntimeGOOS = runtime.GOOS
