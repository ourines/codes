package commands

import (
	"fmt"
	"strings"

	"codes/internal/config"
	"codes/internal/output"
	"codes/internal/remote"
	"codes/internal/ui"
)

// parseSSHAddress parses "user@host" or "host" into user and host parts.
func parseSSHAddress(address string) (user, host string) {
	if i := strings.Index(address, "@"); i >= 0 {
		return address[:i], address[i+1:]
	}
	return "", address
}

// RunRemoteAdd adds a new remote host.
func RunRemoteAdd(name, address string, port int, identity string) {
	user, host := parseSSHAddress(address)

	rh := config.RemoteHost{
		Name:     name,
		Host:     host,
		User:     user,
		Port:     port,
		Identity: identity,
	}

	if output.JSONMode {
		if err := config.AddRemote(rh); err != nil {
			output.PrintError(err)
			return
		}
		output.Print(map[string]interface{}{"added": true, "name": name}, nil)
		return
	}

	if err := config.AddRemote(rh); err != nil {
		ui.ShowError("Failed to add remote", err)
		return
	}

	ui.ShowSuccess("Remote '%s' added successfully!", name)
	ui.ShowInfo("Host: %s", rh.UserAtHost())
	if port != 0 {
		ui.ShowInfo("Port: %d", port)
	}
	if identity != "" {
		ui.ShowInfo("Identity: %s", identity)
	}
}

// RunRemoteRemove removes a remote host.
func RunRemoteRemove(name string) {
	if output.JSONMode {
		if err := config.RemoveRemote(name); err != nil {
			output.PrintError(err)
			return
		}
		output.Print(map[string]interface{}{"removed": true, "name": name}, nil)
		return
	}

	if err := config.RemoveRemote(name); err != nil {
		ui.ShowError("Failed to remove remote", err)
		return
	}
	ui.ShowSuccess("Remote '%s' removed", name)
}

// RunRemoteList lists all remote hosts.
func RunRemoteList() {
	remotes, err := config.ListRemotes()
	if err != nil {
		if output.JSONMode {
			output.PrintError(err)
			return
		}
		ui.ShowError("Failed to list remotes", err)
		return
	}

	if output.JSONMode {
		output.Print(remotes, nil)
		return
	}

	if len(remotes) == 0 {
		ui.ShowInfo("No remote hosts configured")
		ui.ShowInfo("Add a remote with: codes remote add <name> <[user@]host>")
		return
	}

	fmt.Println()
	ui.ShowHeader("Remote Hosts")
	fmt.Println()

	for i, r := range remotes {
		info := r.UserAtHost()
		if r.Port != 0 {
			info += fmt.Sprintf(":%d", r.Port)
		}
		ui.ShowInfo("%d. %s → %s", i+1, r.Name, info)
	}

	fmt.Println()
	ui.ShowInfo("Check status with: codes remote status <name>")
}

// RunRemoteStatus shows the status of a remote host.
func RunRemoteStatus(name string) {
	host, ok := config.GetRemote(name)
	if !ok {
		if output.JSONMode {
			output.PrintError(fmt.Errorf("remote %q not found", name))
			return
		}
		ui.ShowError(fmt.Sprintf("Remote '%s' not found", name), nil)
		return
	}

	if !output.JSONMode {
		ui.ShowInfo("Checking %s (%s)...", name, host.UserAtHost())
	}

	if err := remote.TestConnection(host); err != nil {
		if output.JSONMode {
			output.PrintError(err)
			return
		}
		ui.ShowError("Connection failed", err)
		return
	}

	status, err := remote.CheckRemoteStatus(host)
	if err != nil {
		if output.JSONMode {
			output.PrintError(err)
			return
		}
		ui.ShowError("Failed to check status", err)
		return
	}

	if output.JSONMode {
		output.Print(status, nil)
		return
	}

	fmt.Println()
	ui.ShowSuccess("Connection: OK")
	ui.ShowInfo("OS: %s", status.OS)
	ui.ShowInfo("Arch: %s", status.Arch)

	if status.CodesInstalled {
		ui.ShowSuccess("codes: installed (%s)", status.CodesVersion)
	} else {
		ui.ShowWarning("codes: not installed")
	}

	if status.ClaudeInstalled {
		ui.ShowSuccess("claude: installed")
	} else {
		ui.ShowWarning("claude: not installed")
	}
}

// RunRemoteInstall installs codes on a remote host.
func RunRemoteInstall(name string) {
	host, ok := config.GetRemote(name)
	if !ok {
		ui.ShowError(fmt.Sprintf("Remote '%s' not found", name), nil)
		return
	}

	ui.ShowLoading("Installing codes on %s...", host.UserAtHost())

	out, err := remote.InstallOnRemote(host)
	if err != nil {
		ui.ShowError("Installation failed", err)
		return
	}
	if out != "" {
		fmt.Println(out)
	}

	ui.ShowSuccess("codes installed on %s!", host.UserAtHost())
}

// RunRemoteSync syncs profiles to a remote host.
func RunRemoteSync(name string) {
	host, ok := config.GetRemote(name)
	if !ok {
		ui.ShowError(fmt.Sprintf("Remote '%s' not found", name), nil)
		return
	}

	ui.ShowLoading("Syncing profiles to %s...", host.UserAtHost())

	if err := remote.SyncProfiles(host); err != nil {
		ui.ShowError("Sync failed", err)
		return
	}

	ui.ShowSuccess("Profiles synced to %s!", host.UserAtHost())
}

// RunRemoteSetup runs install + sync on a remote host.
func RunRemoteSetup(name string) {
	host, ok := config.GetRemote(name)
	if !ok {
		ui.ShowError(fmt.Sprintf("Remote '%s' not found", name), nil)
		return
	}

	ui.ShowLoading("Installing codes on %s...", host.UserAtHost())
	if _, err := remote.InstallOnRemote(host); err != nil {
		ui.ShowError("Installation failed", err)
		return
	}
	ui.ShowSuccess("codes installed!")

	ui.ShowLoading("Installing Claude CLI...")
	out, err := remote.InstallClaudeOnRemote(host)
	if err != nil {
		ui.ShowWarning("Claude CLI: %v", err)
	} else {
		if strings.Contains(out, "already installed") {
			ui.ShowSuccess("Claude CLI already installed")
		} else {
			ui.ShowSuccess("Claude CLI installed!")
		}
	}

	ui.ShowLoading("Syncing profiles...")
	if err := remote.SyncProfiles(host); err != nil {
		ui.ShowError("Sync failed", err)
		return
	}
	ui.ShowSuccess("Profiles synced!")

	fmt.Println()
	ui.ShowSuccess("Remote '%s' is ready!", name)
	ui.ShowInfo("Connect with: codes remote ssh %s", name)
}

// RunRemoteSSH opens an interactive SSH session on the remote host.
func RunRemoteSSH(name string, project string) {
	host, ok := config.GetRemote(name)
	if !ok {
		ui.ShowError(fmt.Sprintf("Remote '%s' not found", name), nil)
		return
	}

	var cmd string
	if project != "" {
		cmd = fmt.Sprintf("cd %s && codes", project)
	} else {
		cmd = "codes"
	}

	if err := remote.RunSSHInteractive(host, cmd); err != nil {
		ui.ShowError("SSH session failed", err)
	}
}
