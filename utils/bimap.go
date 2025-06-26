package utils

type BiMultiMap[K comparable, V comparable] struct {
	forward map[K]map[V]struct{}
	reverse map[V]map[K]struct{}
}

func NewBiMultiMap[K comparable, V comparable]() *BiMultiMap[K, V] {
	return &BiMultiMap[K, V]{
		forward: make(map[K]map[V]struct{}),
		reverse: make(map[V]map[K]struct{}),
	}
}

func (m *BiMultiMap[K, V]) Add(k K, v V) {
	if m.forward[k] == nil {
		m.forward[k] = make(map[V]struct{})
	}
	if m.reverse[v] == nil {
		m.reverse[v] = make(map[K]struct{})
	}
	m.forward[k][v] = struct{}{}
	m.reverse[v][k] = struct{}{}
}

func (m *BiMultiMap[K, V]) GetByKey(k K) []V {
	var result []V
	for v := range m.forward[k] {
		result = append(result, v)
	}
	return result
}

func (m *BiMultiMap[K, V]) GetByValue(v V) []K {
	var result []K
	for k := range m.reverse[v] {
		result = append(result, k)
	}
	return result
}

func (m *BiMultiMap[K, V]) Remove(k K, v V) {
	if m.forward[k] != nil {
		delete(m.forward[k], v)
		if len(m.forward[k]) == 0 {
			delete(m.forward, k)
		}
	}
	if m.reverse[v] != nil {
		delete(m.reverse[v], k)
		if len(m.reverse[v]) == 0 {
			delete(m.reverse, v)
		}
	}
}

func (m *BiMultiMap[K, V]) RemoveByKey(k K) {
	vals, ok := m.forward[k]
	if !ok {
		return
	}
	for v := range vals {
		delete(m.reverse[v], k)
		if len(m.reverse[v]) == 0 {
			delete(m.reverse, v)
		}
	}
	delete(m.forward, k)
}

func (m *BiMultiMap[K, V]) RemoveByValue(v V) {
	keys, ok := m.reverse[v]
	if !ok {
		return
	}
	for k := range keys {
		delete(m.forward[k], v)
		if len(m.forward[k]) == 0 {
			delete(m.forward, k)
		}
	}
	delete(m.reverse, v)
}
