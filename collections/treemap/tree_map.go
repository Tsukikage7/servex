// Package treemap 提供基于红黑树实现的有序 Map.
// 注意：本包的数据结构非并发安全，如需在多 goroutine 中使用请自行加锁.
package treemap

import (
	"cmp"
	"iter"
)

// 红黑树节点颜色.
const (
	red   = true
	black = false
)

// Comparator 比较函数.
// 返回值: 负数(a<b), 0(a==b), 正数(a>b).
type Comparator[K any] func(a, b K) int

// Entry 键值对.
type Entry[K any, V any] struct {
	Key   K
	Value V
}

// node 红黑树节点.
type node[K any, V any] struct {
	key    K
	value  V
	color  bool
	left   *node[K, V]
	right  *node[K, V]
	parent *node[K, V]
}

// TreeMap 基于红黑树的有序 Map.
// 特性:
//   - 按键排序存储
//   - Put/Get/Remove 操作时间复杂度 O(log n)
//   - 支持自定义比较器
// 示例:
//	tm := treemap.New[int, string](treemap.OrderedCompare[int])
//	tm.Put(3, "three")
//	tm.Put(1, "one")
//	tm.Put(2, "two")
//	tm.Keys() // [1, 2, 3]
type TreeMap[K any, V any] struct {
	root *node[K, V]
	cmp  Comparator[K]
	size int
}

// New 创建 TreeMap，需要提供比较器.
func New[K any, V any](cmp Comparator[K]) *TreeMap[K, V] {
	return &TreeMap[K, V]{cmp: cmp}
}

// NewOrdered 创建 TreeMap，使用内置类型的默认比较.
// 支持 int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64,
// float32, float64, string 等实现了 cmp.Ordered 的类型.
func NewOrdered[K cmp.Ordered, V any]() *TreeMap[K, V] {
	return &TreeMap[K, V]{cmp: OrderedCompare[K]}
}

// Put 插入或更新键值对.
func (m *TreeMap[K, V]) Put(key K, value V) {
	if m.root == nil {
		m.root = &node[K, V]{key: key, value: value, color: black}
		m.size = 1
		return
	}

	// 查找插入位置
	var parent *node[K, V]
	current := m.root
	for current != nil {
		parent = current
		c := m.cmp(key, current.key)
		if c < 0 {
			current = current.left
		} else if c > 0 {
			current = current.right
		} else {
			// 键已存在，更新值
			current.value = value
			return
		}
	}

	// 插入新节点
	newNode := &node[K, V]{key: key, value: value, color: red, parent: parent}
	if m.cmp(key, parent.key) < 0 {
		parent.left = newNode
	} else {
		parent.right = newNode
	}
	m.size++

	// 修复红黑树性质
	m.insertFixup(newNode)
}

// Get 获取键对应的值.
func (m *TreeMap[K, V]) Get(key K) (V, bool) {
	n := m.findNode(key)
	if n == nil {
		var zero V
		return zero, false
	}
	return n.value, true
}

// GetOrDefault 获取键对应的值，不存在则返回默认值.
func (m *TreeMap[K, V]) GetOrDefault(key K, defaultVal V) V {
	if v, ok := m.Get(key); ok {
		return v
	}
	return defaultVal
}

// Remove 删除键值对，返回被删除的值.
func (m *TreeMap[K, V]) Remove(key K) (V, bool) {
	n := m.findNode(key)
	if n == nil {
		var zero V
		return zero, false
	}

	value := n.value
	m.deleteNode(n)
	m.size--
	return value, true
}

// ContainsKey 判断键是否存在.
func (m *TreeMap[K, V]) ContainsKey(key K) bool {
	return m.findNode(key) != nil
}

// Len 返回元素数量.
func (m *TreeMap[K, V]) Len() int {
	return m.size
}

// IsEmpty 判断是否为空.
func (m *TreeMap[K, V]) IsEmpty() bool {
	return m.size == 0
}

// Clear 清空所有元素.
func (m *TreeMap[K, V]) Clear() {
	m.root = nil
	m.size = 0
}

// Keys 返回所有键（按排序顺序）.
func (m *TreeMap[K, V]) Keys() []K {
	keys := make([]K, 0, m.size)
	m.inorderTraversal(m.root, func(n *node[K, V]) bool {
		keys = append(keys, n.key)
		return true
	})
	return keys
}

// Values 返回所有值（按键排序顺序）.
func (m *TreeMap[K, V]) Values() []V {
	values := make([]V, 0, m.size)
	m.inorderTraversal(m.root, func(n *node[K, V]) bool {
		values = append(values, n.value)
		return true
	})
	return values
}

// Entries 返回所有键值对（按键排序顺序）.
func (m *TreeMap[K, V]) Entries() []Entry[K, V] {
	entries := make([]Entry[K, V], 0, m.size)
	m.inorderTraversal(m.root, func(n *node[K, V]) bool {
		entries = append(entries, Entry[K, V]{Key: n.key, Value: n.value})
		return true
	})
	return entries
}

// Range 按顺序遍历所有键值对.
// fn 返回 false 时停止遍历.
func (m *TreeMap[K, V]) Range(fn func(key K, value V) bool) {
	m.inorderTraversal(m.root, func(n *node[K, V]) bool {
		return fn(n.key, n.value)
	})
}

// RangeReverse 按逆序遍历所有键值对.
// fn 返回 false 时停止遍历.
func (m *TreeMap[K, V]) RangeReverse(fn func(key K, value V) bool) {
	m.reverseInorderTraversal(m.root, func(n *node[K, V]) bool {
		return fn(n.key, n.value)
	})
}

// All 返回按键排序顺序遍历所有键值对的迭代器.
func (m *TreeMap[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		m.inorderTraversal(m.root, func(n *node[K, V]) bool {
			return yield(n.key, n.value)
		})
	}
}

// Backward 返回按键逆序遍历所有键值对的迭代器.
func (m *TreeMap[K, V]) Backward() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		m.reverseInorderTraversal(m.root, func(n *node[K, V]) bool {
			return yield(n.key, n.value)
		})
	}
}

// FirstKey 返回最小的键.
func (m *TreeMap[K, V]) FirstKey() (K, bool) {
	if m.root == nil {
		var zero K
		return zero, false
	}
	n := m.minimum(m.root)
	return n.key, true
}

// LastKey 返回最大的键.
func (m *TreeMap[K, V]) LastKey() (K, bool) {
	if m.root == nil {
		var zero K
		return zero, false
	}
	n := m.maximum(m.root)
	return n.key, true
}

// First 返回最小键的键值对.
func (m *TreeMap[K, V]) First() (Entry[K, V], bool) {
	if m.root == nil {
		return Entry[K, V]{}, false
	}
	n := m.minimum(m.root)
	return Entry[K, V]{Key: n.key, Value: n.value}, true
}

// Last 返回最大键的键值对.
func (m *TreeMap[K, V]) Last() (Entry[K, V], bool) {
	if m.root == nil {
		return Entry[K, V]{}, false
	}
	n := m.maximum(m.root)
	return Entry[K, V]{Key: n.key, Value: n.value}, true
}

// ToMap 转换为原生 map（无序）.
func (m *TreeMap[K, V]) ToMap() map[any]V {
	result := make(map[any]V, m.size)
	m.Range(func(key K, value V) bool {
		result[key] = value
		return true
	})
	return result
}

// Clone 克隆 TreeMap.
func (m *TreeMap[K, V]) Clone() *TreeMap[K, V] {
	clone := New[K, V](m.cmp)
	m.Range(func(key K, value V) bool {
		clone.Put(key, value)
		return true
	})
	return clone
}

// Comparator 返回比较器.
func (m *TreeMap[K, V]) Comparator() Comparator[K] {
	return m.cmp
}

// findNode 查找键对应的节点.
func (m *TreeMap[K, V]) findNode(key K) *node[K, V] {
	current := m.root
	for current != nil {
		c := m.cmp(key, current.key)
		if c < 0 {
			current = current.left
		} else if c > 0 {
			current = current.right
		} else {
			return current
		}
	}
	return nil
}

// minimum 返回子树中最小的节点.
func (m *TreeMap[K, V]) minimum(n *node[K, V]) *node[K, V] {
	for n.left != nil {
		n = n.left
	}
	return n
}

// maximum 返回子树中最大的节点.
func (m *TreeMap[K, V]) maximum(n *node[K, V]) *node[K, V] {
	for n.right != nil {
		n = n.right
	}
	return n
}

// inorderTraversal 中序遍历.
func (m *TreeMap[K, V]) inorderTraversal(n *node[K, V], fn func(*node[K, V]) bool) bool {
	if n == nil {
		return true
	}
	if !m.inorderTraversal(n.left, fn) {
		return false
	}
	if !fn(n) {
		return false
	}
	return m.inorderTraversal(n.right, fn)
}

// reverseInorderTraversal 逆中序遍历（从大到小）.
func (m *TreeMap[K, V]) reverseInorderTraversal(n *node[K, V], fn func(*node[K, V]) bool) bool {
	if n == nil {
		return true
	}
	if !m.reverseInorderTraversal(n.right, fn) {
		return false
	}
	if !fn(n) {
		return false
	}
	return m.reverseInorderTraversal(n.left, fn)
}

// 红黑树操作

func (m *TreeMap[K, V]) rotateLeft(n *node[K, V]) {
	r := n.right
	n.right = r.left
	if r.left != nil {
		r.left.parent = n
	}
	r.parent = n.parent
	if n.parent == nil {
		m.root = r
	} else if n == n.parent.left {
		n.parent.left = r
	} else {
		n.parent.right = r
	}
	r.left = n
	n.parent = r
}

func (m *TreeMap[K, V]) rotateRight(n *node[K, V]) {
	l := n.left
	n.left = l.right
	if l.right != nil {
		l.right.parent = n
	}
	l.parent = n.parent
	if n.parent == nil {
		m.root = l
	} else if n == n.parent.right {
		n.parent.right = l
	} else {
		n.parent.left = l
	}
	l.right = n
	n.parent = l
}

func (m *TreeMap[K, V]) insertFixup(n *node[K, V]) {
	for n.parent != nil && n.parent.color == red {
		if n.parent == n.parent.parent.left {
			uncle := n.parent.parent.right
			if uncle != nil && uncle.color == red {
				n.parent.color = black
				uncle.color = black
				n.parent.parent.color = red
				n = n.parent.parent
			} else {
				if n == n.parent.right {
					n = n.parent
					m.rotateLeft(n)
				}
				n.parent.color = black
				n.parent.parent.color = red
				m.rotateRight(n.parent.parent)
			}
		} else {
			uncle := n.parent.parent.left
			if uncle != nil && uncle.color == red {
				n.parent.color = black
				uncle.color = black
				n.parent.parent.color = red
				n = n.parent.parent
			} else {
				if n == n.parent.left {
					n = n.parent
					m.rotateRight(n)
				}
				n.parent.color = black
				n.parent.parent.color = red
				m.rotateLeft(n.parent.parent)
			}
		}
	}
	m.root.color = black
}

func (m *TreeMap[K, V]) deleteNode(n *node[K, V]) {
	var child, parent *node[K, V]
	var color bool

	// 节点有两个子节点
	if n.left != nil && n.right != nil {
		successor := m.minimum(n.right)
		n.key = successor.key
		n.value = successor.value
		n = successor
	}

	if n.left != nil {
		child = n.left
	} else {
		child = n.right
	}

	parent = n.parent
	color = n.color

	if child != nil {
		child.parent = parent
	}

	if parent == nil {
		m.root = child
	} else if n == parent.left {
		parent.left = child
	} else {
		parent.right = child
	}

	if color == black {
		m.deleteFixup(child, parent)
	}
}

func (m *TreeMap[K, V]) deleteFixup(n, parent *node[K, V]) {
	for (n == nil || n.color == black) && n != m.root {
		if n == parent.left {
			sibling := parent.right
			if sibling != nil && sibling.color == red {
				sibling.color = black
				parent.color = red
				m.rotateLeft(parent)
				sibling = parent.right
			}
			if sibling == nil || ((sibling.left == nil || sibling.left.color == black) &&
				(sibling.right == nil || sibling.right.color == black)) {
				if sibling != nil {
					sibling.color = red
				}
				n = parent
				parent = n.parent
			} else {
				if sibling.right == nil || sibling.right.color == black {
					if sibling.left != nil {
						sibling.left.color = black
					}
					sibling.color = red
					m.rotateRight(sibling)
					sibling = parent.right
				}
				sibling.color = parent.color
				parent.color = black
				if sibling.right != nil {
					sibling.right.color = black
				}
				m.rotateLeft(parent)
				n = m.root
				break
			}
		} else {
			sibling := parent.left
			if sibling != nil && sibling.color == red {
				sibling.color = black
				parent.color = red
				m.rotateRight(parent)
				sibling = parent.left
			}
			if sibling == nil || ((sibling.right == nil || sibling.right.color == black) &&
				(sibling.left == nil || sibling.left.color == black)) {
				if sibling != nil {
					sibling.color = red
				}
				n = parent
				parent = n.parent
			} else {
				if sibling.left == nil || sibling.left.color == black {
					if sibling.right != nil {
						sibling.right.color = black
					}
					sibling.color = red
					m.rotateLeft(sibling)
					sibling = parent.left
				}
				sibling.color = parent.color
				parent.color = black
				if sibling.left != nil {
					sibling.left.color = black
				}
				m.rotateRight(parent)
				n = m.root
				break
			}
		}
	}
	if n != nil {
		n.color = black
	}
}
