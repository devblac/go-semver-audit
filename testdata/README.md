# Test Data

This directory contains sample Go packages used for testing the analyzer.

## Structure

- `oldlib/` - Represents version 1.0.0 of a sample library
- `newlib/` - Represents version 2.0.0 of the same library (with breaking changes)
- `userproject/` - A sample project that uses the library

## Test Scenarios

The test data demonstrates:

1. **Removed functions** - Functions that existed in v1 but removed in v2
2. **Changed signatures** - Functions with modified parameters or return types
3. **Interface changes** - Interfaces with added/removed methods
4. **Added symbols** - New functions/types in v2 (non-breaking)
5. **Type changes** - Struct definitions that changed

These scenarios allow comprehensive testing of the diff engine and usage detection.

