
# Structure 

The CLI is designed with natural language in mind. It prefers commands and subcommands
to flags. The commands prefer full command names but also aliases for brevity. 

```sh
kongctl <verb> <product/area> <resource> --flag <flagvalue> <sub-resource> <resource-id/name>  
```

Total consistency is difficult, so some of that is traded away for natural language readability 
in the command structure. 

The `get` verb is somewhat unique in that it can be used as a get or a list operation depending
on if an identifier is provided as an argument to the command.

Kong often uses a UUID as the idenitifier. We will support lookup by both the UUID id and 
by a name _automatically_ as a command arg.  

For Konnect resources (as of 2024), looking up by name will require querying the "list" endpoints 
and then filtering locally to extract the data value.  On-Prem Kong Gateway entities can be retrieved 
directly by name.

The below command examples are just a sampling, the list of possible commands for all Kong
products and resources is quite large.  The goal here is to define a system.

# Profiles

Profiles are a way to group both Auth and Config values for the CLI based on an name identifier.
Users can create profiles to model their various organizations, permissions, and environments.

Profiles are stored across two files in the user's `XDG_CONFIG_HOME` directory:

```txt
$XDG_CONFIG_HOME/.kongctl/config.yaml
$XDG_CONFIG_HOME/.kongctl/credentials.yaml
```

The files can be overriden by command line flags or environment variable per invocation 

`--config-file`      or `KONGCTL_CONFIG_FILE` 
`--credentials-file` or `KONGCTL_CREDENTIALS_FILE`

This is intentional to separate configurations from secrets as users may want to 
deal with these files differently (for example, sharing the config file but not the credentials file)
or protecting the files differently.

Various file formats for profiles are supported: yaml, json, and toml (via Viper). These are the
hierarchical formats where the flat formats (env, java props) should not be used.

There is a built-in `default` profile that cannot be deleted. The CLI can be used without
specifying a profile at all.

Profile configuration values are meant to aid the user in setting common values
for a given profile.  For example, a user can store a control-plane name or ID as the default
for a given profile and avoid repeating it as they use the CLI. 

## Profile CRUD

```sh
# list all the profiles
kongctl get profiles
# Prints the specific profile's stored values
kongctl get profile my-profile
# create a new profile. Checks for existing profile with the same name
kongctl create profile my-profile
# import a profile from a file
kongctl import profile profile-file.yaml
# export a profile to a file. STDOUT is default if flag not provided
kongctl export profile my-profile --output-file profile-file.yaml
# delete a profile. Confirms with the user before deleting or override with --yes
kongctl delete profile my-profile
```

## Using Profiles

`--profile` is a global flag, whenever it is not specified `default` is used. Also support an
environment variable `KONGCTL_PROFILE` which is only overriden by the flag.

```sh
# example of specifying the profile via env var for an invocation
KONG_PROFILE=my-profile kongctl get konnect gateway service my-service-name
# example of specifying the profile via flag for an invocation
kongctl --profile my-profile get konnect gateway service my-service-name 
```

### Using Profile Auth 

The workflow for auth is to use the CLI to store auth values which are then used
when making requests to the corresponding service. These commands write the auth values to the
`credentials` file. The various commands are aware of the types of auth  methods each product uses
and will expect certain values with certain names for the auth to work properly. The commands guide the user
to these values.

```sh
# login is a special verb that should only work for Konnect.
#   It invokes the device auth grant browser based auth flow and 
#   stores the creds in the profile.
kongctl login konnect --profile my-profile
```

```sh
# Store a PAToken in the profile's konnect auth section
kongctl set konnect auth pat --profile my-profile pat_abc123

# Store a Kong Gateway Enterprise admin token (`Kong-Admin-Token` header)
# The kongctl get gateway services command knows how to look for this and use it in the header.
kongctl set gateway auth admin-token --profile my-profile vajeOlkbsn0q0VD9qw9B3nHYOErgY7b8
```

### Using Profile Config

Every option in the CLI has a cooresponding path based dot notation identifier. This matches the path
to the value in the config file and the structure of the commands they are a part of. 

For example, the `konnect gateway controlplane` command has an
"id" option that represents the Control Plane identifier, a uuid. The path to this value in the config system
is `.konnect.gateway.controlplane.id`.  In storage this value is stored under the profile key.
A user can likely discern the path to values without a lot of effort once they use the CLI for a short period.

These config values will have a flag to make UX smoother. These flag names
need to be unique across the CLI given how the commands are structured. For example CPs _and_ GW Services
have _id_ fields, so `id` will not work well as a flag.

Here are some example paths to flag names:
* `konnect.gateway.controlplane.id` == `cp-id`
* `konnect.gateway.controlplane.name` == `cp-name`
* `konnect.gateway.service.id` == `kk-gw-svc-id`
* `gateway.service.id` == `gw-svc-id`

Callers can specify the value of these using either the specifc flag name or the 
generic `--config path=value` flag. A caller can specify 0-N `--config` flags for any command.

The following will be equivalent:

```sh
kongctl get konnect gateway service \
    --config "konnect.gateway.controlplane.id=123e4567-e89b-12d3-a456-426614174000" my-service-name
kongctl get konnect gateway service --cp-id "123e4567-e89b-12d3-a456-426614174000" my-service-name
```

More helpfully, these config values can be stored in profiles averting the need to specify either of these
flags. For example, if the current or specified profile has `konnect.gateway.controlplane.id` set, 
the command can be reduced because the cp id will be read from the profile.

```sh
# example of using the default or current profile and a saved CP id config value. If
# the konnect.gateway.controlplane.id cannot be read, the command fails.
kongctl get konnect gateway service my-service-name
```

Flags supersede profile values. 

Environment variables are available for all flags, except `--config`. `--config` is 0-N so
env vars are not a viable solution.

For example:

```sh
KONGCTL_CP_ID=123e4567-e89b-12d3-a456-426614174000 kongctl get konnect gateway service my-service-name
# or
kongctl get konnect gateway service my-service-name --cp-id 123e4567-e89b-12d3-a456-426614174000
```

```sh
# Setting configuration values in a profile is done with the `set config` command:
kongctl set config --profile my-profile konnect.gateway.controlplane.id "123e4567-e89b-12d3-a456-426614174000"
```

```sh
# You can also retrieve the current value:
kongctl get config --profile my-profile konnect.gateway.controlplane.id
```

```sh
# delete a config value from a profile
kongctl delete config --profile my-profile konnect.gateway.controlplane.id
```

All of this can be useful for users who take advantage of the profile system to store common 
configurations and authorizations. Imagine a user who works most commonly on the "DEV" control plane in the
"inventory" organization. They can setup a "Inventory DEV" profile with the PAT for the org, and
the DEV control plane ID. Now they can use the CLI swiftly for common tasks but it's flexible 
for non-common tasks.

We should consider safety when writing the config values to the file. If the writing fails, users will 
lose valuable configuration data. Strategies like backing up before writing, or writing to a temp file
should be investigated. 

# Konnect

The Konnect product has is generally broken down into runtime types (gateway, KIC, mesh) but 
also provides other resources like teams, service accounts, analytics, logging, api products, 
notifications, etc...

## Gateway

### List Konnect Control Planes

The `get` verb is overloaded to list resources when the resource is plural. 

```sh
# assume the user has auth settings configured in a profile (default or "current").
# This will list all the control planes in the org
kongctl get konnect gateway control-planes 
# cps == alias for controlplanes
kongctl get konnect gateway cps
# using all aliases to get all konnect gateway controlplanes
kongctl get kk gw cps 
```

### Get Control Plane

```sh
# Get a specific control plane by name
kongctl get konnect gateway controlplane my-control-plane-name
# Get a specific control plane by ID. The UUID is detected in the argument via regex, 
#   otherwise the command assumes it's a name search
kongctl get konnect gateway controlplane 123e4567-e89b-12d3-a456-426614174000
# an example using aliases for each part of the command
kongctl get kk gw cp my-control-plane-name
```

### Create Control Plane

```sh
# Create a control plane with a name, description, type, and labels
#   Interestingly with this we could easily stage a DP docker run command 
#   for the user for a quick full setup
kongctl create konnect gateway controlplane my-control-plane-name \
    --description "My control plane" \
    --cluster-type CLUSTER_TYPE_CONTROL_PLANE \
    --labels "key1=value1,key2=value2"
# Or experiment with a command that does both for the user. 
# The following creates a CP with a unique name and invokes a docker run
# command to connect a DP to the CP and maybe setup a Profile for the user
# with the CP and DP Ids.  Then we could give the user easy follow on commands
# to create entities. 
kongctl create konnect gateway quickstart 
```

### List Konnect Gateway Services

```sh
kongctl get konnect gateway services --cp-id 123e4567-e89b-12d3-a456-426614174000
# or if the user has mapped a CP ID in the profile
kongctl get konnect gateway services 
```

### Get Konnect Gateway Service

```sh
# get a gateway service by name from a specificed control plane using the cp-name specific flag
#  This will require a CP lookup query then the GW service lookup to grab the CP ID
kongctl get konnect gateway service --cp-name my-control-plane my-service-name
# A longer winded way of writing the above is:
kongctl get konnect gateway service \
    --config konnect.gateway.controplane.name=my-control-plane my-control-plane \
    my-service-name
# But assuming a user has CP id or name specified in the current profile, this can be
kongctl get konnect gateway service my-service-name
# The command can also accept the ID in the final argument. This continues to assume
# the CP ID is in the profile or current profile
kongctl get konnect gateway service 123e4567-e89b-12d3-a456-426614174000 
# Aliases can make it as short as
kongctl get kk gw svc my-service-name
```

## Declarative Config

Not sure what this would be given terraform.

```sh
kongctl apply konnect main.tf
```

## Search

```sh
kongctl search konnect "type:control_plane AND name:my-control-plane"
```

## API Products

```sh
# List API products
kongctl get konnect apiproducts
kongctl get konnect prods
```

```sh
# Get specific API Product
kongctl get konnect apiproduct my-api-product-name
```

```sh
# list API Product docs
kongctl get konnect apiproducts docs
# get specific API product doc
kongctl get konnect apiproduct --product-id 123e4567-e89b-12d3-a456-426614174000 \
  doc 123e4567-e89b-12d3-a456-426614174000
```

# Gateway (on-prem)

### List gateway services

```sh
kongctl get gateway services
# Can the alias be the same as the kk gateway service command?
kongctl get gateway svc 
```

### Get Gateway Service

```sh
kongctl get gateway service 123e4567-e89b-12d3-a456-426614174000
kongctl get gateway service my-service-name 
```

### List Gateway Routes

```sh
# list all routes
kongctl get gateway routes
# list routes for a specific service
#    Flag to profile config : --service-id == gateway.service.id 
kongctl get gateway routes --svc-id 123e4567-e89b-12d-3a456-426614174000
# list routes for a specific service
#    Flag to profile config : --service-name == gateway.service.name
kongctl get gateway routes --svc-id my-service-name
```

### Get Gateway Route
```sh
# Get a route by ID directly
kongctl get gateway route 123e4567-e89b-12d3-a456-426614174000
# Get a route by name for a specific service 
kongctl get gateway route --svc-id 123e4567-e89b-12d3-a456-426614174000 my-route-name
```

### Declarative Configuration

Up for debate, decK does this today. The new CLi would need to integrate
the db reconciler library and provide a friction free migration experience
for existing deck users.

```sh
kongctl apply gateway deck-file.yaml deck-file2.yaml
kongctl diff gateway deck-file.yaml deck-file2.yaml
kongctl dump gateway out-file.yaml
```

# APIOps

APIOps commands generally "operate over files". So we have a few verbs that 
are related to working on files. These may be declarative configuration files, 
API Specifications, etc...

```sh
kongctl transform file kong2kic
kongctl transform file add-plugins
kongctl transform file kong2kic
kongctl transform file merge
kongctl lint file
kongctl validate file
```

# Mesh

TBD

# Sample Verbs

get\
set\
create\
delete\
search\
apply\
transform

# Sample Aliases

kk      == konnect\
gw      == gateway\

cp      == controlplane\
cps     == controlplanes\

svc     == service\
svcs    == services\

prod    == apiproduct\
prods   == apiproducts\

rt      == route\
rts     == routes\

