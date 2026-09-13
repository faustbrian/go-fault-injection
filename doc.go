// Package faultinject provides deterministic, bounded fault injection for Go
// tests and explicitly wired controlled experiments.
//
// A zero Runtime is disabled. Built-in application APIs accept only a Runtime,
// which applies authorization, allowlisting, expiry, budget, audit, and
// terminal-disable controls before selecting a fault from an explicit,
// validated Injector.
package faultinject
