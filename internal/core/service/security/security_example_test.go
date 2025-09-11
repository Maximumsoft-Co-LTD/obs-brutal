package security

import "fmt"

// Example demonstrating masking of Thai ID, email, and phone
func Example() {
	masker := NewPIIMasker()
	thaiID := "1-2345-67890-12-3"
	email := "john.doe@example.com"
	phone := "+66 89-123-4567"

	masked := masker.MaskFields(map[string]interface{}{
		"thai_id": thaiID,
		"email":   email,
		"phone":   phone,
	})

	fmt.Println(masked["thai_id"])
	fmt.Println(masked["email"])
	fmt.Println(masked["phone"])
	// Output:
	// x-xxxx-xxxxx-xx-x
	// ***@***.***
	// xxx-xxx-xxxx
}
