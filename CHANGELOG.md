# k8s-blueprint-operator Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [v1.0.0] - 2026-04-07
### Added
- [#3] CAS-backed service registration for `CAS`, `OIDC`, and `OAUTH`
  - operator startup now creates a real CAS client and backend instead of using the noop backend

## [v0.1.0] - 2026-03-03
### Added
- [#1] Initial setup of the operator
  - only uses a noop registration-backend
