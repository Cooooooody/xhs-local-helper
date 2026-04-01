package windowsbundle

import (
	"fmt"
	"path/filepath"
	"strings"
)

const (
	HelperBinaryName  = "xhs-local-helper-windows-amd64.exe"
	TrayBinaryName    = "xhs-local-helper-windows-tray-amd64.exe"
	TrayIconFileName  = "tray-icon.ico"
	UpstreamArchive   = "xiaohongshu-mcp-windows-amd64.zip"
	McpBinaryName     = "xiaohongshu-mcp-windows-amd64.exe"
	LoginBinaryName   = "xiaohongshu-login-windows-amd64.exe"
	DefaultListenPort = "19180"
	BundleFolderName  = "xhs-local-helper-windows-x64"
	MSIFileName       = "xhs-local-helper-windows-x64.msi"
)

type Layout struct {
	RepoRoot     string
	WindowsDir   string
	UpstreamDir  string
	DistDir      string
	BundleDir    string
	InstallerDir string
	MSIPath      string
	ArchivePath  string
	HelperBinary string
	TrayBinary   string
	IconPath     string
}

func RepoLayout(repoRoot string) Layout {
	windowsDir := filepath.Join(repoRoot, "windows")
	distDir := filepath.Join(repoRoot, "dist", "windows-x64")
	bundleDir := filepath.Join(distDir, BundleFolderName)
	return Layout{
		RepoRoot:     repoRoot,
		WindowsDir:   windowsDir,
		UpstreamDir:  filepath.Join(windowsDir, "upstream"),
		DistDir:      distDir,
		BundleDir:    bundleDir,
		InstallerDir: filepath.Join(distDir, "installer"),
		MSIPath:      filepath.Join(distDir, MSIFileName),
		ArchivePath:  filepath.Join(bundleDir, UpstreamArchive),
		HelperBinary: filepath.Join(bundleDir, HelperBinaryName),
		TrayBinary:   filepath.Join(bundleDir, TrayBinaryName),
		IconPath:     filepath.Join(bundleDir, TrayIconFileName),
	}
}

func RenderStartHelperBat() string {
	return strings.Join([]string{
		"@echo off",
		"setlocal ENABLEDELAYEDEXPANSION",
		"set SCRIPT_DIR=%~dp0",
		fmt.Sprintf("set TRAY_BIN=%%SCRIPT_DIR%%%s", TrayBinaryName),
		fmt.Sprintf("set ARCHIVE_PATH=%%SCRIPT_DIR%%%s", UpstreamArchive),
		"if not exist \"%TRAY_BIN%\" (",
		"  echo missing tray binary: %TRAY_BIN%",
		"  exit /b 1",
		")",
		"start \"XHS Local Helper\" /B \"%TRAY_BIN%\" --archive \"%ARCHIVE_PATH%\"",
		"echo tray helper started",
	}, "\r\n") + "\r\n"
}

func RenderStopHelperBat() string {
	return strings.Join([]string{
		"@echo off",
		"taskkill /IM xhs-local-helper-windows-tray-amd64.exe /F >nul 2>&1",
		"taskkill /IM xhs-local-helper-windows-amd64.exe /F >nul 2>&1",
		"echo tray and helper stop commands issued",
	}, "\r\n") + "\r\n"
}

func RenderBundleReadme() string {
	return strings.Join([]string{
		"# XHS Local Helper Windows x64",
		"",
		"This bundle is for Windows x64.",
		"",
		"Included files:",
		fmt.Sprintf("- `%s`", TrayBinaryName),
		fmt.Sprintf("- `%s`", HelperBinaryName),
		fmt.Sprintf("- `%s`", TrayIconFileName),
		fmt.Sprintf("- `%s`", UpstreamArchive),
		"- `start-helper.bat`",
		"- `stop-helper.bat`",
		"",
		"Usage:",
		"1. Double-click `start-helper.bat`.",
		"2. The script starts the Windows tray host, which ensures the helper runtime is available.",
		"3. Use `stop-helper.bat` to stop the tray host and helper process.",
		"",
		"Upstream runtime names expected by the helper:",
		fmt.Sprintf("- `%s`", McpBinaryName),
		fmt.Sprintf("- `%s`", LoginBinaryName),
	}, "\n") + "\n"
}

func NormalizeMSIVersion(version string) (string, error) {
	if version == "" {
		return "", fmt.Errorf("msi version is required")
	}
	parts := strings.Split(version, ".")
	if len(parts) == 3 {
		parts = append(parts, "0")
	}
	if len(parts) != 4 {
		return "", fmt.Errorf("msi version %q must have 3 or 4 numeric parts", version)
	}
	for _, part := range parts {
		if part == "" {
			return "", fmt.Errorf("msi version %q contains an empty segment", version)
		}
		for _, r := range part {
			if r < '0' || r > '9' {
				return "", fmt.Errorf("msi version %q must be numeric", version)
			}
		}
	}
	return strings.Join(parts, "."), nil
}

func RenderMSIWixSource(layout Layout, version string) (string, error) {
	normalizedVersion, err := NormalizeMSIVersion(version)
	if err != nil {
		return "", err
	}

	bundleDir := xmlEscape(wixPath(layout.BundleDir))
	return strings.Join([]string{
		`<?xml version="1.0" encoding="UTF-8"?>`,
		`<Wix xmlns="http://wixtoolset.org/schemas/v4/wxs" xmlns:util="http://wixtoolset.org/schemas/v4/wxs/util">`,
		`  <Package Name="XHS Local Helper" Manufacturer="Chiccify" Version="` + normalizedVersion + `" UpgradeCode="6F23434C-189F-4C39-AC3C-8D2B85A34B5E" Scope="perUser">`,
		`    <MediaTemplate EmbedCab="yes" />`,
		`    <MajorUpgrade DowngradeErrorMessage="A newer version of XHS Local Helper is already installed." />`,
		`    <StandardDirectory Id="LocalAppDataFolder">`,
		`      <Directory Id="INSTALLDIR" Name="Chiccify\XHS Local Helper" />`,
		`    </StandardDirectory>`,
		`    <StandardDirectory Id="ProgramMenuFolder">`,
		`      <Directory Id="ProgramMenuApp" Name="Chiccify\XHS Local Helper" />`,
		`    </StandardDirectory>`,
		`    <Feature Id="MainFeature" Title="XHS Local Helper" Level="1">`,
		`      <ComponentGroupRef Id="AppFiles" />`,
		`      <ComponentRef Id="StartMenuShortcutComponent" />`,
		`    </Feature>`,
		`    <ComponentGroup Id="AppFiles" Directory="INSTALLDIR">`,
		renderWixFileComponent("TrayBinaryComponent", bundleDir+`\`+TrayBinaryName, TrayBinaryName),
		renderWixFileComponent("HelperBinaryComponent", bundleDir+`\`+HelperBinaryName, HelperBinaryName),
		renderWixFileComponent("TrayIconComponent", bundleDir+`\`+TrayIconFileName, TrayIconFileName),
		renderWixFileComponent("ArchiveComponent", bundleDir+`\`+UpstreamArchive, UpstreamArchive),
		renderWixFileComponent("StartScriptComponent", bundleDir+`\start-helper.bat`, "start-helper.bat"),
		renderWixFileComponent("StopScriptComponent", bundleDir+`\stop-helper.bat`, "stop-helper.bat"),
		`    </ComponentGroup>`,
		`    <DirectoryRef Id="ProgramMenuApp">`,
		`      <Component Id="StartMenuShortcutComponent" Guid="*">`,
		`        <Shortcut Id="StartMenuShortcut" Name="XHS Local Helper" Target="[INSTALLDIR]` + TrayBinaryName + `" WorkingDirectory="INSTALLDIR" />`,
		`        <RemoveFolder Id="RemoveProgramMenuAppFolder" On="uninstall" />`,
		`        <RegistryValue Root="HKCU" Key="Software\Chiccify\XHSLocalHelper" Name="Installed" Type="integer" Value="1" KeyPath="yes" />`,
		`      </Component>`,
		`    </DirectoryRef>`,
		`    <util:CloseApplication Id="CloseTray" Target="` + TrayBinaryName + `" CloseMessage="yes" RebootPrompt="no" TerminateProcess="1" />`,
		`    <util:CloseApplication Id="CloseHelper" Target="` + HelperBinaryName + `" CloseMessage="yes" RebootPrompt="no" TerminateProcess="1" />`,
		`  </Package>`,
		`</Wix>`,
		"",
	}, "\n"), nil
}

func renderWixFileComponent(componentID, sourcePath, fileName string) string {
	return strings.Join([]string{
		`      <Component Id="` + componentID + `" Guid="*">`,
		`        <File Source="` + sourcePath + `" Name="` + fileName + `" KeyPath="yes" />`,
		`      </Component>`,
	}, "\n")
}

func wixPath(path string) string {
	return strings.ReplaceAll(path, "/", `\`)
}

func xmlEscape(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		`"`, "&quot;",
		"<", "&lt;",
		">", "&gt;",
	)
	return replacer.Replace(value)
}
