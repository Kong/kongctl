package verbs

const (
	Add    = VerbValue("add")
	Apply  = VerbValue("apply")
	Get    = VerbValue("get")
	Create = VerbValue("create")
	Update = VerbValue("update")
	Delete = VerbValue("delete")
	Help   = VerbValue("help")
	List   = VerbValue("list")
	Login  = VerbValue("login")
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
