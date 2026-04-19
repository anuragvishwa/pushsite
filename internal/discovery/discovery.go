package discovery

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// InstanceInfo holds discovered EC2 instance metadata
type InstanceInfo struct {
	InstanceID string
	Name       string // from Name tag
	PublicIP   string
	PrivateIP  string
	State      string
	Platform   string // linux, windows
	SSMManaged bool
	Region     string
	Tags       map[string]string
}

// DisplayName returns a human-readable label for the instance
func (i *InstanceInfo) DisplayName() string {
	name := i.Name
	if name == "" {
		name = i.InstanceID
	}
	ip := i.PublicIP
	if ip == "" {
		ip = i.PrivateIP
	}
	ssmLabel := ""
	if i.SSMManaged {
		ssmLabel = " [SSM]"
	}
	return fmt.Sprintf("%s (%s) — %s%s", name, i.InstanceID, ip, ssmLabel)
}

// SuggestUser returns the likely SSH user based on platform/AMI
func (i *InstanceInfo) SuggestUser() string {
	// Check tags for hints
	if user, ok := i.Tags["ssh-user"]; ok {
		return user
	}
	// Common AMI defaults
	platform := strings.ToLower(i.Platform)
	if strings.Contains(platform, "windows") {
		return "Administrator"
	}
	// Most AWS Linux AMIs use ec2-user, Ubuntu uses ubuntu
	for _, tag := range []string{i.Name, i.Tags["Name"]} {
		lower := strings.ToLower(tag)
		if strings.Contains(lower, "ubuntu") {
			return "ubuntu"
		}
		if strings.Contains(lower, "amazon") || strings.Contains(lower, "amzn") {
			return "ec2-user"
		}
		if strings.Contains(lower, "debian") {
			return "admin"
		}
	}
	return "ubuntu" // safe default
}

// Discovery provides AWS EC2 instance discovery
type Discovery struct {
	profile string
	region  string
	cfg     aws.Config
}

// New creates a new Discovery client
func New(profile, region string) (*Discovery, error) {
	ctx := context.Background()

	opts := []func(*awsconfig.LoadOptions) error{}
	if profile != "" {
		opts = append(opts, awsconfig.WithSharedConfigProfile(profile))
	}
	if region != "" {
		opts = append(opts, awsconfig.WithRegion(region))
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &Discovery{
		profile: profile,
		region:  region,
		cfg:     cfg,
	}, nil
}

// ListInstances discovers running EC2 instances
func (d *Discovery) ListInstances(ctx context.Context) ([]InstanceInfo, error) {
	client := ec2.NewFromConfig(d.cfg)

	input := &ec2.DescribeInstancesInput{
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("instance-state-name"),
				Values: []string{"running"},
			},
		},
	}

	result, err := client.DescribeInstances(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("EC2 DescribeInstances failed: %w", err)
	}

	var instances []InstanceInfo
	for _, reservation := range result.Reservations {
		for _, inst := range reservation.Instances {
			info := InstanceInfo{
				InstanceID: deref(inst.InstanceId),
				PublicIP:   deref(inst.PublicIpAddress),
				PrivateIP:  deref(inst.PrivateIpAddress),
				State:      string(inst.State.Name),
				Platform:   deref(inst.PlatformDetails),
				Region:     d.cfg.Region,
				Tags:       make(map[string]string),
			}

			// Extract tags
			for _, tag := range inst.Tags {
				key := deref(tag.Key)
				val := deref(tag.Value)
				info.Tags[key] = val
				if key == "Name" {
					info.Name = val
				}
			}

			instances = append(instances, info)
		}
	}

	// Sort by name
	sort.Slice(instances, func(i, j int) bool {
		return instances[i].Name < instances[j].Name
	})

	return instances, nil
}

// CheckSSM checks which instances are SSM-managed
func (d *Discovery) CheckSSM(ctx context.Context, instances []InstanceInfo) ([]InstanceInfo, error) {
	if len(instances) == 0 {
		return instances, nil
	}

	client := ssm.NewFromConfig(d.cfg)

	input := &ssm.DescribeInstanceInformationInput{}
	result, err := client.DescribeInstanceInformation(ctx, input)
	if err != nil {
		// SSM not available — return instances as-is
		return instances, nil
	}

	// Build set of SSM-managed instance IDs
	ssmSet := make(map[string]bool)
	for _, info := range result.InstanceInformationList {
		if info.InstanceId != nil {
			ssmSet[*info.InstanceId] = true
		}
	}

	// Tag instances
	for i := range instances {
		instances[i].SSMManaged = ssmSet[instances[i].InstanceID]
	}

	return instances, nil
}

// ListAndCheck combines ListInstances + CheckSSM
func (d *Discovery) ListAndCheck(ctx context.Context) ([]InstanceInfo, error) {
	instances, err := d.ListInstances(ctx)
	if err != nil {
		return nil, err
	}

	instances, err = d.CheckSSM(ctx, instances)
	if err != nil {
		return instances, nil // SSM check is best-effort
	}

	return instances, nil
}

// ListProfiles reads AWS profile names from ~/.aws/config and ~/.aws/credentials
func ListProfiles() []string {
	profiles := []string{"default"}
	seen := map[string]bool{"default": true}

	home, err := os.UserHomeDir()
	if err != nil {
		return profiles
	}

	for _, file := range []string{
		filepath.Join(home, ".aws", "config"),
		filepath.Join(home, ".aws", "credentials"),
	} {
		f, err := os.Open(file)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			// [profile foo] or [foo]
			if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
				name := strings.TrimPrefix(line, "[")
				name = strings.TrimSuffix(name, "]")
				name = strings.TrimPrefix(name, "profile ")
				name = strings.TrimSpace(name)
				if name != "" && !seen[name] {
					profiles = append(profiles, name)
					seen[name] = true
				}
			}
		}
		f.Close()
	}

	return profiles
}

// CommonRegions returns frequently used AWS regions
func CommonRegions() []string {
	return []string{
		"us-east-1",
		"us-east-2",
		"us-west-1",
		"us-west-2",
		"eu-west-1",
		"eu-west-2",
		"eu-central-1",
		"ap-south-1",
		"ap-southeast-1",
		"ap-southeast-2",
		"ap-northeast-1",
		"ca-central-1",
		"sa-east-1",
	}
}

// HasAWSCredentials checks if AWS credentials are configured
func HasAWSCredentials() bool {
	// Check environment
	if os.Getenv("AWS_ACCESS_KEY_ID") != "" || os.Getenv("AWS_PROFILE") != "" {
		return true
	}

	// Check ~/.aws/credentials
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(home, ".aws", "credentials"))
	if err == nil {
		return true
	}
	_, err = os.Stat(filepath.Join(home, ".aws", "config"))
	return err == nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
