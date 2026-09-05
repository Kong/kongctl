# Render ordinary literals: Homebrew clears environment variables on load.
template = File.read(ARGV.fetch(0))
rendered = template.gsub(/ENV.fetch\("([A-Z0-9_]+)"\)/) do
  ENV.fetch(Regexp.last_match(1)).dump
end
abort "unresolved fixture environment expression" if rendered.include?("ENV.fetch")
puts rendered
