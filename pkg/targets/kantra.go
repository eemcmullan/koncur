package targets

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/konveyor/analyzer-lsp/provider"
	"github.com/konveyor/test-harness/pkg/config"
	"github.com/konveyor/test-harness/pkg/util"
)

// KantraTarget implements Target for Kantra
type KantraTarget struct {
	binaryPath    string
	mavenSettings string
}

// NewKantraTarget creates a new Kantra target
func NewKantraTarget(cfg *config.KantraConfig) (*KantraTarget, error) {
	var binaryPath string
	var mavenSettings string

	// Use configured path if provided
	if cfg != nil && cfg.BinaryPath != "" {
		binaryPath = cfg.BinaryPath
	} else {
		// Find kantra binary in PATH
		var err error
		binaryPath, err = exec.LookPath("kantra")
		if err != nil {
			return nil, fmt.Errorf("kantra binary not found in PATH: %w", err)
		}
	}

	// Get maven settings from config
	if cfg != nil {
		mavenSettings = cfg.MavenSettings
	}

	return &KantraTarget{
		binaryPath:    binaryPath,
		mavenSettings: mavenSettings,
	}, nil
}

// Name returns the target name
func (k *KantraTarget) Name() string {
	return "kantra"
}

// Execute runs kantra analyze
func (k *KantraTarget) Execute(ctx context.Context, test *config.TestDefinition) (*ExecutionResult, error) {
	log := util.GetLogger()
	log.Info("Executing Kantra analysis", "test", test.Name)

	// Validate maven settings requirement
	if test.RequireMavenSettings && k.mavenSettings == "" {
		return nil, fmt.Errorf("test requires maven settings but none configured in target config")
	}

	// Get test directory (where test.yaml is located)
	testDir := test.GetTestDir()
	if testDir == "" {
		return nil, fmt.Errorf("test directory not available")
	}

	// Prepare work directory for execution logs/metadata
	workDir, err := PrepareWorkDir(test.GetWorkDir(), test.Name)
	if err != nil {
		return nil, err
	}

	// Handle application input (clone git repo to test-dir/source if needed)
	inputPath, err := k.prepareInput(ctx, test.Analysis.Application, test.Name, testDir)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare input: %w", err)
	}

	// Create output directory with absolute path
	outputDir := filepath.Join(workDir, "output")
	absOutputDir, err := filepath.Abs(outputDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute output path: %w", err)
	}
	if err := os.MkdirAll(absOutputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Build kantra command arguments
	args := k.buildArgs(test.Analysis, inputPath, absOutputDir, k.mavenSettings)

	// Execute kantra
	result, err := ExecuteCommand(ctx, k.binaryPath, args, workDir, test.GetTimeout())
	if err != nil {
		return nil, err
	}

	// Set the output file path (absOutputDir is already absolute)
	result.OutputFile = filepath.Join(absOutputDir, "output.yaml")

	LogResult(log, result)

	return result, nil
}

// buildArgs constructs the kantra analyze command arguments
func (k *KantraTarget) buildArgs(analysis config.AnalysisConfig, inputPath, outputDir, mavenSettings string) []string {
	args := []string{"analyze", "--context-lines", strconv.Itoa(analysis.ContextLines)}

	// Input application (now using the prepared input path)
	args = append(args, "--input", inputPath)

	// Output directory (now passed as parameter, already absolute)
	args = append(args, "--output", outputDir)

	// Label selector (if specified)
	if analysis.LabelSelector != "" {
		args = append(args, "--label-selector", analysis.LabelSelector)
	}

	if analysis.IncidentSelector != "" {
		args = append(args, "--incident-selector", analysis.IncidentSelector)
	}

	// Maven settings (from test-level configuration)
	if mavenSettings != "" {
		args = append(args, "--maven-settings", mavenSettings)
	}

	if len(analysis.Target) > 0 {
		for _, target := range analysis.Target {
			args = append(args, "-t", target)
		}
	}
	if len(analysis.Source) > 0 {
		for _, source := range analysis.Source {
			args = append(args, "-s", source)
		}
	}
	if len(analysis.Rules) > 0 {
		for _, rule := range analysis.Rules {
			args = append(args, "--rules", rule)
		}
	}

	// Analysis mode
	switch analysis.AnalysisMode {
	case provider.SourceOnlyAnalysisMode:
		args = append(args, "--mode", "source-only")
	case provider.FullAnalysisMode:
		// Full is the default, but we can be explicit
		args = append(args, "--mode", "full")
	}

	// Use container mode instead of run-local to avoid dependency issues
	args = append(args, "--run-local=false")

	// Allow overwriting existing output
	args = append(args, "--overwrite")

	return args
}

// prepareInput handles git URLs, local paths, and binary files
// Returns the local path to use as input for kantra
func (k *KantraTarget) prepareInput(ctx context.Context, application, testName, workDir string) (string, error) {
	log := util.GetLogger()

	// Check if it's a binary file (.jar, .war, .ear)
	if IsBinaryFile(application) {
		log.Info("Detected binary input", "file", application)
		return k.prepareBinary(application, workDir)
	}

	// Check if it's a git URL (starts with http://, https://, or git@)
	// or contains a git reference (has #branch)
	isGitURL := strings.HasPrefix(application, "http://") ||
		strings.HasPrefix(application, "https://") ||
		strings.HasPrefix(application, "git@")

	if !isGitURL {
		// It's a local path or binary reference
		// Handle binary: prefix (legacy support)
		if strings.HasPrefix(application, "binary:") {
			// Extract the binary file name
			binaryFile := application[7:] // Remove "binary:" prefix
			// For now, just return the binary file as-is
			// In the future, we might need to look for it in a specific directory
			return binaryFile, nil
		}
		// Return as-is for local paths
		return application, nil
	}

	// Parse git URL, reference, and path
	// Format: git_url#branch/path/to/subdir
	var gitURL, gitRef, gitPath string
	if strings.Contains(application, "#") {
		parts := strings.SplitN(application, "#", 2)
		gitURL = parts[0]
		if len(parts) > 1 {
			// Split the reference on "/" to separate branch from path
			refParts := strings.SplitN(parts[1], "/", 2)
			gitRef = refParts[0]
			if len(refParts) > 1 {
				gitPath = refParts[1]
			}
		}
	} else {
		gitURL = application
	}

	// Clone the git repository into workDir/source folder
	cloneDir := filepath.Join(workDir, "source")

	// Get absolute path for clone directory
	absCloneDir, err := filepath.Abs(cloneDir)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Determine the final input directory (may be a subdirectory if path is specified)
	var absInputDir string
	if gitPath != "" {
		absInputDir = filepath.Join(absCloneDir, gitPath)
	} else {
		absInputDir = absCloneDir
	}

	// Check if directory already exists
	if _, err := os.Stat(absInputDir); err == nil {
		log.Info("Repository already exists, skipping clone", "dest", absInputDir)
		return absInputDir, nil
	}

	log.Info("Cloning git repository", "url", gitURL, "ref", gitRef, "path", gitPath, "dest", absCloneDir)

	// Build git clone command
	var gitArgs []string
	if gitRef != "" {
		gitArgs = []string{"clone", "--depth", "1", "--branch", gitRef, gitURL, absCloneDir}
	} else {
		gitArgs = []string{"clone", "--depth", "1", gitURL, absCloneDir}
	}

	// Execute git clone
	result, err := ExecuteCommand(ctx, "git", gitArgs, ".", 5*60*1000000000) // 5 minute timeout for clone
	if err != nil {
		log.Info("Git clone failed", "error", err.Error(), "exitCode", result.ExitCode, "stderr", result.Stderr)
		return "", fmt.Errorf("git clone failed: %w", err)
	}

	log.Info("Git clone completed successfully")

	// Remove .git directory to save space and avoid git-related issues
	gitDir := filepath.Join(absCloneDir, ".git")
	if err := os.RemoveAll(gitDir); err != nil {
		log.Info("Warning: failed to remove .git directory", "error", err.Error())
		// Don't fail the entire operation if we can't remove .git
	} else {
		log.Info("Removed .git directory", "path", gitDir)
	}

	// Verify the target path exists if specified
	if gitPath != "" {
		if _, err := os.Stat(absInputDir); err != nil {
			return "", fmt.Errorf("specified path does not exist in repository: %s: %w", gitPath, err)
		}
		log.Info("Using subdirectory from repository", "path", gitPath, "fullPath", absInputDir)
	}

	return absInputDir, nil
}

// prepareBinary validates and resolves the path to a binary file (.jar, .war, .ear)
// Returns the absolute path to the binary file
func (k *KantraTarget) prepareBinary(binaryPath, testDir string) (string, error) {
	log := util.GetLogger()

	// Check if path is absolute
	if filepath.IsAbs(binaryPath) {
		if _, err := os.Stat(binaryPath); err != nil {
			return "", fmt.Errorf("binary file not found: %w", err)
		}
		log.Info("Using absolute binary path", "path", binaryPath)
		return binaryPath, nil
	}

	// Relative path - resolve relative to test directory
	absPath := filepath.Join(testDir, binaryPath)

	if _, err := os.Stat(absPath); err != nil {
		return "", fmt.Errorf("binary file not found at %s: %w", absPath, err)
	}

	log.Info("Resolved relative binary path", "original", binaryPath, "resolved", absPath)
	return absPath, nil
}
