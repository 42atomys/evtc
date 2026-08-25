package timeline

import (
	"sync"
	"time"

	"github.com/42atomys/evtc"
)

// ExtensionHealingStats is the signature of the healing stats addon, the
// extension that logs heals and barrier as combat events. The
// extensions/healingstats package of this module decodes them.
const ExtensionHealingStats uint32 = 0x9c9b3c99

// Extension is an arcdps extension that registered in the log. arcdps does
// not manage extension data: the registration event and the combat events
// the extension wrote are kept as logged and their meaning belongs to the
// extension. A package that knows an extension registers an
// ExtensionDecoder for its signature, and Build keeps what the decoder
// built in Decoded.
type Extension struct {
	// Time is the time of the registration.
	Time time.Duration
	// Signature identifies the extension: the low 32 bits of the src_agent
	// field of its registration event, repeated in the pad61 to pad64
	// bytes of every combat event it writes.
	Signature uint32
	// Version is the text the extension wrote in the dst_agent field of its
	// registration event, empty when those bytes are not printable.
	Version string
	// Event is the registration event.
	Event *evtc.Event
	// Timeline owns the extension.
	Timeline *Timeline
	// Decoded is what the ExtensionDecoder registered for the signature
	// built from the events of the extension, nil when no decoder is
	// registered or when the extension is a later registration of a
	// signature. The package of the decoder names its type and offers a
	// typed accessor, such as healingstats.Of.
	Decoded any

	events []*evtc.Event
}

// Events returns the combat events the extension wrote, in time order.
// When a signature registered more than once, its events belong to the
// first registration.
func (x *Extension) Events() Events { return Events{Query: From(x.events), tl: x.Timeline} }

// ExtensionDecoder decodes the combat events of one extension into a graph
// of its own, so that a package outside timeline adds the nodes of the
// extension on top of the core graph without changing it. Build calls the
// decoder registered for the signature of every extension that registered
// in the log, once the core graph is complete, and keeps the result in
// Extension.Decoded.
type ExtensionDecoder interface {
	// Signature returns the extension signature the decoder handles.
	Signature() uint32
	// Decode builds the graph of the extension from its registration event
	// and the combat events it wrote, both reachable from x, and returns
	// it. The timeline of x is complete and read-only at that point; the
	// skills the events name are in it.
	Decode(x *Extension) any
}

// RegisterExtension registers a decoder for the signature it reports; a
// later registration for the same signature replaces the earlier one. A
// package implementing a decoder calls it from its init function, so that
// importing the package is enough for Build to decode the extension. It
// panics on a nil decoder.
func RegisterExtension(d ExtensionDecoder) {
	if d == nil {
		panic("timeline: RegisterExtension with a nil decoder")
	}
	extensionDecoders.Store(d.Signature(), d)
}

// extensionDecoders holds the registered decoders by signature.
var extensionDecoders sync.Map

// extensionDecoder returns the decoder registered for a signature, nil
// when there is none.
func extensionDecoder(sig uint32) ExtensionDecoder {
	if d, ok := extensionDecoders.Load(sig); ok {
		return d.(ExtensionDecoder)
	}
	return nil
}
