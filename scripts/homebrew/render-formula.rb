# frozen_string_literal: true

abort "usage: render-formula.rb VERSION CHECKSUMS [ARCHIVE_ROOT]" unless (2..3).cover?(ARGV.length)
version, manifest, archive_root = ARGV
abort "invalid version" unless version.match?(/\A\d+\.\d+\.\d+(?:-[A-Za-z0-9.-]+)?\z/)
archive_root ||= "https://github.com/Kong/kongctl/releases/download/v#{version}"
unless archive_root == "https://github.com/Kong/kongctl/releases/download/v#{version}" ||
       archive_root.start_with?("file:///")
  abort "expected the official release URL or a local validation fixture"
end
checksums = {}
File.foreach(manifest) do |line|
  digest, filename = line.split
  next unless filename&.match?(/\Akongctl_(darwin|linux)_(amd64|arm64)\.zip\z/)

  abort "invalid/duplicate checksum for #{filename}" unless digest.match?(/\A[0-9a-f]{64}\z/) && !checksums.key?(filename)
  checksums[filename] = digest
end
%w[darwin linux].product(%w[arm64 amd64]).each do |os, arch|
  abort "missing #{os}/#{arch} checksum" unless checksums.key?("kongctl_#{os}_#{arch}.zip")
end
values = { "VERSION" => version }
%w[darwin linux].product(%w[arm64 amd64]).each do |os, arch|
  filename = "kongctl_#{os}_#{arch}.zip"
  values["#{os}_#{arch}_URL".upcase] = "#{archive_root}/#{filename}"
  values["#{os}_#{arch}_SHA".upcase] = checksums.fetch(filename)
end
template = File.read(File.join(__dir__, "kongctl.rb.template"))
puts template.gsub(/@([A-Z0-9_]+)@/) { values.fetch(Regexp.last_match(1)).dump }
