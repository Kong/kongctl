# Getting Started with `kongctl` Declarative Configuration

This guide gets you started with managing your API Platform using `kongctl`'s 
declarative configuration. You'll quickly create a configuration for a developer portal 
and an API and apply them to your Kong Konnect account.

## Prerequisites

1. **Kong Konnect Account**: [Sign up for free](https://konghq.com/products/kong-konnect/register)
2. **`kongctl` installed**: See [installation instructions](../README.md#installation)
3. **Authenticated with Konnect**: Run `kongctl login`

## Key Concepts

- **Resources**: Kong Konnect API Platform objects, for example APIs, Portals, 
  and Auth Strategies. 
- **References (ref)**: Identifier string _in the declarative configuration_ which 
  uniquely identifies each resource
- **Plan**: A plan is a JSON artifact that captures the exact changes to be made 
  to your resources. While `apply` and `sync` generate plans internally, you can 
  create explicit plan artifacts for review, audit, and deferred execution
- **Apply**: Execute changes to resources (create and update only)
- **Sync**: Full declarative reconciliation (create, update and delete). Not used in this guide


## Step 1: Create a Directory

Create a clean directory to work in:

```shell
mkdir kong-portal && cd kong-portal
```

## Step 2: Create Your Configuration File

Create a file named `portal.yaml` and build it step by step. Start with a public Developer Portal:

```yaml
portals:
  - ref: my-portal
    name: "my-developer-portal"
    display_name: "My Developer Portal"
    description: "API documentation for developers"
    authentication_enabled: false
    default_api_visibility: "public"
    default_page_visibility: "public"
```

## Step 3: Add an API and Publish it to Your Portal

Create an API and publish it to the portal. Add this to your `portal.yaml` 
file:

```yaml
apis:
  - ref: users-api
    name: "Users API"
    description: "API for user management"
    
    publications:
      - ref: users-api-publication
        portal_id: my-portal
```

## Step 4: Apply Your Configuration

Be sure to save the file. Your complete `portal.yaml` file should now look like this:

```yaml
portals:
  - ref: my-portal
    name: "my-developer-portal"
    display_name: "My Developer Portal"
    description: "API documentation for developers"
    authentication_enabled: false
    default_api_visibility: "public"
    default_page_visibility: "public"

apis:
  - ref: users-api
    name: "Users API"
    description: "API for user management"
    
    publications:
      - ref: users-api-publication
        portal_id: my-portal
```

Apply the configuration to create your portal and API:

```shell
kongctl apply -f portal.yaml
```

Review the changes before confirming the apply. `kongctl` reconciles the resources to match the desired state
and provides a summary of the changes made.

## Step 5: Verify Your Resources

Check that your resources were created:

List the developer portals in your organization:

```shell
kongctl get portals
```

List the APIs:

```shell
kongctl get apis
```

Your developer portal and API are now live! Visit the [Konnect Console](https://cloud.konghq.com/us/portals/) 
to see your developer portal with the published API.

## Next Steps

To see a more complete Developer Portal example with pages, API specifications and more check out the
[portal specific example](docs/examples/declarative/portal/README.md).

## Troubleshooting

If you encounter issues:

- Check authentication: `kongctl get me`
- Enable debug logging: `kongctl apply -f portal.yaml --log-level debug`
- See the [Troubleshooting Guide](troubleshooting.md)
- Reach out to the community on [Kong Nation](https://discuss.konghq.com/)
