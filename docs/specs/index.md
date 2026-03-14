# Outport Specification

> This is the authoritative specification for how Outport works. Each subsystem has its own detailed spec. This document provides the system overview and ties them together.

## Status: Draft

This spec is being developed alongside the tool. Sections marked with _(placeholder)_ are not yet written.

## Audience

_(placeholder — who is this tool for, what does their day look like, what skill level do we assume)_

## Problems We Solve

_(placeholder — explicit list of challenges this tool addresses, with concrete scenarios)_

## Problems We Don't Solve

_(placeholder — what's out of scope, where does Outport's responsibility end)_

## System Overview

_(placeholder — how the pieces connect: config → allocation → registry → .env → DNS → proxy → tunnels)_

## Design Principles

_(placeholder — zero thinking, .env is the contract, framework-agnostic, worktree-native, single binary)_

## Subsystem Specs

| Spec | Status | Description |
|------|--------|-------------|
| [Configuration](configuration.md) | Draft | Config format, fields, rules, edge cases |
| [Allocation](allocation.md) | Placeholder | Port assignment algorithm, deterministic hashing, registry |
| [DNS & Proxy](dns-proxy.md) | Placeholder | Friendly hostnames, reverse proxy, TLD choice |
| [SSL](ssl.md) | Placeholder | Certificates, ACME DNS-PERSIST-01, CertMagic |
| [Sharing](sharing.md) | Placeholder | Tunneling, QR codes, multi-service orchestration |
