// Package config provides a generic configuration loader for the kat application.
//
// It provides a [Loader] type that can load, validate, and parse configuration
// files in YAML format for any type implementing [github.com/macropower/kat/api/v1beta1.Object].
//
// Configuration types are defined in sub-packages of [github.com/macropower/kat/api/v1beta1]:
//   - [github.com/macropower/kat/api/v1beta1/configs] - Global configuration
//   - [github.com/macropower/kat/api/v1beta1/policies] - Policy configuration
//   - [github.com/macropower/kat/api/v1beta1/projectconfigs] - Project configuration
package config
