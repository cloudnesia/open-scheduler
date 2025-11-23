package rest

import (
	"fmt"
	"regexp"
	"strings"
)

// ValidJobRequest validates a job submission request
func ValidateJobRequest(req *SubmitJobRequest) error {
	// Validate job name
	if req.JobName == "" {
		return fmt.Errorf("job_name is required")
	}
	if len(req.JobName) > 255 {
		return fmt.Errorf("job_name must be 255 characters or less")
	}
	if !isValidJobName(req.JobName) {
		return fmt.Errorf("job_name contains invalid characters (use alphanumeric, dash, underscore)")
	}

	// Validate job type
	if req.JobType != "" {
		if !isValidJobType(req.JobType) {
			return fmt.Errorf("invalid job_type: %s (must be 'single', 'service', or 'batch')", req.JobType)
		}
	}

	// Validate driver type
	if req.Driver != "" {
		if !isValidDriverType(req.Driver) {
			return fmt.Errorf("invalid driver: %s (must be 'podman', 'incus', or 'process')", req.Driver)
		}
	}

	// Validate workload type
	if req.WorkloadType != "" {
		if !isValidWorkloadType(req.WorkloadType) {
			return fmt.Errorf("invalid workload_type: %s (must be 'container', 'vm', or 'native')", req.WorkloadType)
		}
	}

	// Validate instance config if provided
	if req.InstanceConfig != nil {
		if err := ValidateInstanceSpec(req.InstanceConfig); err != nil {
			return fmt.Errorf("instance_config: %w", err)
		}
	}

	// Validate resources if provided
	if req.Resources != nil {
		if err := ValidateResources(req.Resources); err != nil {
			return fmt.Errorf("resources: %w", err)
		}
	}

	// Validate volumes if provided
	if len(req.Volumes) > 0 {
		for i, vol := range req.Volumes {
			if err := ValidateVolume(vol, i); err != nil {
				return fmt.Errorf("volumes[%d]: %w", i, err)
			}
		}
	}

	// Validate metadata
	if len(req.Meta) > 100 {
		return fmt.Errorf("metadata contains too many fields (max 100)")
	}
	for k, v := range req.Meta {
		if len(k) > 255 {
			return fmt.Errorf("metadata key too long: %s", k)
		}
		if len(v) > 2048 {
			return fmt.Errorf("metadata value too long for key: %s", k)
		}
	}

	return nil
}

// ValidateInstanceSpec validates instance configuration
func ValidateInstanceSpec(spec *InstanceSpecRequest) error {
	if spec == nil {
		return nil
	}

	// Image name is required if instance config is provided
	if spec.Image == "" {
		return fmt.Errorf("image is required when instance_config is provided")
	}

	if len(spec.Image) > 512 {
		return fmt.Errorf("image name is too long (max 512 characters)")
	}

	// Validate image format (basic check)
	if !isValidImageName(spec.Image) {
		return fmt.Errorf("invalid image name format: %s", spec.Image)
	}

	// Validate command if provided
	if len(spec.Command) > 100 {
		return fmt.Errorf("command has too many parts (max 100)")
	}
	for i, cmd := range spec.Command {
		if len(cmd) > 2048 {
			return fmt.Errorf("command[%d] is too long (max 2048 chars)", i)
		}
	}

	// Validate args if provided
	if len(spec.Args) > 100 {
		return fmt.Errorf("arguments have too many elements (max 100)")
	}
	for i, arg := range spec.Args {
		if len(arg) > 2048 {
			return fmt.Errorf("args[%d] is too long (max 2048 chars)", i)
		}
	}

	// Validate driver options
	if len(spec.Options) > 50 {
		return fmt.Errorf("driver options contain too many fields (max 50)")
	}
	for k, v := range spec.Options {
		if len(k) > 128 {
			return fmt.Errorf("driver option key too long: %s", k)
		}
		if len(v) > 4096 {
			return fmt.Errorf("driver option value too long for key: %s", k)
		}
	}

	return nil
}

// ValidateResources validates resource requirements
func ValidateResources(res *ResourcesRequest) error {
	if res == nil {
		return nil
	}

	// Memory validation
	if res.MemoryMB < 0 {
		return fmt.Errorf("memory_mb cannot be negative")
	}
	if res.MemoryMB > 1048576 { // 1TB
		return fmt.Errorf("memory_mb is too large (max 1048576 MB)")
	}
	if res.MemoryReserveMB < 0 {
		return fmt.Errorf("memory_reserve_mb cannot be negative")
	}
	if res.MemoryReserveMB > res.MemoryMB {
		return fmt.Errorf("memory_reserve_mb cannot exceed memory_mb")
	}

	// CPU validation
	if res.CPU < 0 {
		return fmt.Errorf("cpu cannot be negative")
	}
	if res.CPU > 256 { // 256 cores max
		return fmt.Errorf("cpu is too large (max 256 cores)")
	}
	if res.CPUReserve < 0 {
		return fmt.Errorf("cpu_reserve cannot be negative")
	}
	if res.CPUReserve > res.CPU {
		return fmt.Errorf("cpu_reserve cannot exceed cpu")
	}

	return nil
}

// ValidateVolume validates a volume mount
func ValidateVolume(vol VolumeRequest, index int) error {
	if vol.HostPath == "" {
		return fmt.Errorf("host_path is required")
	}
	if vol.InstancePath == "" {
		return fmt.Errorf("instance_path is required")
	}

	// Validate paths don't contain suspicious patterns
	if !isValidVolumePath(vol.HostPath) {
		return fmt.Errorf("host_path contains invalid characters or suspicious patterns")
	}
	if !isValidVolumePath(vol.InstancePath) {
		return fmt.Errorf("instance_path contains invalid characters or suspicious patterns")
	}

	// Prevent path traversal
	if strings.Contains(vol.HostPath, "..") {
		return fmt.Errorf("host_path contains '..' (path traversal not allowed)")
	}
	if strings.Contains(vol.InstancePath, "..") {
		return fmt.Errorf("instance_path contains '..' (path traversal not allowed)")
	}

	return nil
}

// === Helper validation functions ===

// isValidJobName checks if job name contains only valid characters
func isValidJobName(name string) bool {
	// Allow alphanumeric, dash, underscore
	validPattern := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	return validPattern.MatchString(name)
}

// isValidJobType checks if job type is one of valid values
func isValidJobType(jt string) bool {
	valid := map[string]bool{
		"single":  true,
		"service": true,
		"batch":   true,
	}
	return valid[jt]
}

// isValidDriverType checks if driver type is supported
func isValidDriverType(driver string) bool {
	valid := map[string]bool{
		"podman":  true,
		"incus":   true,
		"process": true,
	}
	return valid[driver]
}

// isValidWorkloadType checks if workload type is valid
func isValidWorkloadType(wt string) bool {
	valid := map[string]bool{
		"container": true,
		"vm":        true,
		"native":    true,
	}
	return valid[wt]
}

// isValidImageName validates container image name format
// Allows: registry/name:tag, name:tag, registry/name, name
func isValidImageName(image string) bool {
	// Very permissive - just check length and basic format
	// Real validation should be more strict but this is for MVP
	if image == "" || len(image) > 512 {
		return false
	}
	// Disallow some suspicious patterns
	if strings.Contains(image, "&&") || strings.Contains(image, ";") ||
		strings.Contains(image, "|") || strings.Contains(image, "`") {
		return false
	}
	return true
}

// isValidVolumePath validates volume mount paths
func isValidVolumePath(path string) bool {
	if path == "" {
		return false
	}
	// Basic path validation - must start with / or relative path
	if !strings.HasPrefix(path, "/") && !isValidRelativePath(path) {
		return false
	}
	// Disallow suspicious characters
	if strings.Contains(path, "&&") || strings.Contains(path, ";") ||
		strings.Contains(path, "|") || strings.Contains(path, "`") ||
		strings.Contains(path, "$") {
		return false
	}
	return true
}

// isValidRelativePath checks if path is a valid relative path
func isValidRelativePath(path string) bool {
	if path == "" || path == "/" {
		return false
	}
	// Relative paths should be simple names or basic relative paths
	validPattern := regexp.MustCompile(`^[a-zA-Z0-9._\-/]+$`)
	return validPattern.MatchString(path)
}
