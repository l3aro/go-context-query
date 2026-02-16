package main

import (
	"fmt"
	"go-utils/math"
)

// helper is a local helper function
func helper() int {
	return 42
}

// main is the entry point that calls functions from other files
func main() {
	// Call local helper
	result := helper()

	// Call function from utils/math package
	sum := math.Add(1, 2)

	// Call another function from utils
	product := math.Multiply(3, 4)

	fmt.Println(result, sum, product)
}

// callerFunction calls both local and external functions
func callerFunction() int {
	a := helper()
	b := math.Add(a, 10)
	c := math.Multiply(b, 2)
	return c
}
