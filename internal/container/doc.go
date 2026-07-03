// Package container provides Docker and Podman container management.
//
// # How it works (no Docker SDK, no client library)
//
// Everything is done via exec.Command to docker/podman CLI. This means:
//   - Zero dependencies beyond the runtime binary
//   - Works with any docker-compatible CLI (docker, podman, nerdctl)
//   - No API version compatibility issues
//   - No socket permissions headaches (uses the same perms as the user's shell)
//
// # Adding a new container action
//
//  1. Add a function here (e.g. Exec, Pause, Rename)
//  2. Wire it into the TUI in tui.go (add a key binding + action)
//  3. That's it — no interface changes, no registries
//
// # Adding support for another runtime (e.g. nerdctl, crictl)
//
// Just add the binary name to the loop in List():
//
//	for _, rt := range []string{"docker", "podman", "nerdctl"} { ... }
//
// The --format flag is docker-compatible across all these tools.
//
// # Why not use the Docker Go SDK?
//
// For a TUI dashboard that lists/starts/stops containers, shelling out is
// simpler, has fewer deps, and works identically for Docker and Podman.
// The SDK would add ~20 transitive deps for the same 5 lines of output parsing.
package container
