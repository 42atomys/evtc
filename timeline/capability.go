package timeline

import (
	"slices"

	"github.com/42atomys/evtc"
)

// survey asks the log once what it can carry and what is wrong with it, so
// that Has costs a search whatever the way the log was made.
func (tl *Timeline) survey() {
	tl.capabilities = slices.Clip(tl.Log.Capabilities())
	tl.warnings = slices.Clip(tl.Log.Warnings())
	// FirstBuild is 0 past the last capability.
	for c := evtc.Capability(0); c.FirstBuild() != 0; c++ {
		if _, ok := slices.BinarySearch(tl.capabilities, c); !ok {
			tl.missing = append(tl.missing, c)
		}
	}
	tl.missing = slices.Clip(tl.missing)
}

// warned reports whether the log has a warning of that code.
func (tl *Timeline) warned(code evtc.WarningCode) bool {
	return slices.ContainsFunc(tl.warnings, func(w evtc.Warning) bool { return w.Code == code })
}

// Has reports whether the log can carry c, by the arcdps build that wrote
// it or by what its events prove (see evtc.Log.Has). When it reports
// false, an empty result proves nothing: without evtc.CapabilityStealth,
// no stealth span does not mean nobody used stealth.
func (tl *Timeline) Has(c evtc.Capability) bool {
	_, ok := slices.BinarySearch(tl.capabilities, c)
	return ok
}

// Capabilities lists what the log can carry, in the order of the
// constants. The slice must not be modified.
func (tl *Timeline) Capabilities() []evtc.Capability { return tl.capabilities }

// Missing lists what the log cannot carry, in the order of the constants.
// The slice must not be modified.
func (tl *Timeline) Missing() []evtc.Capability { return tl.missing }

// Warnings lists what is known to be wrong with the log (see
// evtc.Log.Warnings), or nil. The messages arcdps wrote itself are in
// Integrity. The slice must not be modified.
func (tl *Timeline) Warnings() []evtc.Warning { return tl.warnings }
