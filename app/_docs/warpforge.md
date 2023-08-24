---
title: "wf CLI ref: warpforge"
layout: base.njk
eleventyNavigation:
    parent: Warpforge CLI
    order: 40
---


`warpforge` command reference
=============================

[testmark]:# (docs)
```clidoc
## NAME
warpforge - the everything-builder and any-environment manager

## USAGE
See subcommands for details.

## VERSION
v0.4.0

## COMMANDS
### catalog
Subcommands that operate on catalogs

### check
Check file(s) for syntax and sanity

### ferk
Starts a containerized environment for interactive use

### healthcheck
Check for potential errors in system configuration

### plan
Runs planning commands to generate inputs

### quickstart
Generate a basic module and plot

### run
Run a module or formula

### spark
Experimental RPC for getting module build status from the watch server

### status, info
Get status of workspaces and installation

### ware
Subcommands that operate on wares

### watch
Watch a module for changes to plot ingest inputs. Currently only git ingests are supported.

### help, h
Shows a list of commands or help for one command

## GLOBAL OPTIONS
#### --verbose, -v
(default: **false**)

(env var: $**WARPFORGE_DEBUG**)

#### --quiet
(default: **false**)

#### --json
Enable JSON API output

(default: **false**)

#### --trace.file=<VALUE>
Enable tracing and emit output to file

#### --trace.http.enable
Enable remote tracing over http

(default: **false**)

#### --trace.http.insecure
Allows insecure http

(default: **false**)

#### --trace.http.endpoint=<VALUE>
Sets an endpoint for remote open-telemetry tracing collection

#### --help, -h
show help

#### --version
print the version

```
