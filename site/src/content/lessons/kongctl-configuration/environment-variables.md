---
title: Environment Variables
summary: Set profile configuration with environment variables.
order: 4
---

## Goal

Learn how `kongctl` reads environment variables for different profiles based
on the variable name.

## Environment variables and profiles

Every configuration path can also be set with an environment variable:

> **Environment variable pattern**
>
> `KONGCTL_<PROFILE>_<CONFIGURATION_PATH>`

Uppercase the profile and configuration path, then replace dots and hyphens
with underscores. Examples from the previous lesson's `default` profile:

| Configuration path  | Environment variable                |
| ------------------- | ----------------------------------- |
| `output`            | `KONGCTL_DEFAULT_OUTPUT`            |
| `konnect.page-size` | `KONGCTL_DEFAULT_KONNECT_PAGE_SIZE` |
| `konnect.pat`       | `KONGCTL_DEFAULT_KONNECT_PAT`       |

The access token variable shown in the installation lesson now makes more sense:
`KONGCTL_DEFAULT_KONNECT_PAT` sets `konnect.pat` for the `default` profile.
It accepts either a personal access token (PAT) or a system account access
token (SPAT).
