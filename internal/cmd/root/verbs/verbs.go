package verbs

const (
	Add    = VerbValue("add")
	Apply  = VerbValue("apply")
	Adopt  = VerbValue("adopt")
	Ask    = VerbValue("ask")
	Get    = VerbValue("get")
	Create = VerbValue("create")
	Dump   = VerbValue("dump")
	Update = VerbValue("update")
	Delete = VerbValue("delete")
	Help   = VerbValue("help")
	List   = VerbValue("list")
	Login  = VerbValue("login")
	Plan   = VerbValue("plan")
	Sync   = VerbValue("sync")
	Diff   = VerbValue("diff")
	Export = VerbValue("export")
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
