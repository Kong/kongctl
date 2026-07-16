# Capability maturity

`kongctl` is generally available (GA). Individual product areas and
capabilities can have a lower maturity without changing the maturity of the
rest of the CLI.

Supported maturity levels, from most to least mature, are:

- ga
- beta
- tech preview

An unlabeled capability is GA. `beta` and `tech preview` status applies only to
the command, flag, argument, accepted value, declarative resource, or resource
operation that carries the label. A non-GA command passes its maturity to its
subcommands, flags, and arguments unless a narrower capability is explicitly
less mature.

Non-GA interfaces may change before promotion. Maturity labels are discovery
information; they do not add warnings, prompts, opt-in flags, or runtime
gating. They do not change API requests, command exit codes, structured
execution output, declarative plan artifacts, or telemetry.

## Where maturity appears

Command listings, including missing-subcommand guidance, append `[beta]` or
`[tech preview]` when a child is less mature than the command being displayed.
Help for a non-GA command includes a `Maturity` section immediately after
`Usage`. Less-mature flags, arguments, and accepted values on an otherwise
more-mature command are listed as exceptions in that section.

`kongctl explain` reports maturity for every declarative resource. Its JSON
and YAML schemas expose the same information through
`x-kongctl-maturity`. Scaffolds for non-GA resources begin with maturity
comments; GA scaffold output is unchanged.

## Contributor policy

Attach metadata beside the command construction or declarative resource
registration that owns the capability. Put non-GA metadata at the highest
appropriate ancestor and rely on inheritance for descendants. Add narrower
metadata only when the child capability is less mature; do not duplicate
inherited metadata or attempt to raise an inherited level.

Promotion is performed by changing or removing the co-located override. Do
not maintain a separate catalog keyed by command paths or resource names.
