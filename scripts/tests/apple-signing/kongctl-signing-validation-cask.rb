# Ephemeral runner fixture, never published to the production tap.
cask "kongctl-signing-validation" do
  version "0.0.0"
  sha256 ENV.fetch("KONGCTL_SIGNING_VALIDATION_SHA256")

  url ENV.fetch("KONGCTL_SIGNING_VALIDATION_URL")
  name "kongctl Apple signing validation"
  desc "Validate preservation of kongctl Apple signatures during cask installation"
  homepage "https://github.com/Kong/kongctl"

  binary "kongctl"
end
