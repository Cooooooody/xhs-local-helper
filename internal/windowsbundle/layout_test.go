package windowsbundle

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRepoLayoutUsesIsolatedWindowsPaths(t *testing.T) {
	root := "/repo"

	layout := RepoLayout(root)

	if layout.UpstreamDir != filepath.Join(root, "windows", "upstream") {
		t.Fatalf("layout.UpstreamDir = %q", layout.UpstreamDir)
	}
	if layout.DistDir != filepath.Join(root, "dist", "windows-x64") {
		t.Fatalf("layout.DistDir = %q", layout.DistDir)
	}
	if layout.BundleDir != filepath.Join(root, "dist", "windows-x64", "xhs-local-helper-windows-x64") {
		t.Fatalf("layout.BundleDir = %q", layout.BundleDir)
	}
	if layout.TrayBinary != filepath.Join(root, "dist", "windows-x64", "xhs-local-helper-windows-x64", "xhs-local-helper-windows-tray-amd64.exe") {
		t.Fatalf("layout.TrayBinary = %q", layout.TrayBinary)
	}
	if layout.IconPath != filepath.Join(root, "dist", "windows-x64", "xhs-local-helper-windows-x64", TrayIconFileName) {
		t.Fatalf("layout.IconPath = %q", layout.IconPath)
	}
	if layout.MSIPath != filepath.Join(root, "dist", "windows-x64", "xhs-local-helper-windows-x64.msi") {
		t.Fatalf("layout.MSIPath = %q", layout.MSIPath)
	}
	if layout.InstallerDir != filepath.Join(root, "dist", "windows-x64", "installer") {
		t.Fatalf("layout.InstallerDir = %q", layout.InstallerDir)
	}
}

func TestRenderStartHelperBatIncludesExpectedWindowsAssets(t *testing.T) {
	script := RenderStartHelperBat()

	for _, want := range []string{
		"xhs-local-helper-windows-tray-amd64.exe",
		"xiaohongshu-mcp-windows-amd64.zip",
		"start \"XHS Local Helper\" /B \"%TRAY_BIN%\"",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("start script missing %q:\n%s", want, script)
		}
	}
	if strings.Contains(script, "curl.exe") {
		t.Fatalf("start script should no longer call install directly:\n%s", script)
	}
}

func TestRenderStopHelperBatTargetsWindowsHelperProcess(t *testing.T) {
	script := RenderStopHelperBat()

	if !strings.Contains(script, "taskkill") {
		t.Fatalf("stop script missing taskkill:\n%s", script)
	}
	if !strings.Contains(script, "xhs-local-helper-windows-amd64.exe") {
		t.Fatalf("stop script missing helper exe name:\n%s", script)
	}
}

func TestRenderBundleReadmeMentionsStartAndStopScripts(t *testing.T) {
	readme := RenderBundleReadme()

	for _, want := range []string{
		"start-helper.bat",
		"stop-helper.bat",
		"xiaohongshu-mcp-windows-amd64.zip",
		"xhs-local-helper-windows-tray-amd64.exe",
	} {
		if !strings.Contains(readme, want) {
			t.Fatalf("bundle readme missing %q:\n%s", want, readme)
		}
	}
}

func TestNormalizeMSIVersion(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "three parts", input: "1.2.3", want: "1.2.3.0"},
		{name: "four parts", input: "2.3.4.5", want: "2.3.4.5"},
		{name: "empty", input: "", wantErr: true},
		{name: "two parts", input: "1.2", wantErr: true},
		{name: "non numeric", input: "1.2.beta", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeMSIVersion(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("NormalizeMSIVersion(%q) error = nil", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeMSIVersion(%q) error = %v", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("NormalizeMSIVersion(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestRenderMSIWixSourceIncludesExpectedPayloadAndActions(t *testing.T) {
	layout := RepoLayout(`C:\repo`)

	source, err := RenderMSIWixSource(layout, "1.2.3")
	if err != nil {
		t.Fatalf("RenderMSIWixSource error = %v", err)
	}

	for _, want := range []string{
		`Version="1.2.3.0"`,
		HelperBinaryName,
		TrayBinaryName,
		TrayIconFileName,
		UpstreamArchive,
		"start-helper.bat",
		"stop-helper.bat",
		`LocalAppDataFolder`,
		`ProgramMenuFolder`,
		`util:CloseApplication`,
		`Target="[INSTALLDIR]` + TrayBinaryName + `"`,
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("wix source missing %q:\n%s", want, source)
		}
	}
}
