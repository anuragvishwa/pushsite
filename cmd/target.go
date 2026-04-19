package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/anuragvishwa/pushsite/internal/discovery"
	"github.com/anuragvishwa/pushsite/internal/target"
	"github.com/spf13/cobra"
)

var targetCmd = &cobra.Command{
	Use:   "target",
	Short: "Manage server targets",
	Long: `Save and manage server connection profiles so you never have
to type host, user, key, or instance ID again.

Targets are stored in ~/.pushsite/targets.yaml and can be used
with any command via the --target flag:

  pushsite deploy --target prod-web`,
}

var targetAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new target (auto-discovers AWS instances)",
	RunE:  runTargetAdd,
}

var targetListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all saved targets",
	Aliases: []string{"ls"},
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := target.Load("")
		if err != nil {
			return err
		}

		targets := store.List()
		if len(targets) == 0 {
			output.Info("No targets saved yet")
			output.Info("Run 'pushsite target add' to add one")
			return nil
		}

		output.Title("📍 Saved Targets")
		output.NewLine()

		for _, t := range targets {
			isDefault := ""
			if t.Name == store.Default {
				isDefault = " ← default"
			}

			conn := t.Method
			if t.Method == "ssm" {
				conn = fmt.Sprintf("ssm (%s, %s)", t.InstanceID, t.Region)
			} else {
				conn = fmt.Sprintf("ssh %s@%s:%d", t.User, t.Host, t.Port)
			}

			output.Print("  %s%s", t.Name, isDefault)
			output.Print("    %s", conn)
			if len(t.Tags) > 0 {
				var tags []string
				for k, v := range t.Tags {
					tags = append(tags, fmt.Sprintf("%s=%s", k, v))
				}
				output.Print("    tags: %s", strings.Join(tags, ", "))
			}
			output.NewLine()
		}
		return nil
	},
}

var targetRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a saved target",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := target.Load("")
		if err != nil {
			return err
		}

		name := args[0]
		if err := store.Remove(name); err != nil {
			return err
		}

		output.Success("Removed target: %s", name)
		return nil
	},
}

var targetUseCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Set the default target",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := target.Load("")
		if err != nil {
			return err
		}

		name := args[0]
		if err := store.SetDefault(name); err != nil {
			return err
		}

		output.Success("Default target set to: %s", name)
		return nil
	},
}

func runTargetAdd(cmd *cobra.Command, args []string) error {
	output.Title("📍 Add Server Target")
	output.NewLine()

	store, err := target.Load("")
	if err != nil {
		return err
	}

	// Check if AWS is available
	hasAWS := discovery.HasAWSCredentials()

	methods := []string{"Enter SSH details manually"}
	if hasAWS {
		methods = append([]string{"Pick from AWS (scans EC2 instances)"}, methods...)
	}

	_, method, err := output.Select("Connection source", methods)
	if err != nil {
		return err
	}

	var newTarget *target.Target

	if strings.Contains(method, "AWS") {
		newTarget, err = addFromAWS(store)
	} else {
		newTarget, err = addManually()
	}

	if err != nil {
		return err
	}

	// Save
	if err := store.Add(newTarget); err != nil {
		return err
	}

	output.NewLine()
	output.Success("Saved target: %s", newTarget.Name)
	if store.Default == newTarget.Name {
		output.Info("Set as default target")
	}
	output.Info("Use it: pushsite deploy --target %s", newTarget.Name)
	return nil
}

// addFromAWS walks through profile → region → instance selection
func addFromAWS(store *target.Store) (*target.Target, error) {
	ctx := context.Background()

	// 1. Pick AWS profile
	profiles := discovery.ListProfiles()
	profileIdx := 0
	if len(profiles) > 1 {
		var profileName string
		profileIdx, profileName, _ = output.Select("AWS Profile", profiles)
		_ = profileName
	}
	profile := profiles[profileIdx]
	if profile == "default" {
		profile = ""
	}

	// 2. Pick region
	regions := discovery.CommonRegions()
	_, region, err := output.Select("AWS Region", regions)
	if err != nil {
		return nil, err
	}

	// 3. Scan instances
	output.Info("Scanning EC2 instances in %s...", region)

	disc, err := discovery.New(profile, region)
	if err != nil {
		return nil, fmt.Errorf("AWS discovery failed: %w", err)
	}

	instances, err := disc.ListAndCheck(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list instances: %w", err)
	}

	if len(instances) == 0 {
		output.Warn("No running EC2 instances found in %s", region)
		return addManually()
	}

	// 4. Display and pick instance
	labels := make([]string, len(instances))
	for i, inst := range instances {
		labels[i] = inst.DisplayName()
	}

	selectedIdx, _, err := output.Select("Select instance", labels)
	if err != nil {
		return nil, err
	}
	inst := instances[selectedIdx]

	// 5. Determine method
	method := "ssh"
	if inst.SSMManaged {
		methods := []string{"SSM (recommended — no SSH key needed)", "SSH"}
		idx, _, err := output.Select("Connection method", methods)
		if err != nil {
			return nil, err
		}
		if idx == 0 {
			method = "ssm"
		}
	}

	// 6. User
	suggestedUser := inst.SuggestUser()
	user, err := output.Prompt("SSH user", suggestedUser)
	if err != nil {
		return nil, err
	}

	// 7. Name the target
	defaultName := inst.Name
	if defaultName == "" {
		defaultName = inst.InstanceID
	}
	// Sanitize: lowercase, replace spaces
	defaultName = strings.ToLower(strings.ReplaceAll(defaultName, " ", "-"))

	name, err := output.Prompt("Target name", defaultName)
	if err != nil {
		return nil, err
	}

	t := &target.Target{
		Name:       name,
		Method:     method,
		InstanceID: inst.InstanceID,
		Region:     region,
		PublicIP:   inst.PublicIP,
		User:       user,
		Tags:       inst.Tags,
	}

	// For SSH, also set host and key
	if method == "ssh" {
		t.Host = inst.PublicIP
		if t.Host == "" {
			t.Host, err = output.PromptRequired("Server host (no public IP detected)")
			if err != nil {
				return nil, err
			}
		}
		t.Key, err = output.Prompt("SSH key path", "~/.ssh/id_rsa")
		if err != nil {
			return nil, err
		}
		t.Port = 22
	}

	return t, nil
}

// addManually prompts for all fields
func addManually() (*target.Target, error) {
	name, err := output.PromptRequired("Target name (e.g., prod, staging)")
	if err != nil {
		return nil, err
	}

	_, method, err := output.Select("Connection method", []string{"ssh", "ssm"})
	if err != nil {
		return nil, err
	}

	t := &target.Target{
		Name:   name,
		Method: method,
	}

	if method == "ssh" {
		t.Host, err = output.PromptRequired("Server host (IP or hostname)")
		if err != nil {
			return nil, err
		}
		t.User, err = output.Prompt("SSH user", "ubuntu")
		if err != nil {
			return nil, err
		}
		t.Key, err = output.Prompt("SSH key path", "~/.ssh/id_rsa")
		if err != nil {
			return nil, err
		}
		t.Port = 22
	} else {
		t.InstanceID, err = output.PromptRequired("EC2 Instance ID (i-xxxxxxxx)")
		if err != nil {
			return nil, err
		}
		t.Region, err = output.Prompt("AWS Region", "us-east-1")
		if err != nil {
			return nil, err
		}
		t.User, err = output.Prompt("SSH user", "ubuntu")
		if err != nil {
			return nil, err
		}
	}

	return t, nil
}

func init() {
	targetCmd.AddCommand(targetAddCmd, targetListCmd, targetRemoveCmd, targetUseCmd)
	rootCmd.AddCommand(targetCmd)
}
