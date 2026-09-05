# Ephemeral runner fixture, never published to the production tap.
class KongctlSigningValidation < Formula
  desc "Validate preservation of kongctl Apple signatures during bottling"
  homepage "https://github.com/Kong/kongctl"
  url ENV.fetch("KONGCTL_SIGNING_VALIDATION_URL")
  version "0.0.0"
  sha256 ENV.fetch("KONGCTL_SIGNING_VALIDATION_SHA256")
  license "Apache-2.0"

  def install
    bin.install "kongctl"
  end

  test do
    assert_match "kongctl", shell_output("#{bin}/kongctl version --full")
  end
end
