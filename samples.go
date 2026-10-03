// Package samples embeds the sample fixtures the setup wizard runs on.
// The desktop GUI ships as a standalone binary outside any source
// checkout — the wizard's demo dry-run and simulation must work there,
// so the fixture bytes ride inside the binary. Byte-identical to
// testdata/fixtures/sendgrid/sample-export.json and
// testdata/simulate/plan.json; CI gates on the originals.
package samples

import _ "embed"

//go:embed testdata/fixtures/sendgrid/sample-export.json
var SendgridSampleExport []byte

//go:embed testdata/simulate/plan.json
var SimulateSamplePlan []byte
