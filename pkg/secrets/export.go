// Package secrets provides utilities for secure export of Pulumi outputs.
// All exports are encrypted using the configured passphrase (PULUMI_CONFIG_PASSPHRASE).
//
// Usage:
//
//	// Option 1: Use SecretExporter for multiple exports
//	exporter := secrets.NewSecretExporter(ctx)
//	exporter.Export("key", value)
//
//	// Option 2: Use standalone function for single exports
//	secrets.Export(ctx, "key", value)
package secrets

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Export is a convenience function that exports a value as an encrypted secret.
// This is equivalent to ctx.Export(name, pulumi.ToSecret(value)).
func Export(ctx *pulumi.Context, name string, value pulumi.Input) {
	ctx.Export(name, pulumi.ToSecret(value))
}

// ExportString exports a string value as an encrypted secret.
func ExportString(ctx *pulumi.Context, name string, value string) {
	ctx.Export(name, pulumi.ToSecret(pulumi.String(value)))
}

// ExportInt exports an integer value as an encrypted secret.
func ExportInt(ctx *pulumi.Context, name string, value int) {
	ctx.Export(name, pulumi.ToSecret(pulumi.Int(value)))
}

// ExportBool exports a boolean value as an encrypted secret.
func ExportBool(ctx *pulumi.Context, name string, value bool) {
	ctx.Export(name, pulumi.ToSecret(pulumi.Bool(value)))
}

// ExportMap exports a map value as an encrypted secret.
func ExportMap(ctx *pulumi.Context, name string, value pulumi.Map) {
	ctx.Export(name, pulumi.ToSecret(value))
}

// SecretExporter provides methods to export all values as encrypted secrets.
// This ensures that sensitive data in the Pulumi state is always encrypted
// using the configured passphrase.
type SecretExporter struct {
	ctx *pulumi.Context
}

// NewSecretExporter creates a new SecretExporter instance.
func NewSecretExporter(ctx *pulumi.Context) *SecretExporter {
	return &SecretExporter{ctx: ctx}
}

// Export exports a value as an encrypted secret.
// All values are wrapped with pulumi.ToSecret() to ensure encryption.
func (e *SecretExporter) Export(name string, value pulumi.Input) {
	e.ctx.Export(name, pulumi.ToSecret(value))
}

// ExportString exports a string value as an encrypted secret.
func (e *SecretExporter) ExportString(name string, value string) {
	e.ctx.Export(name, pulumi.ToSecret(pulumi.String(value)))
}

// ExportInt exports an integer value as an encrypted secret.
func (e *SecretExporter) ExportInt(name string, value int) {
	e.ctx.Export(name, pulumi.ToSecret(pulumi.Int(value)))
}

// ExportBool exports a boolean value as an encrypted secret.
func (e *SecretExporter) ExportBool(name string, value bool) {
	e.ctx.Export(name, pulumi.ToSecret(pulumi.Bool(value)))
}

// ExportMap exports a map value as an encrypted secret.
func (e *SecretExporter) ExportMap(name string, value pulumi.Map) {
	e.ctx.Export(name, pulumi.ToSecret(value))
}

// ExportStringOutput exports a StringOutput as an encrypted secret.
func (e *SecretExporter) ExportStringOutput(name string, value pulumi.StringOutput) {
	e.ctx.Export(name, pulumi.ToSecret(value))
}

// ExportOutput exports any Output as an encrypted secret.
func (e *SecretExporter) ExportOutput(name string, value pulumi.Output) {
	e.ctx.Export(name, pulumi.ToSecret(value))
}

// SecretMap creates a pulumi.Map where all values are wrapped as secrets.
func SecretMap(m map[string]pulumi.Input) pulumi.Map {
	result := pulumi.Map{}
	for k, v := range m {
		result[k] = pulumi.ToSecret(v)
	}
	return result
}

// ToSecretMap wraps an existing pulumi.Map to have all its values as secrets.
func ToSecretMap(m pulumi.Map) pulumi.Output {
	return pulumi.ToSecret(m)
}

// SecretStringMap creates a map from string keys and values, all as secrets.
func SecretStringMap(m map[string]string) pulumi.Map {
	result := pulumi.Map{}
	for k, v := range m {
		result[k] = pulumi.ToSecret(pulumi.String(v))
	}
	return result
}
