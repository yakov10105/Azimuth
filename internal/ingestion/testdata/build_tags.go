//go:build linux || darwin

package main

// platformSpecific returns the platform family name.
func platformSpecific() string {
	return "unix"
}
