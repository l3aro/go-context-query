package main

// HelperFunction returns a constant value
func HelperFunction() int {
	return 100
}

// AnotherHelper is used to test cross-file resolution
func AnotherHelper(x int) int {
	return x * 2
}
