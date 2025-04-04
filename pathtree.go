package lux

import (
	"fmt"
	"strings"
)

// Param represents a URL parameter with key-value pair
type Param struct {
	Key   string // Parameter name
	Value string // Parameter value from the URL
}

// Params is a collection of URL parameters
type Params []Param

func (ps Params) Get(name string) (string, bool) {
	for _, param := range ps {
		if param.Key == name {
			return param.Value, true
		}
	}
	return "", false
}

func (ps Params) ByName(name string) (v string) {
	v, _ = ps.Get(name)
	return
}

// NodeType defines the type of router tree node
type NodeType int

// Node types for the router tree
const (
	Static    NodeType = iota // Regular path segment
	Root                      // Root node of the tree
	Parameter                 // Path parameter (e.g., :id)
	Wildcard                  // Wildcard parameter (e.g., *filepath)
)

// Node represents a node in the router tree
type Node struct {
	Path     string       // Path segment this node represents
	NodeType NodeType     // Type of the node
	Handlers HandlerChain // Handlers associated with this endpoint
	Children []*Node      // Child nodes
}

// addRoute adds a new route to the node tree
// Panics if the path is already registered with handlers
func (n *Node) addRoute(path string, handlers []HandlerFunc) {
	segments := splitPath(path)
	current := n

	// Track if we're adding to an existing endpoint
	pathExists := true

	for i, segment := range segments {
		if segment == "" {
			continue
		}

		found := false
		for _, child := range current.Children {
			if child.Path == segment || (child.NodeType == Parameter && segment[0] == ':') {
				current = child
				found = true
				break
			}
		}

		if !found {
			pathExists = false // New node means this is a new path
			nodeType := Static
			if segment[0] == ':' {
				nodeType = Parameter
			} else if segment[0] == '*' {
				nodeType = Wildcard
			}
			newNode := &Node{
				Path:     segment,
				NodeType: nodeType,
				Children: make([]*Node, 0),
			}
			current.Children = append(current.Children, newNode)
			current = newNode
		}

		if i == len(segments)-1 {
			// Check for duplicate routes
			if len(current.Handlers) > 0 && pathExists {
				panic(fmt.Sprintf("Route already exists: %s", path))
			}
			current.Handlers = handlers
		}
	}

	// Special case for root path
	if len(path) == 1 && path == "/" {
		// Check for duplicate root handler
		if len(n.Handlers) > 0 {
			panic("Root route '/' already registered")
		}
		n.Handlers = handlers
	}
}

// NodeTree represents a router tree for a specific HTTP method
type NodeTree struct {
	Root   *Node  // Root node of the tree
	Method string // HTTP method this tree is for (GET, POST, etc.)
}

// methodTrees is a collection of method-specific router trees
type methodTrees []NodeTree

// get returns the root node for a specific HTTP method
func (trees methodTrees) get(method string) *Node {
	for _, tree := range trees {
		if tree.Method == method {
			return tree.Root
		}
	}
	return nil
}

// NewNodeTree creates a new router tree
func NewNodeTree() *NodeTree {
	return &NodeTree{Root: &Node{
		NodeType: Root,
		Path:     "/",
		Children: make([]*Node, 0),
	}}
}

// addRoute adds a new route to the tree
// Panics if the path is already registered with handlers
func (nt *NodeTree) addRoute(path string, handlers []HandlerFunc) {
	segments := splitPath(path)
	current := nt.Root

	// Track if we're adding to an existing endpoint
	pathExists := true

	for i, segment := range segments {
		if segment == "" {
			continue
		}

		found := false
		for _, child := range current.Children {
			if child.Path == segment || (child.NodeType == Parameter && segment[0] == ':') {
				current = child
				found = true
				break
			}
		}

		if !found {
			pathExists = false // New node means this is a new path
			nodeType := Static
			if segment[0] == ':' {
				nodeType = Parameter
			} else if segment[0] == '*' {
				nodeType = Wildcard
			}
			newNode := &Node{
				Path:     segment,
				NodeType: nodeType,
				Children: make([]*Node, 0),
			}
			current.Children = append(current.Children, newNode)
			current = newNode
		}

		if i == len(segments)-1 {
			// Check for duplicate routes
			if len(current.Handlers) > 0 && pathExists {
				panic(fmt.Sprintf("Duplicate route detected: %s", path))
			}
			current.Handlers = handlers
		}
	}

	// Special case for empty path or root path
	if path == "" || path == "/" {
		if len(nt.Root.Handlers) > 0 {
			panic("Root route '/' already registered")
		}
		nt.Root.Handlers = handlers
	}
}

// Find locates a handler for the given path and extracts URL parameters
func (nt *NodeTree) Find(path string) (HandlerChain, Params) {
	segments := splitPath(path)
	params := make(Params, 0)
	skippedNodes := make([]skippedNode, 0, 2) // Create skippedNodes for backtracking
	handler := nt.findNode(nt.Root, segments, &params, 0, &skippedNodes)
	return handler, params
}

// skippedNode represents a potential alternative path during route matching
type skippedNode struct {
	node        *Node
	segmentIdx  int
	paramsCount int
}

// findNode recursively searches for a matching node in the tree
func (nt *NodeTree) findNode(node *Node, segments []string, params *Params, index int, skippedNodes *[]skippedNode) HandlerChain {
	// End of path, return handlers if any
	if index >= len(segments) {
		if len(node.Handlers) > 0 {
			return node.Handlers
		}

		// No handler found, try backtracking
		return nt.tryBacktrack(segments, params, skippedNodes)
	}

	segment := segments[index]
	// Handle empty segment at the end (trailing slash)
	if segment == "" && index == len(segments)-1 {
		if len(node.Handlers) > 0 {
			return node.Handlers
		}

		// No handler found, try backtracking
		return nt.tryBacktrack(segments, params, skippedNodes)
	}

	// First try to match static nodes (most common case)
	for _, child := range node.Children {
		if child.NodeType == Static && child.Path == segment {
			// Before going down this path, check if there are parameter nodes
			// that could be alternative paths
			for _, paramChild := range node.Children {
				if paramChild.NodeType == Parameter || paramChild.NodeType == Wildcard {
					// Save this as a skipped node for potential backtracking
					*skippedNodes = append(*skippedNodes, skippedNode{
						node:        paramChild,
						segmentIdx:  index,
						paramsCount: len(*params),
					})
				}
			}

			if handler := nt.findNode(child, segments, params, index+1, skippedNodes); handler != nil {
				return handler
			}
		}
	}

	// Then try parameter nodes
	for _, child := range node.Children {
		if child.NodeType == Parameter {
			// Save previous params length for backtracking
			originalParamsLen := len(*params)

			*params = append(*params, Param{
				Key:   child.Path[1:], // skip the ':' prefix
				Value: segment,
			})

			if handler := nt.findNode(child, segments, params, index+1, skippedNodes); handler != nil {
				return handler
			}

			// Remove param if no match found with this path
			*params = (*params)[:originalParamsLen]
		}
	}

	// Finally try wildcard nodes (they match rest of the path)
	for _, child := range node.Children {
		if child.NodeType == Wildcard {
			remaining := strings.Join(segments[index:], "/")
			*params = append(*params, Param{
				Key:   child.Path[1:], // skip '*' prefix
				Value: remaining,
			})
			return child.Handlers
		}
	}

	// No match found, try backtracking
	return nt.tryBacktrack(segments, params, skippedNodes)
}

// tryBacktrack attempts to find an alternative path using saved skipped nodes
func (nt *NodeTree) tryBacktrack(segments []string, params *Params, skippedNodes *[]skippedNode) HandlerChain {
	// No more skipped nodes to try
	if len(*skippedNodes) == 0 {
		return nil
	}

	// Pop the last skipped node
	lastIdx := len(*skippedNodes) - 1
	skipped := (*skippedNodes)[lastIdx]
	*skippedNodes = (*skippedNodes)[:lastIdx]

	// Reset params to the state when this node was skipped
	*params = (*params)[:skipped.paramsCount]

	// Try this alternative path
	if skipped.node.NodeType == Parameter {
		segment := segments[skipped.segmentIdx]
		*params = append(*params, Param{
			Key:   skipped.node.Path[1:], // skip the ':' prefix
			Value: segment,
		})
		return nt.findNode(skipped.node, segments, params, skipped.segmentIdx+1, skippedNodes)
	} else if skipped.node.NodeType == Wildcard {
		remaining := strings.Join(segments[skipped.segmentIdx:], "/")
		*params = append(*params, Param{
			Key:   skipped.node.Path[1:], // skip '*' prefix
			Value: remaining,
		})
		return skipped.node.Handlers
	}

	// Continue with static node
	return nt.findNode(skipped.node, segments, params, skipped.segmentIdx+1, skippedNodes)
}

// splitPath splits a URL path into segments
func splitPath(path string) []string {
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		return []string{""}
	}
	return strings.Split(path, "/")
}
