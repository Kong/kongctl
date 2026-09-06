# frozen_string_literal: true

require "json"
require "digest"

abort "usage: validate-bottles.rb DIRECTORY VERSION" unless ARGV.length == 2
directory, version = ARGV
abort "invalid release version" unless version.match?(/\A\d+\.\d+\.\d+(?:-[A-Za-z0-9.-]+)?\z/)
files = Dir[File.join(directory, "*.bottle.json")]
abort "expected three bottle metadata files" unless files.length == 3
tags = []
recipes = []
files.each do |file|
  document = JSON.parse(File.read(file))
  abort "unexpected formula" unless document.keys == ["kong/kongctl/kongctl"]
  data = document.fetch("kong/kongctl/kongctl")
  abort "wrong version" unless data.dig("formula", "pkg_version") == version
  bottle = data.fetch("bottle")
  abort "unexpected registry" unless bottle["root_url"] == "https://ghcr.io/v2/kong/kongctl"
  abort "unexpected rebuild" unless bottle["rebuild"] == 0
  abort "expected exactly one platform" unless bottle.fetch("tags").length == 1
  tag, entry = bottle.fetch("tags").first
  abort "unexpected platform" unless %w[arm64_sequoia sequoia x86_64_linux].include?(tag)
  tags << tag
  cellar = entry["cellar"] || bottle["cellar"]
  abort "bottle would require relocation" unless cellar == "any_skip_relocation"
  filename = entry.fetch("local_filename")
  expected = "kongctl--#{version}.#{tag}.bottle.tar.gz"
  abort "unexpected bottle filename" unless filename == expected
  digest = entry.fetch("sha256")
  unless digest.match?(/\A[0-9a-f]{64}\z/) && Digest::SHA256.file(File.join(directory, filename)).hexdigest == digest
    abort "bottle checksum mismatch"
  end
  recipe = File.read(File.join(directory, "kongctl-#{tag}.rb"))
  abort "not an upstream binary formula" unless recipe.include?('bin.install "kongctl"') &&
                                              !recipe.include?("depends_on") &&
                                              recipe.include?("/releases/download/v#{version}/") &&
                                              !recipe.include?("file://")
  recipes << recipe
end
abort "wrong or duplicate platforms" unless tags.sort == %w[arm64_sequoia sequoia x86_64_linux]
abort "platform recipes differ" unless recipes.uniq.length == 1
puts "Verified three relocatable bottles and matching upstream formula recipes"
