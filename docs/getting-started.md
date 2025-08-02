# Getting Started with `kongctl`

This guide gets you started with managing your API Platform using `kongctl`'s 
declarative configuration. You'll create a developer portal and publish an API 
in just a few steps.

## Prerequisites

1. **Kong Konnect Account**: [Sign up for free](https://konghq.com/products/kong-konnect/register)
2. **`kongctl` installed**: See [installation instructions](../README.md#installation)
3. **Authenticated with Konnect**: Run `kongctl login`

## Key Concepts

- **Resources**: Kong Konnect API Platform objects. For example APIs, Portals, 
  and Auth Strategies. More resource types will be added over time
- **References (ref)**: Identifier string in the declarative configuration which 
  uniquely identifies each resource
- **Plan**: Plans are artifacts that contain desired changes to resources. Plans 
  are the foundation to the declarative configuration feature, but using them 
  directly is optional
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

Now add an API and publish it to your portal. Add this to your `portal.yaml` 
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

Your complete `portal.yaml` file should now look like this:

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

Your developer portal and API are now live! Visit the Konnect dashboard to see 
your developer portal with the published API.

## Next Steps

To see a more complete example with Portal pages, API specifications and versions, see 
the portal example in [docs/examples/declarative/portal](docs/examples/declarative/portal).

## Troubleshooting

If you encounter issues:

- Check authentication: `kongctl get me`
- Enable debug logging: `kongctl apply -f portal.yaml --log-level debug`
- See the [Troubleshooting Guide](troubleshooting.md)
- Reach out to the community on [Kong Nation](https://discuss.konghq.com/)
