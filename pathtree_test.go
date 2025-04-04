package lux

import (
	"fmt"
	"reflect"
	"testing"
)

// MockHandlerFunc for testing purposes
func mockHandler(index int) HandlerFunc {
	return func(c *Context) {
		// This is just a placeholder for testing
	}
}

// Create mock handler chains for testing
func createHandlers(count int) HandlerChain {
	handlers := make(HandlerChain, count)
	for i := 0; i < count; i++ {
		handlers[i] = mockHandler(i)
	}
	return handlers
}

func TestNodeTreeAddRoute(t *testing.T) {
	tree := NewNodeTree()

	// Test adding basic routes
	routes := []string{
		"/",
		"/users",
		"/users/:id",
		"/posts",
		"/posts/:id/comments",
		"/posts/:id/comments/:commentId",
		"/static/*filepath",
	}

	for _, route := range routes {
		handlers := createHandlers(1)
		tree.addRoute(route, handlers)

		// Verify route was added correctly
		foundHandlers, _ := tree.Find(route)
		if foundHandlers == nil {
			t.Errorf("Route not found after adding: %s", route)
		}

		// For parameterized routes, test with actual values
		if route == "/users/:id" {
			handlers, params := tree.Find("/users/123")
			if handlers == nil {
				t.Errorf("Parameterized route not found: %s", route)
			}
			if len(params) != 1 || params[0].Key != "id" || params[0].Value != "123" {
				t.Errorf("Parameter extraction failed for route: %s", route)
			}
		}

		if route == "/static/*filepath" {
			handlers, params := tree.Find("/static/css/style.css")
			if handlers == nil {
				t.Errorf("Wildcard route not found: %s", route)
			}
			if len(params) != 1 || params[0].Key != "filepath" || params[0].Value != "css/style.css" {
				t.Errorf("Wildcard parameter extraction failed for route: %s", route)
			}
		}
	}
}

func TestDuplicateRoutePanic(t *testing.T) {
	tree := NewNodeTree()

	// First add a route
	tree.addRoute("/users", createHandlers(1))

	// Test that adding the same route panics
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected panic when adding duplicate route, but no panic occurred")
		}
	}()

	// This should cause a panic
	tree.addRoute("/users", createHandlers(1))
}

func TestMultipleDuplicateScenarios(t *testing.T) {
	testCases := []struct {
		name        string
		firstRoute  string
		secondRoute string
		shouldPanic bool
	}{
		{"Same static route", "/users", "/users", true},
		{"Different static routes", "/users", "/posts", false},
		{"Root route duplicate", "/", "/", true},
		{"Parameter route duplicate", "/users/:id", "/users/:id", true},
		{"Different parameter names same position", "/users/:id", "/users/:userId", true}, // Should still panic
		{"Wildcard duplicate", "/static/*filepath", "/static/*path", true},
		{"Parameter vs static route", "/users/:id", "/users/profile", false}, // Different routes
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tree := NewNodeTree()

			// Add first route
			tree.addRoute(tc.firstRoute, createHandlers(1))

			// Test adding second route
			defer func() {
				r := recover()
				if tc.shouldPanic && r == nil {
					t.Errorf("Expected panic when adding duplicate route %s after %s, but no panic occurred",
						tc.secondRoute, tc.firstRoute)
				}
				if !tc.shouldPanic && r != nil {
					t.Errorf("Unexpected panic when adding route %s after %s: %v",
						tc.secondRoute, tc.firstRoute, r)
				}
			}()

			// This may cause a panic depending on the test case
			tree.addRoute(tc.secondRoute, createHandlers(1))
		})
	}
}

func TestParamExtraction(t *testing.T) {
	tree := NewNodeTree()

	// Add routes with parameters
	tree.addRoute("/users/:id", createHandlers(1))
	tree.addRoute("/posts/:postId/comments/:commentId", createHandlers(1))
	tree.addRoute("/files/*filepath", createHandlers(1))

	testCases := []struct {
		route        string
		requestPath  string
		expectParams Params
	}{
		{
			route:       "/users/:id",
			requestPath: "/users/123",
			expectParams: Params{
				{Key: "id", Value: "123"},
			},
		},
		{
			route:       "/posts/:postId/comments/:commentId",
			requestPath: "/posts/456/comments/789",
			expectParams: Params{
				{Key: "postId", Value: "456"},
				{Key: "commentId", Value: "789"},
			},
		},
		{
			route:       "/files/*filepath",
			requestPath: "/files/documents/report.pdf",
			expectParams: Params{
				{Key: "filepath", Value: "documents/report.pdf"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("Params for %s", tc.route), func(t *testing.T) {
			_, params := tree.Find(tc.requestPath)

			if !reflect.DeepEqual(params, tc.expectParams) {
				t.Errorf("Parameter extraction failed for %s\nGot:  %+v\nWant: %+v",
					tc.requestPath, params, tc.expectParams)
			}
		})
	}
}

func TestRouteNotFound(t *testing.T) {
	tree := NewNodeTree()

	// Add some routes
	tree.addRoute("/users", createHandlers(1))
	tree.addRoute("/posts/:id", createHandlers(1))

	// Test routes that shouldn't match
	notFoundPaths := []string{
		"/unknown",
		"/users/profile", // No handler for this specific path
		"/posts",         // Only "/posts/:id" is registered
		"/admin",
	}

	for _, path := range notFoundPaths {
		t.Run(fmt.Sprintf("Not found: %s", path), func(t *testing.T) {
			handlers, _ := tree.Find(path)

			if handlers != nil {
				t.Errorf("Expected no handlers for path %s, but got handlers", path)
			}
		})
	}
}

func TestNodeIsolation(t *testing.T) {
	// Create multiple method trees to ensure they're isolated
	getTree := NewNodeTree()
	getTree.Method = "GET"

	postTree := NewNodeTree()
	postTree.Method = "POST"

	// Add same path to different trees
	getTree.addRoute("/api/resource", createHandlers(1))
	postTree.addRoute("/api/resource", createHandlers(2)) // Different handler count

	// Verify that both trees have the route but with different handlers
	getHandlers, _ := getTree.Find("/api/resource")
	postHandlers, _ := postTree.Find("/api/resource")

	if len(getHandlers) != 1 {
		t.Errorf("GET tree should have 1 handler, got %d", len(getHandlers))
	}

	if len(postHandlers) != 2 {
		t.Errorf("POST tree should have 2 handlers, got %d", len(postHandlers))
	}
}
