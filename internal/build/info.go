package build

type Key struct{}

var InfoKey = Key{}

type Info struct {
	Version, Commit, Date string
}
