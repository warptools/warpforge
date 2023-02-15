// The subcmd package is where the actual business logic for warpforge commands resides.
// This differs from the cmd/warpforge directory which is specifically a CLI application.
// The purpose is to create a clear split between the CLI application concerns and command domain logic.
//
// No packages outside of cmd/ or subcmd/ is expected to import packages from here. This set of packages is where
// we stitch together the other pieces of warpforge.
package subcmd
