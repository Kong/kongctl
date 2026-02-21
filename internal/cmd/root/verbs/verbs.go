package verbs

import (
	"fmt"

	"github.com/spf13/cobra"
)

const (
	Add    = VerbValue("add")
	Apply  = VerbValue("apply")
	Adopt  = VerbValue("adopt")
	Kai    = VerbValue("kai")
	API    = VerbValue("api")
	Get    = VerbValue("get")
	Create = VerbValue("create")
	Dump   = VerbValue("dump")
	Update = VerbValue("update")
	Delete = VerbValue("delete")
	Help   = VerbValue("help")
	List   = VerbValue("list")
	Login  = VerbValue("login")
	Logout = VerbValue("logout")
	Plan   = VerbValue("plan")
	View   = VerbValue("view")
	Sync   = VerbValue("sync")
	Diff   = VerbValue("diff")
	Export = VerbValue("export")
	Patch  = VerbValue("patch")
)

// Empty type to represent the _type_ Verb. Genesis is to support a key in a Context
type VerbKey struct{}

// Verb is a global instance of the VerbKey type
var Verb = VerbKey{}

// Will represent a specific Verb (get, create, update, delete, etc)
type VerbValue string

func (v VerbValue) String() string {
	return string(v)
}

// NoPositionalArgs returns an Args validator that rejects positional arguments
// with a helpful message directing users to use the -f/--filename flag instead.
// Use this for commands (e.g. plan, diff, sync) that accept input only via flags.
func NoPositionalArgs(_ *cobra.Command, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unexpected argument %q: use -f/--filename to specify input files", args[0])
	}
	return nil
}
