package paramvalidator

import "sync"

// NewParamMask создает новую пустую битовую маску
func NewParamMask() ParamMask {
	return ParamMask{}
}

// NewParamIndex создает новый индекс параметров
func NewParamIndex() *ParamIndex {
	return &ParamIndex{
		maxIndex: int32(MaxParamsCount),
	}
}

// SetBit устанавливает бит по индексу
func (pm *ParamMask) SetBit(index int) {
	if index < 0 || index >= 128 {
		return
	}
	part := index / 32
	bit := uint32(index % 32)
	pm.parts[part] |= (1 << bit)
}

// ClearBit очищает бит по индексу
func (pm *ParamMask) ClearBit(index int) {
	if index < 0 || index >= 128 {
		return
	}
	part := index / 32
	bit := uint32(index % 32)
	pm.parts[part] &^= (1 << bit)
}

// GetBit возвращает значение бита по индексу (ИСПРАВЛЕНО: получатель по значению)
func (pm ParamMask) GetBit(index int) bool {
	if index < 0 || index >= 128 {
		return false
	}
	part := index / 32
	bit := uint32(index % 32)
	return (pm.parts[part] & (1 << bit)) != 0
}

// IsEmpty проверяет, пуста ли маска (ИСПРАВЛЕНО: получатель по значению)
func (pm ParamMask) IsEmpty() bool {
	for i := 0; i < 4; i++ {
		if pm.parts[i] != 0 {
			return false
		}
	}
	return true
}

// Union объединяет две маски (логическое ИЛИ) (ИСПРАВЛЕНО: получатель по значению)
func (pm ParamMask) Union(other ParamMask) ParamMask {
	var result ParamMask
	for i := 0; i < 4; i++ {
		result.parts[i] = pm.parts[i] | other.parts[i]
	}
	return result
}

// Intersection пересекает две маски (логическое И) (ИСПРАВЛЕНО: получатель по значению)
func (pm ParamMask) Intersection(other ParamMask) ParamMask {
	var result ParamMask
	for i := 0; i < 4; i++ {
		result.parts[i] = pm.parts[i] & other.parts[i]
	}
	return result
}

// Difference возвращает разность масок (pm AND NOT other) (ИСПРАВЛЕНО: получатель по значению)
func (pm ParamMask) Difference(other ParamMask) ParamMask {
	var result ParamMask
	for i := 0; i < 4; i++ {
		result.parts[i] = pm.parts[i] &^ other.parts[i]
	}
	return result
}

// Contains проверяет, содержит ли маска все биты другой маски (ИСПРАВЛЕНО: получатель по значению)
func (pm ParamMask) Contains(other ParamMask) bool {
	for i := 0; i < 4; i++ {
		if (other.parts[i] &^ pm.parts[i]) != 0 {
			return false
		}
	}
	return true
}

// Equals проверяет равенство масок (ИСПРАВЛЕНО: получатель по значению)
func (pm ParamMask) Equals(other ParamMask) bool {
	for i := 0; i < 4; i++ {
		if pm.parts[i] != other.parts[i] {
			return false
		}
	}
	return true
}

// Count возвращает количество установленных битов (ИСПРАВЛЕНО: получатель по значению)
func (pm ParamMask) Count() int {
	count := 0
	for i := 0; i < 4; i++ {
		count += countBits(pm.parts[i])
	}
	return count
}

// GetIndices возвращает индексы установленных битов (ИСПРАВЛЕНО: получатель по значению)
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

// countBits подсчитывает количество установленных битов в uint32
func countBits(x uint32) int {
	count := 0
	for x != 0 {
		count++
		x &= x - 1
	}
	return count
}

// GetOrCreateIndex возвращает индекс параметра, создавая новый если нужно
func (pi *ParamIndex) GetOrCreateIndex(paramName string) int {
	if idx, ok := pi.paramToIndex.Load(paramName); ok {
		return idx.(int)
	}

	// Atomic создание нового индекса
	newIdx := int(pi.nextIndex.Add(1) - 1)
	if newIdx >= int(pi.maxIndex) {
		return -1 // Достигнут лимит
	}

	// Пытаемся сохранить, если другой горутина уже не сделала это
	if actual, loaded := pi.paramToIndex.LoadOrStore(paramName, newIdx); loaded {
		return actual.(int)
	}

	return newIdx
}

// GetParamName возвращает имя параметра по индексу (медленнее, но редко используется)
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

// Clear очищает индекс
func (pi *ParamIndex) Clear() {
	pi.paramToIndex = sync.Map{}
	pi.nextIndex.Store(0)
}

// GetIndex возвращает индекс параметра или -1 если не найден
func (pi *ParamIndex) GetIndex(paramName string) int {
	if idx, ok := pi.paramToIndex.Load(paramName); ok {
		return idx.(int)
	}
	return -1
}

// CreateMaskForParams создает маску для списка параметров
func (pi *ParamIndex) CreateMaskForParams(params map[string]*ParamRule) ParamMask {
	mask := NewParamMask()
	for paramName := range params {
		if idx := pi.GetIndex(paramName); idx != -1 {
			mask.SetBit(idx) // SetBit работает с указателем, но Go автоматически берет адрес
		}
	}
	return mask
}

// GetParamsFromMask возвращает имена параметров из маски
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

// CombinedMask возвращает объединенную маску с учетом приоритетов (ИСПРАВЛЕНО: получатель по значению)
func (pm ParamMasks) CombinedMask() ParamMask {
	// SpecificURL имеет высший приоритет
	result := pm.Global.Union(pm.URL)
	result = result.Union(pm.SpecificURL) // SpecificURL перезаписывает конфликтующие параметры
	return result
}

// GetRuleSource возвращает источник правила для параметра (ИСПРАВЛЕНО: получатель по значению)
func (pm ParamMasks) GetRuleSource(paramIndex int) RuleSource {
	// ВАЖНО: проверяем в порядке приоритета
	if pm.SpecificURL.GetBit(paramIndex) {
		return SourceSpecificURL
	}
	if pm.URL.GetBit(paramIndex) {
		return SourceURL
	}
	if pm.Global.GetBit(paramIndex) {
		return SourceGlobal
	}
	return SourceNone
}
