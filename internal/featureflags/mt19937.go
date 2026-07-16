package featureflags

// This file is a bit-for-bit port of random-js v1.0.8's MT19937 implementation
// (lib/random.js), specifically the engine + realZeroToOneExclusive function.
// The vendored reference source is at testdata/random-js-1.0.8.js.
//
// The port is critical: conveyor's feature-flag probability assignment is
// deterministic per (account, flag) pair. If the Go MT19937 produces different
// output than the JS one, every user's flag assignment would silently change.
//
// Key algorithm details (verified against the JS source):
//   - State is Int32Array(624), seeded via init_genrand then init_by_array
//   - seedWithArray first calls init_genrand(0x012bd6aa), then init_by_array
//   - real(0,1) (exclusive) = uint53(engine) / 2^53
//   - uint53 = ((engine() & 0x1fffff) * 0x100000000) + (engine() >>> 0)
//   - All arithmetic is int32 (Math.imul semantics) with |0 coercions

const (
	mtSize      = 624
	mtPeriod    = 397
	mtMatrixA   = 0x9908b0df
	mtUpperMask = 0x80000000
	mtLowerMask = 0x7fffffff
)

// mt19937 is the MT19937 PRNG state, ported from random-js v1.
type mt19937 struct {
	data  [mtSize]int32
	index int
}

// newMT19937 creates a zeroed MT19937 engine.
func newMT19937() *mt19937 {
	return &mt19937{index: mtSize}
}

// seed implements random-js's next.seed(initial) — init_genrand.
func (m *mt19937) seed(initial int32) {
	previous := initial
	m.data[0] = previous
	for i := 1; i < mtSize; i++ {
		// data[i] = (data[i-1] ^ (data[i-1] >>> 30)) * 0x6c078965 + i  (all int32)
		previous = imul(previous^urshift(previous, 30), 0x6c078965) + int32(i)
		m.data[i] = previous
	}
	m.index = mtSize
}

// seedWithArray implements random-js's next.seedWithArray(source).
// First calls seed(0x012bd6aa), then init_by_array.
func (m *mt19937) seedWithArray(source []int32) {
	m.seed(0x012bd6aa)
	initByArray(&m.data, source)
	m.index = mtSize
}

// initByArray implements random-js's seedWithArray(data, source) body.
// This is the standard MT19937 init_by_array, but with random-js's specific
// initial seed (0x012bd6aa) already applied by the caller via seed().
func initByArray(data *[mtSize]int32, source []int32) {
	sourceLength := len(source)
	k := maxInt(sourceLength, mtSize)
	i := 1
	j := 0
	previous := data[0]

	// First loop: uses multiplier 0x0019660d, adds source[j] + j.
	for ; k > 0; k-- {
		// data[i] = previous = ((data[i] ^ imul((previous ^ (urshift(previous, 30))), 0x0019660d)) + source[j] + j) | 0
		data[i] = (data[i] ^ imul(previous^(urshift(previous, 30)), 0x0019660d)) + source[j] + int32(j)
		previous = data[i]
		i++
		if i > mtSize-1 {
			data[0] = data[mtSize-1]
			i = 1
		}
		j++
		if j >= sourceLength {
			j = 0
		}
	}

	// Second loop: uses multiplier 0x5d588b65, subtracts i.
	for k = mtSize - 1; k > 0; k-- {
		// data[i] = previous = ((data[i] ^ imul((previous ^ (urshift(previous, 30))), 0x5d588b65)) - i) | 0
		data[i] = (data[i] ^ imul(previous^(urshift(previous, 30)), 0x5d588b65)) - int32(i)
		previous = data[i]
		i++
		if i > mtSize-1 {
			data[0] = data[mtSize-1]
			i = 1
		}
	}

	data[0] = int32(-2147483648)
}

// refreshData implements random-js's refreshData(data).
func (m *mt19937) refreshData() {
	data := &m.data
	for i := 0; i < mtSize-mtPeriod; i++ {
		val := (data[i] & int32(-2147483648)) | (data[i+1] & int32(mtLowerMask))
		data[i] = data[i+mtPeriod] ^ urshift(val, 1) ^ (mtA(val))
	}
	for i := mtSize - mtPeriod; i < mtSize-1; i++ {
		val := (data[i] & int32(-2147483648)) | (data[i+1] & int32(mtLowerMask))
		data[i] = data[i+(mtPeriod-mtSize)] ^ urshift(val, 1) ^ (mtA(val))
	}
	val := (data[mtSize-1] & int32(-2147483648)) | (data[0] & int32(mtLowerMask))
	data[mtSize-1] = data[mtPeriod-1] ^ urshift(val, 1) ^ (mtA(val))
	m.index = 0
}

// mtA implements the mag01 lookup from the JS source:
//   data[i] = data[...] ^ (tmp >>> 1) ^ ((tmp & 0x1) ? 0x9908b0df : 0)
func mtA(val int32) int32 {
	if val&1 == 1 {
		return int32(-1728053041)
	}
	return 0
}

// next implements random-js's engine() — returns the next int32.
func (m *mt19937) next() int32 {
	if m.index >= mtSize {
		m.refreshData()
	}
	val := m.data[m.index]
	m.index++

	// Tempering (matching the JS implementation exactly).
	val ^= urshift(val, 11)
	val ^= (val << 7) & int32(-1653897088)
	val ^= (val << 15) & int32(-272236544)
	val ^= urshift(val, 18)

	return val
}

// realZeroToOneExclusive returns a float64 in [0, 1).
// This is random-js's Random.realZeroToOneExclusive:
//   uint53(engine) / 0x20000000000000
// where uint53 = ((engine() & 0x1fffff) * 0x100000000) + (engine() >>> 0)
func (m *mt19937) realZeroToOneExclusive() float64 {
	high := int64(m.next() & 0x1fffff) // 21 bits
	low := int64(uint32(m.next()))     // 32 bits (unsigned)
	combined := float64(high)*4294967296.0 + float64(low)
	return combined / 9007199254740992.0 // / 2^53
}

// imul implements Math.imul: 32-bit integer multiply.
func imul(a, b int32) int32 {
	return int32(int64(a) * int64(b))
}

// urshift implements JavaScript's >>> (unsigned right shift) for int32.
// Go has no >>> operator; we convert to uint32, shift, and convert back.
func urshift(x int32, n uint) int32 {
	return int32(uint32(x) >> n)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
