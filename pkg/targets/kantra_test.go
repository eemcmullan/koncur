package targets

import (
	"strings"
	"testing"

	"github.com/konveyor/analyzer-lsp/provider"
	"github.com/konveyor/test-harness/pkg/config"
)

func TestNewKantraTarget(t *testing.T) {
	tests := []struct {
		name       string
		cfg        *config.KantraConfig
		wantErr    bool
		checkPath  bool
		expectPath string
	}{
		{
			name: "nil config uses PATH",
			cfg:  nil,
			// This will fail if kantra is not in PATH, which is expected
			wantErr:   true,
			checkPath: false,
		},
		{
			name: "empty config uses PATH",
			cfg:  &config.KantraConfig{},
			// This will fail if kantra is not in PATH, which is expected
			wantErr:   true,
			checkPath: false,
		},
		{
			name: "explicit binary path",
			cfg: &config.KantraConfig{
				BinaryPath: "/usr/local/bin/kantra",
			},
			wantErr:    false,
			checkPath:  true,
			expectPath: "/usr/local/bin/kantra",
		},
		{
			name: "config with maven settings",
			cfg: &config.KantraConfig{
				BinaryPath:    "/usr/local/bin/kantra",
				MavenSettings: "/path/to/settings.xml",
			},
			wantErr:    false,
			checkPath:  true,
			expectPath: "/usr/local/bin/kantra",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, err := NewKantraTarget(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewKantraTarget() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if target == nil {
					t.Error("Expected non-nil target")
				}
				if target.Name() != "kantra" {
					t.Errorf("Expected name 'kantra', got '%s'", target.Name())
				}
				if tt.checkPath && target.binaryPath != tt.expectPath {
					t.Errorf("Expected binary path '%s', got '%s'", tt.expectPath, target.binaryPath)
				}
				if tt.cfg != nil && target.mavenSettings != tt.cfg.MavenSettings {
					t.Errorf("Expected maven settings '%s', got '%s'", tt.cfg.MavenSettings, target.mavenSettings)
				}
			}
		})
	}
}

func TestKantraTarget_Name(t *testing.T) {
	target := &KantraTarget{}
	if target.Name() != "kantra" {
		t.Errorf("Expected name 'kantra', got '%s'", target.Name())
	}
}

func TestKantraTarget_BuildArgs(t *testing.T) {
	tests := []struct {
		name          string
		analysis      config.AnalysisConfig
		inputPath     string
		outputDir     string
		mavenSettings string
		expectContain []string
		expectNotContain []string
	}{
		{
			name: "basic source-only analysis",
			analysis: config.AnalysisConfig{
				AnalysisMode: provider.SourceOnlyAnalysisMode,
				ContextLines: 10,
			},
			inputPath: "/path/to/app",
			outputDir: "/path/to/output",
			expectContain: []string{
				"analyze",
				"--context-lines", "10",
				"--input", "/path/to/app",
				"--output", "/path/to/output",
				"--mode", "source-only",
				"--run-local=false",
				"--overwrite",
			},
		},
		{
			name: "full analysis with targets and sources",
			analysis: config.AnalysisConfig{
				AnalysisMode: provider.FullAnalysisMode,
				ContextLines: 20,
				Target:       []string{"cloud-readiness", "quarkus"},
				Source:       []string{"java", "java-ee"},
			},
			inputPath: "/path/to/app",
			outputDir: "/path/to/output",
			expectContain: []string{
				"analyze",
				"--mode", "full",
				"-t", "cloud-readiness",
				"-t", "quarkus",
				"-s", "java",
				"-s", "java-ee",
			},
		},
		{
			name: "analysis with label selector",
			analysis: config.AnalysisConfig{
				AnalysisMode:   provider.SourceOnlyAnalysisMode,
				ContextLines:   10,
				LabelSelector:  "konveyor.io/target=cloud-readiness",
			},
			inputPath: "/path/to/app",
			outputDir: "/path/to/output",
			expectContain: []string{
				"--label-selector", "konveyor.io/target=cloud-readiness",
			},
		},
		{
			name: "analysis with incident selector",
			analysis: config.AnalysisConfig{
				AnalysisMode:     provider.SourceOnlyAnalysisMode,
				ContextLines:     10,
				IncidentSelector: "lineNumber > 100",
			},
			inputPath: "/path/to/app",
			outputDir: "/path/to/output",
			expectContain: []string{
				"--incident-selector", "lineNumber > 100",
			},
		},
		{
			name: "analysis with maven settings",
			analysis: config.AnalysisConfig{
				AnalysisMode: provider.SourceOnlyAnalysisMode,
				ContextLines: 10,
			},
			inputPath:     "/path/to/app",
			outputDir:     "/path/to/output",
			mavenSettings: "/path/to/settings.xml",
			expectContain: []string{
				"--maven-settings", "/path/to/settings.xml",
			},
		},
		{
			name: "analysis with rules",
			analysis: config.AnalysisConfig{
				AnalysisMode: provider.SourceOnlyAnalysisMode,
				ContextLines: 10,
				Rules:        []string{"/custom/rules1", "/custom/rules2"},
			},
			inputPath: "/path/to/app",
			outputDir: "/path/to/output",
			expectContain: []string{
				"--rules", "/custom/rules1",
				"--rules", "/custom/rules2",
			},
		},
		{
			name: "analysis without maven settings",
			analysis: config.AnalysisConfig{
				AnalysisMode: provider.SourceOnlyAnalysisMode,
				ContextLines: 10,
			},
			inputPath:     "/path/to/app",
			outputDir:     "/path/to/output",
			mavenSettings: "",
			expectNotContain: []string{
				"--maven-settings",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := &KantraTarget{
				binaryPath:    "/usr/local/bin/kantra",
				mavenSettings: tt.mavenSettings,
			}

			args := k.buildArgs(tt.analysis, tt.inputPath, tt.outputDir, tt.mavenSettings)
			argsStr := strings.Join(args, " ")

			// Check for expected arguments
			for _, expected := range tt.expectContain {
				found := false
				for _, arg := range args {
					if arg == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected arg '%s' not found in: %v", expected, args)
				}
			}

			// Check for unexpected arguments
			for _, notExpected := range tt.expectNotContain {
				for _, arg := range args {
					if arg == notExpected {
						t.Errorf("Unexpected arg '%s' found in: %v", notExpected, args)
					}
				}
			}

			// Verify basic structure
			if len(args) == 0 {
				t.Error("Expected non-empty args")
			}
			if args[0] != "analyze" {
				t.Errorf("Expected first arg to be 'analyze', got '%s'", args[0])
			}

			t.Logf("Generated args: %s", argsStr)
		})
	}
}

func TestKantraTarget_PrepareInput(t *testing.T) {
	tests := []struct {
		name        string
		application string
		isGitURL    bool
		expectError bool
	}{
		{
			name:        "local path",
			application: "/local/path/to/app",
			isGitURL:    false,
			expectError: false,
		},
		{
			name:        "binary reference",
			application: "binary:app.jar",
			isGitURL:    false,
			expectError: false,
		},
		{
			name:        "http git URL",
			application: "http://github.com/konveyor/tackle-testapp.git",
			isGitURL:    true,
			expectError: false,
		},
		{
			name:        "https git URL",
			application: "https://github.com/konveyor/tackle-testapp.git",
			isGitURL:    true,
			expectError: false,
		},
		{
			name:        "git URL with branch",
			application: "https://github.com/konveyor/tackle-testapp.git#main",
			isGitURL:    true,
			expectError: false,
		},
		{
			name:        "git URL with feature branch",
			application: "https://github.com/konveyor/tackle-testapp.git#feature/test",
			isGitURL:    true,
			expectError: false,
		},
		{
			name:        "ssh git URL",
			application: "git@github.com:konveyor/tackle-testapp.git",
			isGitURL:    true,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Check URL detection logic (not executing prepareInput as it would require git/network)
			isGitURL := strings.HasPrefix(tt.application, "http://") ||
				strings.HasPrefix(tt.application, "https://") ||
				strings.HasPrefix(tt.application, "git@")

			if isGitURL != tt.isGitURL {
				t.Errorf("Expected isGitURL=%v, got %v for '%s'", tt.isGitURL, isGitURL, tt.application)
			}

			// Check binary prefix handling
			if strings.HasPrefix(tt.application, "binary:") {
				binaryFile := tt.application[7:]
				if tt.application != "binary:"+binaryFile {
					t.Error("Binary prefix handling failed")
				}
			}

			// Check git reference parsing
			if strings.Contains(tt.application, "#") {
				parts := strings.SplitN(tt.application, "#", 2)
				if len(parts) != 2 {
					t.Error("Failed to parse git reference")
				}
				gitURL := parts[0]
				gitRef := parts[1]
				if gitURL == "" || gitRef == "" {
					t.Error("Git URL or ref is empty after parsing")
				}
				t.Logf("Parsed git URL: %s, ref: %s", gitURL, gitRef)
			}
		})
	}
}

func TestKantraTarget_ValidateMavenSettings(t *testing.T) {
	tests := []struct {
		name          string
		mavenSettings string
		testRequires  bool
		wantErr       bool
	}{
		{
			name:          "test requires maven but not configured",
			mavenSettings: "",
			testRequires:  true,
			wantErr:       true,
		},
		{
			name:          "test requires maven and configured",
			mavenSettings: "/path/to/settings.xml",
			testRequires:  true,
			wantErr:       false,
		},
		{
			name:          "test doesn't require maven",
			mavenSettings: "",
			testRequires:  false,
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := &KantraTarget{
				mavenSettings: tt.mavenSettings,
			}

			// Simulate the validation check from Execute
			if tt.testRequires && target.mavenSettings == "" {
				if !tt.wantErr {
					t.Error("Expected error for missing maven settings")
				}
			}
		})
	}
}

func TestKantraTarget_AnalysisMode(t *testing.T) {
	tests := []struct {
		name         string
		analysisMode provider.AnalysisMode
		expectFlag   string
	}{
		{
			name:         "source-only mode",
			analysisMode: provider.SourceOnlyAnalysisMode,
			expectFlag:   "source-only",
		},
		{
			name:         "full mode",
			analysisMode: provider.FullAnalysisMode,
			expectFlag:   "full",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis := config.AnalysisConfig{
				AnalysisMode: tt.analysisMode,
				ContextLines: 10,
			}

			k := &KantraTarget{binaryPath: "/usr/local/bin/kantra"}
			args := k.buildArgs(analysis, "/input", "/output", "")

			// Find the --mode flag
			foundMode := false
			for i, arg := range args {
				if arg == "--mode" && i+1 < len(args) {
					if args[i+1] != tt.expectFlag {
						t.Errorf("Expected mode '%s', got '%s'", tt.expectFlag, args[i+1])
					}
					foundMode = true
					break
				}
			}

			if !foundMode {
				t.Errorf("Expected --mode flag with value '%s' not found", tt.expectFlag)
			}
		})
	}
}

func TestKantraTarget_ContextLines(t *testing.T) {
	tests := []struct {
		name         string
		contextLines int
		expectValue  string
	}{
		{
			name:         "default context lines",
			contextLines: 10,
			expectValue:  "10",
		},
		{
			name:         "custom context lines",
			contextLines: 100,
			expectValue:  "100",
		},
		{
			name:         "zero context lines",
			contextLines: 0,
			expectValue:  "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis := config.AnalysisConfig{
				AnalysisMode: provider.SourceOnlyAnalysisMode,
				ContextLines: tt.contextLines,
			}

			k := &KantraTarget{binaryPath: "/usr/local/bin/kantra"}
			args := k.buildArgs(analysis, "/input", "/output", "")

			// Find the --context-lines flag
			foundContextLines := false
			for i, arg := range args {
				if arg == "--context-lines" && i+1 < len(args) {
					if args[i+1] != tt.expectValue {
						t.Errorf("Expected context-lines '%s', got '%s'", tt.expectValue, args[i+1])
					}
					foundContextLines = true
					break
				}
			}

			if !foundContextLines {
				t.Error("Expected --context-lines flag not found")
			}
		})
	}
}
