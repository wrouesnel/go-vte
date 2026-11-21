package vte

import (
	"math"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testSequence interface {
	sequence()
}

type testCsiSequence struct {
	params        [][]uint16
	intermediates []byte
	ignore        bool
	rune          rune
}

func (testCsiSequence) sequence() {}

type testOscSequence struct {
	params [][]byte
	bell   bool
}

func (testOscSequence) sequence() {}

type testEscSequence struct {
	intermediates []byte
	ignore        bool
	b             byte
}

func (testEscSequence) sequence() {}

type testDcsHookSequence struct {
	params        [][]uint16
	intermediates []byte
	ignore        bool
	rune          rune
}

func (testDcsHookSequence) sequence() {}

type testDcsPutSequence byte

func (testDcsPutSequence) sequence() {}

type testDcsUnhookSequence struct{}

func (testDcsUnhookSequence) sequence() {}

type testSosPmApcSequence struct {
	kind           SosPmApcKind
	data           []byte
	bellTerminated bool
}

func (testSosPmApcSequence) sequence() {}

type testDispatcher struct {
	dispatched []testSequence
}

// The parser's slices are only good during a call: a dispatcher keeping them copies them.
func cloneParams(params [][]uint16) [][]uint16 {
	if params == nil {
		return nil
	}
	out := make([][]uint16, len(params))
	for i, p := range params {
		out[i] = append([]uint16{}, p...)
	}
	return out
}

func cloneBytes(b []byte) []byte {
	if b == nil {
		return nil
	}
	return append([]byte{}, b...)
}

func cloneOscParams(params [][]byte) [][]byte {
	if params == nil {
		return nil
	}
	out := make([][]byte, len(params))
	for i, p := range params {
		out[i] = cloneBytes(p)
	}
	return out
}

var _ Performer = &testDispatcher{}

// CsiDispatch implements Performer.
func (d *testDispatcher) CsiDispatch(params [][]uint16, intermediates []byte, ignore bool, r rune) {
	d.dispatched = append(d.dispatched, testCsiSequence{
		params:        cloneParams(params),
		intermediates: cloneBytes(intermediates),
		ignore:        ignore,
		rune:          r,
	})
}

// EscDispatch implements Performer.
func (d *testDispatcher) EscDispatch(intermediates []byte, ignore bool, b byte) {
	d.dispatched = append(d.dispatched, testEscSequence{
		intermediates: cloneBytes(intermediates),
		ignore:        ignore,
		b:             b,
	})
}

// Execute implements Performer.
func (*testDispatcher) Execute(b byte) {}

// Hook implements Performer.
func (d *testDispatcher) Hook(params [][]uint16, intermediates []byte, ignore bool, r rune) {
	d.dispatched = append(d.dispatched, testDcsHookSequence{
		params:        cloneParams(params),
		intermediates: cloneBytes(intermediates),
		ignore:        ignore,
		rune:          r,
	})
}

// OscDispatch implements Performer.
func (d *testDispatcher) OscDispatch(params [][]byte, bellTerminated bool) {
	d.dispatched = append(d.dispatched, testOscSequence{
		params: cloneOscParams(params),
		bell:   bellTerminated,
	})
}

// Print implements Performer.
func (*testDispatcher) Print(r rune) {}

// Put implements Performer.
func (d *testDispatcher) Put(b byte) {
	d.dispatched = append(d.dispatched, testDcsPutSequence(b))
}

// Unhook implements Performer.
func (d *testDispatcher) Unhook() {
	d.dispatched = append(d.dispatched, testDcsUnhookSequence{})
}

// SosPmApcDispatch implements Performer.
func (d *testDispatcher) SosPmApcDispatch(kind SosPmApcKind, data []byte, bellTerminated bool) {
	d.dispatched = append(d.dispatched, testSosPmApcSequence{
		kind:           kind,
		data:           cloneBytes(data),
		bellTerminated: bellTerminated,
	})
}

func TestSaddu16(t *testing.T) {
	assert.Equal(t, uint16(math.MaxUint16), saddu16(math.MaxUint16, 1))
	assert.Equal(t, uint16(1), saddu16(1, 0))
}

func TestSmulu16(t *testing.T) {
	assert.Equal(t, uint16(math.MaxUint16), smulu16(math.MaxUint16, 1))
	assert.Equal(t, uint16(math.MaxUint16), smulu16(math.MaxUint16, 2))
	assert.Equal(t, uint16(0), smulu16(math.MaxUint16, 0))
	assert.Equal(t, uint16(0), smulu16(0, 0))
}

type benchDispatcher struct{}

func (p *benchDispatcher) Print(r rune) {}

func (p *benchDispatcher) Execute(b byte) {}

func (p *benchDispatcher) Put(b byte) {}

func (p *benchDispatcher) Unhook() {}

func (p *benchDispatcher) Hook(params [][]uint16, intermediates []byte, ignore bool, r rune) {}

func (p *benchDispatcher) OscDispatch(params [][]byte, bellTerminated bool) {}

func (p *benchDispatcher) CsiDispatch(params [][]uint16, intermediates []byte, ignore bool, r rune) {
}

func (p *benchDispatcher) EscDispatch(intermediates []byte, ignore bool, b byte) {}

func (p *benchDispatcher) SosPmApcDispatch(kind SosPmApcKind, data []byte, bellTerminated bool) {}

func BenchmarkNext(bm *testing.B) {
	bytes, err := os.ReadFile("./fixtures/demo.vte")

	if err != nil {
		bm.Fatalf("Error: %v", err)
	}

	bm.ResetTimer()
	dispatcher := &benchDispatcher{}
	parser := NewParser(dispatcher)

	for _, b := range bytes {
		parser.Advance(b)
	}
}

func BenchmarkStateChanges(bm *testing.B) {
	input := "\x1b]2;X\x1b\\ \x1b[0m \x1bP0@\x1b\\"

	for i := 0; i < bm.N; i++ {
		dispatcher := &benchDispatcher{}
		parser := NewParser(dispatcher)

		for i := 0; i < 1000; i++ {
			for _, b := range []byte(input) {
				parser.Advance(b)
			}
		}
	}
}

// countDispatcher looks at what it's given without keeping it.
type countDispatcher struct{ n int }

func (d *countDispatcher) CsiDispatch(params [][]uint16, intermediates []byte, _ bool, _ rune) {
	d.n += len(params) + len(intermediates)
}
func (d *countDispatcher) EscDispatch(intermediates []byte, _ bool, _ byte) {
	d.n += len(intermediates)
}
func (*countDispatcher) Execute(byte) {}
func (d *countDispatcher) Hook(params [][]uint16, intermediates []byte, _ bool, _ rune) {
	d.n += len(params) + len(intermediates)
}
func (d *countDispatcher) OscDispatch(params [][]byte, _ bool) { d.n += len(params) }
func (*countDispatcher) Print(rune)                            {}
func (*countDispatcher) Put(byte)                              {}
func (*countDispatcher) Unhook()                               {}
func (d *countDispatcher) SosPmApcDispatch(_ SosPmApcKind, data []byte, _ bool) {
	d.n += len(data)
}

// TestParsingDoesntAllocate checks that once warm, parsing a terminal's usual output,
// with control sequences in every line, allocates nothing.
func TestParsingDoesntAllocate(t *testing.T) {
	d := &countDispatcher{}
	p := NewParser(d)
	input := []byte("\x1b[1;31mred\x1b[0m \x1b[38;2;10;20;30mrgb\x1b[m\x1b[2;5H\x1b[?25l\x1b(0q\x1b(B" +
		"\x1b]0;title\x07\x1b_apc\x1b\\\x1bP1$rq\x1b\\ text\r\n")
	for _, b := range input {
		p.Advance(b)
	}
	allocs := testing.AllocsPerRun(100, func() {
		for _, b := range input {
			p.Advance(b)
		}
	})
	if allocs != 0 {
		t.Errorf("parsing allocated %v objects", allocs)
	}
	if d.n == 0 {
		t.Error("nothing was dispatched")
	}
}
