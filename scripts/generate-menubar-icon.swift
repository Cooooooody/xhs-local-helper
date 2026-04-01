import AppKit
import Foundation

let root = URL(fileURLWithPath: FileManager.default.currentDirectoryPath)
let outURL = root.appendingPathComponent("macos/MenuBarApp/menubar-icon.png")
try? FileManager.default.createDirectory(
    at: outURL.deletingLastPathComponent(),
    withIntermediateDirectories: true
)

let canvasSize = NSSize(width: 44, height: 44)
let image = NSImage(size: canvasSize)
image.lockFocus()

NSColor.clear.setFill()
NSBezierPath(rect: NSRect(origin: .zero, size: canvasSize)).fill()

let fontCandidates = [
    "Helvetica-Bold",
    "Arial-BoldMT",
]

let font: NSFont = fontCandidates.compactMap { name in
    NSFont(name: name, size: 34)
}.first ?? NSFont.systemFont(ofSize: 34, weight: .bold)

let style = NSMutableParagraphStyle()
style.alignment = .left

let attrs: [NSAttributedString.Key: Any] = [
    .font: font,
    .foregroundColor: NSColor.black,
    .paragraphStyle: style,
]

let text = NSString(string: "C")
text.draw(
    in: NSRect(x: 8, y: 3, width: canvasSize.width - 12, height: canvasSize.height - 6),
    withAttributes: attrs
)

image.unlockFocus()

guard
    let tiff = image.tiffRepresentation,
    let rep = NSBitmapImageRep(data: tiff),
    let pngData = rep.representation(using: .png, properties: [:])
else {
    fputs("failed to render C menubar icon\n", stderr)
    exit(1)
}

try pngData.write(to: outURL)
