"""Bounded organization-user inputs and recording-only identity pseudonyms.

Live scenarios keep their environment-provided lookup selectors. Offline runs
use reserved example.invalid addresses; no real identity mapping is persisted.
"""

import re


USER_ENV = tuple(f"KONGCTL_E2E_ORG_USER_EMAIL_{index}" for index in range(1, 4))
USER_SCENARIOS = {
    "dump/organization-teams": USER_ENV[:1],
    "org/users/assignments": USER_ENV,
    "org/users/get": USER_ENV[:2],
    "org/users/plan/apply-workflow": USER_ENV[:2],
    "org/users/plan/sync-workflow": USER_ENV[:2],
    "org/users/sync": USER_ENV[:2],
}
EMAIL = re.compile(r"[\w.+-]+@[\w.-]+\.[a-z]{2,}", re.I)
SYNTHETIC_EMAIL = re.compile(r"replay-user-[1-9][0-9]*@example\.invalid")
USER_FIELDS = {"id", "email", "full_name", "preferred_name", "active",
               "inferred_region", "created_at", "updated_at"}


def replay_inputs(scenario):
    return {name: f"replay-user-{USER_ENV.index(name) + 1}@example.invalid"
            for name in USER_SCENARIOS.get(scenario, ())}


def recording_inputs(scenario, environ):
    values = {name: environ.get(name, "").strip() for name in USER_SCENARIOS.get(scenario, ())}
    if (any(not EMAIL.fullmatch(value) or SYNTHETIC_EMAIL.fullmatch(value) for value in values.values())
            or len(set(value.lower() for value in values.values())) != len(values)):
        raise ValueError("recording requires distinct existing organization user email inputs")
    return values


def recording_org(scenario):
    if scenario == "dump/portal-owned":
        # Match this scenario's successful live shard. Acceptance-3 injects a
        # default auth-strategy ID that cannot survive its reset round trip.
        return "kongctl-acceptance-2"
    return "kongctl-acceptance" if scenario in USER_SCENARIOS else "kongctl-acceptance-3"


def check_user_profiles(value):
    """Reject unsanitized or expanded user records even in edited cassettes."""
    if isinstance(value, dict):
        if {"email", "full_name", "preferred_name"} & value.keys():
            email = value.get("email")
            if (set(value) - USER_FIELDS or not isinstance(email, str)
                    or not SYNTHETIC_EMAIL.fullmatch(email)):
                raise ValueError("unreviewed or unsanitized organization user profile")
            label = email.split("@", 1)[0]
            if any(value.get(key) not in (None, "", "n/a", label) for key in ("full_name", "preferred_name")):
                raise ValueError("unsanitized organization user name")
            if any(value.get(key) not in (None, "2026-01-01T00:00:00Z") for key in ("created_at", "updated_at")):
                raise ValueError("unsanitized organization user timestamp")
        for item in value.values():
            check_user_profiles(item)
    elif isinstance(value, list):
        for item in value:
            check_user_profiles(item)


class UserIdentities:
    """Pseudonymize users without filtering collections or changing relations."""

    def __init__(self, inputs):
        self.emails = {value.lower(): f"replay-user-{USER_ENV.index(name) + 1}@example.invalid"
                       for name, value in inputs.items()}
        self.next_id = len(USER_ENV) + 1

    def email(self, value):
        key = value.lower()
        if key not in self.emails:
            self.emails[key] = f"replay-user-{self.next_id}@example.invalid"
            self.next_id += 1
        return self.emails[key]

    def normalize(self, value):
        if isinstance(value, dict):
            if "email" in value:
                if (set(value) - USER_FIELDS or not isinstance(value["email"], str)
                        or not EMAIL.fullmatch(value["email"])):
                    raise ValueError("unreviewed organization user profile schema")
                email = self.email(value["email"])
                label = email.split("@", 1)[0]
                value = dict(value)
                for key in ("full_name", "preferred_name"):
                    if value.get(key) not in (None, "", "n/a"):
                        if not isinstance(value[key], str):
                            raise ValueError("unreviewed organization user name schema")
                        value[key] = label
                # Account registration timestamps are identifying, not scenario
                # assertions. Preserve their presence/nullability and ISO shape.
                for key in ("created_at", "updated_at"):
                    if value.get(key) is not None:
                        value[key] = "2026-01-01T00:00:00Z"
            return {key: self.normalize(item) for key, item in value.items()}
        if isinstance(value, list):
            return [self.normalize(item) for item in value]
        if isinstance(value, str):
            return EMAIL.sub(lambda match: self.email(match.group()), value)
        return value
