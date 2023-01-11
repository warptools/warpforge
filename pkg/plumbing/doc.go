// The plumbing package is where the actual business logic for warpforge commands resides.
// This differs from the cmd/warpforge directory which is specifically a CLI application.
// The purpose is to create a clear split between CLI application  concerns and business logic of warpforge.
//
// No packages outside of cmd/ is expected to import plumbing packages. This set of packages is where
// we stitch together the other pieces of warpforge.
package plumbing
