package trie

type node struct {
	children map[byte]*node
	word     bool
}

// Trie is a byte-based prefix tree.
// It is not safe for concurrent use.
type Trie struct {
	root *node
}

func New() *Trie {
	return &Trie{
		root: &node{children: make(map[byte]*node)},
	}
}

func (t *Trie) Insert(word string) bool {
	if word == "" {
		return false
	}

	current := t.root
	for i := 0; i < len(word); i++ {
		b := word[i]
		if current.children[b] == nil {
			current.children[b] = &node{
				children: make(map[byte]*node),
			}
		}
		current = current.children[b]
	}

	if current.word {
		return false
	}
	current.word = true
	return true
}

func (t *Trie) Contains(word string) bool {
	if word == "" {
		return false
	}

	current := t.root
	for i := 0; i < len(word); i++ {
		current = current.children[word[i]]
		if current == nil {
			return false
		}
	}
	return current.word
}

func (t *Trie) Delete(word string) bool {
	if !t.Contains(word) {
		return false
	}

	var remove func(*node, int) bool
	remove = func(current *node, depth int) bool {
		if depth == len(word) {
			current.word = false
		} else {
			b := word[depth]
			if remove(current.children[b], depth+1) {
				delete(current.children, b)
			}
		}

		return !current.word && len(current.children) == 0
	}

	remove(t.root, 0)
	return true
}

func (t *Trie) WithPrefix(prefix string, limit int) []string {
	current := t.root
	for i := 0; i < len(prefix); i++ {
		current = current.children[prefix[i]]
		if current == nil {
			return []string{}
		}
	}

	result := []string{}

	var walk func(*node, string)
	walk = func(n *node, word string) {
		if limit > 0 && len(result) >= limit {
			return
		}

		if n.word {
			result = append(result, word)
		}

		for b := 0; b < 256; b++ {
			if limit > 0 && len(result) >= limit {
				return
			}

			if child := n.children[byte(b)]; child != nil {
				walk(child, word+string([]byte{byte(b)}))
			}
		}
	}

	walk(current, prefix)
	return result
}
