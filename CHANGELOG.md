# k8s-blueprint-operator Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]
### Fixed
- [#8] Align service-registration-regex with the pattern from JSON-service-registry

## [v1.1.1] - 2026-05-12
### Fixed
- [#6] Allow optional ports in generated CAS registered service ID patterns.

## [v1.1.0] - 2026-04-10
### Changed
- [#5] Rename generated K8s resources to match the Helm release
   - if ldap-mapper is installed by an umbrella chart like `lop-idp` the deployed resources will receive a name like `lop-idp-ldap-mapper` instead of only `ldap-mapper`
- Update go libraries to newer patch versions
- (internal) Update Makefiles to 10.7.3

### Removed
- Remove excessive blank trimming from Helm template YAML files

### Fixed
- Backoff on errors in the reconciliation loop 

## [v1.0.0] - 2026-04-07
### Added
- [#3] CAS-backed service registration for `CAS`, `OIDC`, and `OAUTH`
  - operator startup now creates a real CAS client and backend instead of using the noop backend

## [v0.1.0] - 2026-03-03
### Added
- [#1] Initial setup of the operator
  - only uses a noop registration-backend
