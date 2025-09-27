package paramvalidator

import "sync"

var bitCountTable [256]byte

// NewParamMask creates a new empty bit mask
func NewParamMask() ParamMask {
	return ParamMask{}
}

// NewParamIndex creates a new parameter index
func NewParamIndex() *ParamIndex {
	return &ParamIndex{
		maxIndex: int32(MaxParamsCount),
	}
}

// SetBit sets the bit at the specified index
func (pm *ParamMask) SetBit(index int) {
	if index < 0 || index >= 128 {
		return
	}
	part := index / 32
	bit := uint32(index % 32)
	pm.parts[part] |= (1 << bit)
}

// ClearBit clears the bit at the specified index
func (pm *ParamMask) ClearBit(index int) {
	if index < 0 || index >= 128 {
		return
	}
	part := index / 32
	bit := uint32(index % 32)
	pm.parts[part] &^= (1 << bit)
}

// GetBit returns the value of the bit at the specified index
func (pm ParamMask) GetBit(index int) bool {
	if index < 0 || index >= 128 {
		return false
	}
	part := index / 32
	bit := uint32(index % 32)
	return (pm.parts[part] & (1 << bit)) != 0
}

// IsEmpty checks if the mask is empty
func (pm ParamMask) IsEmpty() bool {
	for i := 0; i < 4; i++ {
		if pm.parts[i] != 0 {
			return false
		}
	}
	return true
}

// Union combines two masks (logical OR)
func (pm ParamMask) Union(other ParamMask) ParamMask {
	var result ParamMask
	for i := 0; i < 4; i++ {
		result.parts[i] = pm.parts[i] | other.parts[i]
	}
	return result
}

// Intersection intersects two masks (logical AND)
func (pm ParamMask) Intersection(other ParamMask) ParamMask {
	var result ParamMask
	for i := 0; i < 4; i++ {
		result.parts[i] = pm.parts[i] & other.parts[i]
	}
	return result
}

// Difference returns the difference of masks (pm AND NOT other)
func (pm ParamMask) Difference(other ParamMask) ParamMask {
	var result ParamMask
	for i := 0; i < 4; i++ {
		result.parts[i] = pm.parts[i] &^ other.parts[i]
	}
	return result
}

// Contains checks if the mask contains all bits of another mask
func (pm ParamMask) Contains(other ParamMask) bool {
	for i := 0; i < 4; i++ {
		if (other.parts[i] &^ pm.parts[i]) != 0 {
			return false
		}
	}
	return true
}

// Equals checks if masks are equal
func (pm ParamMask) Equals(other ParamMask) bool {
	for i := 0; i < 4; i++ {
		if pm.parts[i] != other.parts[i] {
			return false
		}
	}
	return true
}

func init() {
	for i := range bitCountTable {
		bitCountTable[i] = bitCountTable[i/2] + byte(i&1)
	}
}

// Count returns the number of set bits in the mask
func (pm ParamMask) Count() int {
	count := 0
	for i := 0; i < 4; i++ {
		// Fast counting using lookup table
		count += int(bitCountTable[byte(pm.parts[i]>>24)]) +
			int(bitCountTable[byte(pm.parts[i]>>16)]) +
			int(bitCountTable[byte(pm.parts[i]>>8)]) +
			int(bitCountTable[byte(pm.parts[i])])
	}
	return count
}

// GetIndices returns indices of set bits
func (pm ParamMask) GetIndices() []int {
	var indices []int
	for i := 0; i < 4; i++ {
		for j := 0; j < 32; j++ {
			if (pm.parts[i] & (1 << j)) != 0 {
				indices = append(indices, i*32+j)
			}
		}
	}
	return indices
}

// GetOrCreateIndex returns parameter index, creating new one if needed
func (pi *ParamIndex) GetOrCreateIndex(paramName string) int {
	if idx, ok := pi.paramToIndex.Load(paramName); ok {
		return idx.(int)
	}

	// Atomic creation of new index
	newIdx := int(pi.nextIndex.Add(1) - 1)
	if newIdx >= int(pi.maxIndex) {
		return -1 // Limit reached
	}

	// Try to store, unless another goroutine already did it
	if actual, loaded := pi.paramToIndex.LoadOrStore(paramName, newIdx); loaded {
		return actual.(int)
	}

	return newIdx
}

// GetParamName returns parameter name by index (slower, rarely used)
func (pi *ParamIndex) GetParamName(index int) string {
	var result string
	pi.paramToIndex.Range(func(key, value interface{}) bool {
		if value.(int) == index {
			result = key.(string)
			return false
		}
		return true
	})
	return result
}

// Clear clears the index
func (pi *ParamIndex) Clear() {
	pi.paramToIndex = sync.Map{}
	pi.nextIndex.Store(0)
}

// GetIndex returns parameter index or -1 if not found
func (pi *ParamIndex) GetIndex(paramName string) int {
	if idx, ok := pi.paramToIndex.Load(paramName); ok {
		return idx.(int)
	}
	return -1
}

// GetBitUnsafe fast version without bounds checking
func (pm ParamMask) GetBitUnsafe(index int) bool {
	part := index / 32
	bit := uint32(index % 32)
	return (pm.parts[part] & (1 << bit)) != 0
}

// SetBitUnsafe fast version without bounds checking
func (pm *ParamMask) SetBitUnsafe(index int) {
	part := index / 32
	bit := uint32(index % 32)
	pm.parts[part] |= (1 << bit)
}

// CreateMaskForParams creates mask for parameter list
func (pi *ParamIndex) CreateMaskForParams(params map[string]*ParamRule) ParamMask {
	mask := NewParamMask()
	for paramName := range params {
		if idx := pi.GetIndex(paramName); idx != -1 {
			mask.SetBitUnsafe(idx) // Without bounds checking
		}
	}
	return mask
}

// GetParamsFromMask returns parameter names from mask
func (pi *ParamIndex) GetParamsFromMask(mask ParamMask) []string {
	indices := mask.GetIndices()
	params := make([]string, 0, len(indices))

	for _, idx := range indices {
		if name := pi.GetParamName(idx); name != "" {
			params = append(params, name)
		}
	}
	return params
}

// CombinedMask returns combined mask considering priorities
func (pm ParamMasks) CombinedMask() ParamMask {
	// SpecificURL has highest priority
	result := pm.Global.Union(pm.URL)
	result = result.Union(pm.SpecificURL) // SpecificURL overwrites conflicting parameters
	return result
}

// GetRuleSource returns rule source for parameter
func (pm ParamMasks) GetRuleSource(paramIndex int) RuleSource {
	// IMPORTANT: check in priority order
	if pm.SpecificURL.GetBitUnsafe(paramIndex) {
		return SourceSpecificURL
	}
	if pm.URL.GetBitUnsafe(paramIndex) {
		return SourceURL
	}
	if pm.Global.GetBitUnsafe(paramIndex) {
		return SourceGlobal
	}
	return SourceNone
}
