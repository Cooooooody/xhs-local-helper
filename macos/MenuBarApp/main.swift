import AppKit
import Foundation

private let appTitle = "chiccify小红书发布助手"
private let openWebMenuTitle = "chiccify小红书发布小助手"
private let openWebShortcutMenuTitle = "打开网页"
private let clearAccountsMenuTitle = "清空所有账号"
private let quitMenuTitle = "退出小助手"
private let publishPageURL = URL(string: "https://musegate.tech/#/text2img/auto-generation")!
private let startedNotificationBody = "chiccify小红书发布助手已启动，可返回网页继续发布"
private let alreadyRunningNotificationBody = "chiccify小红书发布助手已在运行"

#if arch(x86_64)
private let supportBinaryName = "xhs-local-helper-app-support-darwin-amd64"
#else
private let supportBinaryName = "xhs-local-helper-app-support-darwin-arm64"
#endif

final class MenuBarAppDelegate: NSObject, NSApplicationDelegate {
    private var statusItem: NSStatusItem!
    private let supportBinaryURL: URL
    private var lastEnsureTriggerAt: Date = .distantPast

    override init() {
        guard let resourceURL = Bundle.main.resourceURL else {
            fatalError("Bundle resource path is missing")
        }
        self.supportBinaryURL = resourceURL.appendingPathComponent(supportBinaryName)
        super.init()
    }

    func applicationDidFinishLaunching(_ notification: Notification) {
        configureStatusItem()
        handleEnsureStarted()
    }

    func applicationShouldHandleReopen(_ sender: NSApplication, hasVisibleWindows flag: Bool) -> Bool {
        handleEnsureStarted()
        return false
    }

    func applicationDidBecomeActive(_ notification: Notification) {
        handleEnsureStarted()
    }

    private func configureStatusItem() {
        statusItem = NSStatusBar.system.statusItem(withLength: NSStatusItem.squareLength)
        if let button = statusItem.button {
            button.image = loadTemplateIcon()
            button.imageScaling = .scaleProportionallyUpOrDown
            button.imagePosition = .imageOnly
            button.toolTip = appTitle
            if button.image == nil {
                button.title = "C"
            }
        }

        let menu = NSMenu()
        menu.addItem(makeMenuItem(title: openWebMenuTitle, action: #selector(openWebpage)))
        menu.addItem(makeMenuItem(title: openWebShortcutMenuTitle, action: #selector(openWebpage)))
        menu.addItem(makeMenuItem(title: clearAccountsMenuTitle, action: #selector(clearAccounts)))
        menu.addItem(NSMenuItem.separator())
        menu.addItem(makeMenuItem(title: quitMenuTitle, action: #selector(quitHelper)))
        statusItem.menu = menu
    }

    private func makeMenuItem(title: String, action: Selector) -> NSMenuItem {
        let item = NSMenuItem(title: title, action: action, keyEquivalent: "")
        item.target = self
        return item
    }

    @objc private func openWebpage() {
        NSWorkspace.shared.open(publishPageURL)
    }

    @objc private func clearAccounts() {
        _ = runSupportCommand("clear-accounts", showAlertOnFailure: true)
    }

    @objc private func quitHelper() {
        _ = runSupportCommand("stop-all", showAlertOnFailure: false)
        NSApp.terminate(nil)
    }

    @discardableResult
    private func runSupportCommand(_ command: String, showAlertOnFailure: Bool) -> String? {
        let process = Process()
        process.executableURL = supportBinaryURL
        process.arguments = [command]

        let outputPipe = Pipe()
        let errorPipe = Pipe()
        process.standardOutput = outputPipe
        process.standardError = errorPipe

        do {
            try process.run()
            process.waitUntilExit()
        } catch {
            if showAlertOnFailure {
                showCriticalAlert(error.localizedDescription)
            }
            return nil
        }

        let outputData = outputPipe.fileHandleForReading.readDataToEndOfFile()
        let errorData = errorPipe.fileHandleForReading.readDataToEndOfFile()
        if process.terminationStatus != 0 {
            if showAlertOnFailure {
                let message = String(data: errorData, encoding: .utf8)?.trimmingCharacters(in: .whitespacesAndNewlines)
                showCriticalAlert(message?.isEmpty == false ? message! : "小助手操作失败")
            }
            return nil
        }

        return String(data: outputData, encoding: .utf8)
    }

    private func showCriticalAlert(_ message: String) {
        NSApp.activate(ignoringOtherApps: true)
        let alert = NSAlert()
        alert.alertStyle = .critical
        alert.messageText = appTitle
        alert.informativeText = message
        alert.runModal()
    }

    private func loadTemplateIcon() -> NSImage? {
        guard let iconURL = Bundle.main.resourceURL?.appendingPathComponent("menubar-icon.png"),
              let image = NSImage(contentsOf: iconURL) else {
            return nil
        }
        image.size = NSSize(width: 18, height: 18)
        image.isTemplate = true
        return image
    }

    private func handleEnsureStarted() {
        let now = Date()
        guard now.timeIntervalSince(lastEnsureTriggerAt) > 0.8 else {
            return
        }
        lastEnsureTriggerAt = now
        let result = runSupportCommand("ensure-started", showAlertOnFailure: true)
        switch result?.trimmingCharacters(in: .whitespacesAndNewlines) {
        case "started":
            showOsaScriptNotification(startedNotificationBody)
        case "already_running":
            showOsaScriptNotification(alreadyRunningNotificationBody)
        default:
            break
        }
    }

    private func showOsaScriptNotification(_ message: String) {
        let process = Process()
        process.executableURL = URL(fileURLWithPath: "/usr/bin/osascript")
        process.arguments = ["-e", #"display notification "\#(message)" with title "\#(appTitle)""#]
        do {
            try process.run()
        } catch {
            // Non-fatal; the helper itself is already running.
        }
    }
}

let app = NSApplication.shared
let delegate = MenuBarAppDelegate()
app.delegate = delegate
app.setActivationPolicy(.accessory)
app.run()
