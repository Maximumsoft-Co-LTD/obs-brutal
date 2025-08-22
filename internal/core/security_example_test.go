package core

import "fmt"

// Example demonstrating masking of Thai ID, email, and phone
func Example() {
	masker := NewPIIMasker()
	thaiID := "1-2345-67890-12-3"
	email := "john.doe@example.com"
	phone := "+66 89-123-4567"

	fmt.Println(masker.maskStringPatterns(thaiID))
	fmt.Println(masker.maskStringPatterns(email))
	fmt.Println(masker.maskStringPatterns(phone))
	// Output:
	// x-xxxx-xxxxx-xx-x
	// ***@***.***
	// xxx-xxx-xxxx
}
