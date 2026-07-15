package internal

// LinkedExtensionInstallMessage improves post-install UX (issue #1617).
func LinkedExtensionInstallMessage(name string) string {
        if name == "" {
                name = "extension"
        }
        return "Installed linked " + name + ". Next: run `kongctl extension list` to verify it is active, then restart the gateway if it was already running."
}
