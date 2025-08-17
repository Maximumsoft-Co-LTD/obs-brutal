package example

import (
	"fmt"
	"log"

	lb "obs-brutal/pkg/obsvbrutal"

	"github.com/gin-gonic/gin"
)

// User struct with log tags
type User struct {
	ID       int    `json:"id" log:"user_id"`
	Name     string `json:"name" log:"name"`
	Email    string `json:"email" log:"email,sensitive=true"`
	Password string `json:"password" log:"-"` // Skip this field
	Age      int    `json:"age" log:"age"`
}

// Product struct
type Product struct {
	ID    int     `json:"id" log:"product_id"`
	Name  string  `json:"name" log:"product_name"`
	Price float64 `json:"price" log:"price"`
}

func RegisterSimpleDemo() {
	// Initialize logger
	logger, err := lb.NewLogger(
		lb.WithLevel(lb.InfoLevel),
	)
	if err != nil {
		log.Fatal("Failed to create logger:", err)
	}

	// Create Gin router
	router := gin.New()
	router.Use(lb.GinMiddleware(logger))

	// Register routes
	router.GET("/example/struct", handleStructExample)
	router.GET("/example/slice", handleSliceExample)
	router.GET("/example/types", handleTypesExample)

	log.Println("Starting server on :8080")
	log.Println("Try:")
	log.Println("  curl http://localhost:8080/example/struct")
	log.Println("  curl http://localhost:8080/example/slice")
	log.Println("  curl http://localhost:8080/example/types")
	router.Run(":8001")
}

// Example 1: Handle single struct
func handleStructExample(c *gin.Context) {
	lg := lb.GetLogFrmGin(c, "struct-example")
	defer lg.Close()

	// Example with single struct
	user := User{
		ID:       123,
		Name:     "John Doe",
		Email:    "john@example.com",
		Password: "secret123",
		Age:      30,
	}

	tracer := lg.FlatPr("user-operation")
	attrs := tracer.Body("user-data", user)

	// Add attributes to tracer
	tracer.Add(attrs...)
	lg.F("attributes_count", len(attrs)).Prt("Struct processed")
	tracer.End()

	// Response with pointer to struct
	userPtr := &User{
		ID:    456,
		Name:  "Jane Smith",
		Email: "jane@example.com",
		Age:   25,
	}

	// Use another tracer for response
	respTracer := lg.FlatPr("response-preparation")
	respAttrs := respTracer.Body("response-user", userPtr)
	respTracer.Add(respAttrs...)
	respTracer.End()

	c.JSON(200, gin.H{
		"message": "Single struct example",
		"user":    user,
		"attributes": gin.H{
			"count":   len(attrs),
			"details": fmt.Sprintf("%v", attrs),
		},
	})
}

// Example 2: Handle slice of structs
func handleSliceExample(c *gin.Context) {
	lg := lb.GetLogFrmGin(c, "slice-example")
	defer lg.Close()

	// Example with slice of structs
	products := []Product{
		{ID: 1, Name: "Laptop", Price: 999.99},
		{ID: 2, Name: "Mouse", Price: 29.99},
		{ID: 3, Name: "Keyboard", Price: 79.99},
	}

	tracer := lg.FlatPr("product-list-operation")
	attrs := tracer.Body("products", products)
	tracer.Add(attrs...)
	lg.F("products_count", len(products)).Prt("Product list processed")
	tracer.End()

	// Example with slice of pointers
	productPtrs := []*Product{
		{ID: 4, Name: "Monitor", Price: 299.99},
		{ID: 5, Name: "Headphones", Price: 149.99},
	}

	tracer2 := lg.FlatPr("product-ptr-list")
	attrs2 := tracer2.Body("product-ptrs", productPtrs)
	tracer2.Add(attrs2...)
	tracer2.End()

	// Example with empty slice
	var emptyProducts []Product
	tracer3 := lg.FlatPr("empty-list")
	attrs3 := tracer3.Body("empty-products", emptyProducts)
	tracer3.Add(attrs3...)
	tracer3.End()

	c.JSON(200, gin.H{
		"message":      "Slice examples",
		"products":     products,
		"product_ptrs": productPtrs,
		"empty":        emptyProducts,
		"attributes": gin.H{
			"slice_attrs": len(attrs),
			"ptr_attrs":   len(attrs2),
			"empty_attrs": len(attrs3),
		},
	})
}

// Example 3: Handle different types
func handleTypesExample(c *gin.Context) {
	lg := lb.GetLogFrmGin(c, "types-example")
	defer lg.Close()

	tracer := lg.FlatPr("different-types")

	// String
	strAttrs := tracer.Body("message", "Hello World")
	tracer.Add(strAttrs...)

	// Number
	numAttrs := tracer.Body("count", 42)
	tracer.Add(numAttrs...)

	// Boolean
	boolAttrs := tracer.Body("is_active", true)
	tracer.Add(boolAttrs...)

	// Map
	mapData := map[string]interface{}{
		"key1": "value1",
		"key2": 123,
		"key3": true,
	}
	mapAttrs := tracer.Body("metadata", mapData)
	tracer.Add(mapAttrs...)

	// Nil
	nilAttrs := tracer.Body("nil-value", nil)
	tracer.Add(nilAttrs...)

	tracer.End()

	c.JSON(200, gin.H{
		"message": "Different types example",
		"types": gin.H{
			"string":  "Hello World",
			"number":  42,
			"boolean": true,
			"map":     mapData,
			"nil":     nil,
		},
		"attributes_count": gin.H{
			"string":  len(strAttrs),
			"number":  len(numAttrs),
			"boolean": len(boolAttrs),
			"map":     len(mapAttrs),
			"nil":     len(nilAttrs),
		},
	})
}
