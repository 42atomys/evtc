package combine

import (
	"github.com/42atomys/evtc/timeline"
)

// named counts the casts of a timeline whose caster the log names. A log
// that writes the end of a cast without address holds that end as the
// cast of nobody.
func named(tl *timeline.Timeline) int {
	return tl.Casts().Where(func(c *timeline.Cast) bool { return c.Caster != tl.Unknown }).Count()
}
