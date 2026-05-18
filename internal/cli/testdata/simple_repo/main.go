package main

import "fmt"

// Greet prints a greeting for the given name.
func Greet(name string) {
	fmt.Println(Hello(name))
}

// Hello returns a greeting string.
func Hello(name string) string {
	return "Hello, " + name
}
