# Canonical identity of the release and every asset. Notes may change; assets
# must not change between native verification and publication.
def required_assets:
  ["checksums.txt",
   "kongctl_darwin_amd64.zip", "kongctl_darwin_arm64.zip",
   "kongctl_linux_amd64.zip", "kongctl_linux_arm64.zip",
   "kongctl_windows_amd64.zip", "kongctl_windows_arm64.zip"];
if .tag_name != $tag or .prerelease != false or
   (.id | type) != "number" or
   (.draft | type) != "boolean" or
   ((required_assets - [.assets[].name]) | length) != 0 or
   ([.assets[].name] | length) != ([.assets[].name] | unique | length) or
   any(.assets[]; (.id | type) != "number" or .state != "uploaded" or .size <= 0)
then error("Invalid or incomplete stable release")
else {id, tag_name, draft, assets:
  [.assets[] | {id, name, size, digest, updated_at}] | sort_by(.name)}
end
