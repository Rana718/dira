// Package ports provides network socket scanning.
//
// On Linux it uses ss(8), on macOS it uses lsof(8).
// Both produce the same []Entry output for the cmd layer.
package ports
