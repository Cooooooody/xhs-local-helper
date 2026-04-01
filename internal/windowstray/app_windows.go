//go:build windows

package windowstray

import (
	"fmt"
	"os/exec"
	"syscall"
	"unsafe"
)

const (
	wmApp           = 0x8000
	trayCallbackMsg = wmApp + 1
	wmCommand       = 0x0111
	wmDestroy       = 0x0002
	wmClose         = 0x0010
	wmRButtonUp     = 0x0205
	wmLButtonUp     = 0x0202
	csHRedraw       = 0x0002
	csVRedraw       = 0x0001
	wsOverlapped    = 0x00000000
	cwUseDefault    = 0x80000000
	nimAdd          = 0x00000000
	nimDelete       = 0x00000002
	nifMessage      = 0x00000001
	nifIcon         = 0x00000002
	nifTip          = 0x00000004
	tpmLeftAlign    = 0x0000
	tpmBottomAlign  = 0x0020
	tpmRightButton  = 0x0002
	mfString        = 0x0000
	imageIcon       = 1
	idiApplication  = 32512
	mbIconError     = 0x00000010
	lrDefaultSize   = 0x00000040
	lrLoadFromFile  = 0x00000010

	errorAlreadyExists = 183
	menuBrandOpen      = 1001
	menuOpen           = 1002
	menuClear          = 1003
	menuQuit           = 1004
)

type point struct {
	X int32
	Y int32
}

type msg struct {
	Hwnd     syscall.Handle
	Message  uint32
	WParam   uintptr
	LParam   uintptr
	Time     uint32
	Pt       point
	LPrivate uint32
}

type wndClassEx struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     syscall.Handle
	HIcon         syscall.Handle
	HCursor       syscall.Handle
	HbrBackground syscall.Handle
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       syscall.Handle
}

type notifyIconData struct {
	CbSize            uint32
	HWnd              syscall.Handle
	UID               uint32
	UFlags            uint32
	UCallbackMessage  uint32
	HIcon             syscall.Handle
	SzTip             [128]uint16
	DwState           uint32
	DwStateMask       uint32
	SzInfo            [256]uint16
	UTimeoutOrVersion uint32
	SzInfoTitle       [64]uint16
	DwInfoFlags       uint32
	GuidItem          [16]byte
	HBalloonIcon      syscall.Handle
}

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	shell32              = syscall.NewLazyDLL("shell32.dll")
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	procRegisterClassEx  = user32.NewProc("RegisterClassExW")
	procCreateWindowEx   = user32.NewProc("CreateWindowExW")
	procDefWindowProc    = user32.NewProc("DefWindowProcW")
	procGetMessage       = user32.NewProc("GetMessageW")
	procTranslateMessage = user32.NewProc("TranslateMessage")
	procDispatchMessage  = user32.NewProc("DispatchMessageW")
	procPostQuitMessage  = user32.NewProc("PostQuitMessage")
	procLoadImage        = user32.NewProc("LoadImageW")
	procLoadIcon         = user32.NewProc("LoadIconW")
	procCreatePopupMenu  = user32.NewProc("CreatePopupMenu")
	procAppendMenu       = user32.NewProc("AppendMenuW")
	procGetCursorPos     = user32.NewProc("GetCursorPos")
	procSetForegroundWnd = user32.NewProc("SetForegroundWindow")
	procTrackPopupMenu   = user32.NewProc("TrackPopupMenu")
	procDestroyMenu      = user32.NewProc("DestroyMenu")
	procMessageBox       = user32.NewProc("MessageBoxW")
	procShellNotifyIcon  = shell32.NewProc("Shell_NotifyIconW")
	procCreateMutex      = kernel32.NewProc("CreateMutexW")
	procGetModuleHandle  = kernel32.NewProc("GetModuleHandleW")
	procCloseHandle      = kernel32.NewProc("CloseHandle")
)

type App struct {
	Controller Controller
	OpenURL    func() error
	AppTitle   string
	IconPath   string
	mutex      syscall.Handle
}

var (
	globalApp *App
)

func (a *App) Run() error {
	if a.AppTitle == "" {
		a.AppTitle = "XHS Local Helper"
	}
	if a.OpenURL == nil {
		a.OpenURL = func() error {
			return exec.Command("rundll32", "url.dll,FileProtocolHandler", PublishPageURL).Start()
		}
	}

	globalApp = a

	alreadyRunning, err := a.acquireSingleInstance()
	if err != nil {
		return err
	}
	if alreadyRunning {
		_, err := a.Controller.EnsureHelperStarted()
		return err
	}
	defer a.releaseSingleInstance()

	if _, err := a.Controller.EnsureHelperStarted(); err != nil {
		showError(a.AppTitle, err.Error())
	}

	return runWindowsTrayLoop(a.AppTitle, a.IconPath)
}

func (a *App) acquireSingleInstance() (bool, error) {
	name, err := syscall.UTF16PtrFromString("Local\\XhsLocalHelperWindowsTray")
	if err != nil {
		return false, err
	}
	handle, _, callErr := procCreateMutex.Call(0, 0, uintptr(unsafe.Pointer(name)))
	if handle == 0 {
		if callErr != syscall.Errno(0) {
			return false, callErr
		}
		return false, fmt.Errorf("create mutex failed")
	}

	a.mutex = syscall.Handle(handle)
	if syscall.GetLastError() == syscall.Errno(errorAlreadyExists) {
		_, _, _ = procCloseHandle.Call(handle)
		a.mutex = 0
		return true, nil
	}
	return false, nil
}

func (a *App) releaseSingleInstance() {
	if a.mutex == 0 {
		return
	}
	_, _, _ = procCloseHandle.Call(uintptr(a.mutex))
	a.mutex = 0
}

func runWindowsTrayLoop(title, iconPath string) error {
	hInstanceRaw, _, callErr := procGetModuleHandle.Call(0)
	if hInstanceRaw == 0 {
		if callErr != syscall.Errno(0) {
			return callErr
		}
		return fmt.Errorf("GetModuleHandleW failed")
	}
	hInstance := syscall.Handle(hInstanceRaw)

	className, err := syscall.UTF16PtrFromString("XhsLocalHelperTrayWindow")
	if err != nil {
		return err
	}
	titlePtr, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return err
	}

	icon := loadTrayIcon(iconPath)

	wc := wndClassEx{
		CbSize:        uint32(unsafe.Sizeof(wndClassEx{})),
		Style:         csHRedraw | csVRedraw,
		LpfnWndProc:   syscall.NewCallback(windowProc),
		HInstance:     hInstance,
		HIcon:         icon,
		HIconSm:       icon,
		LpszClassName: className,
	}
	if r, _, callErr := procRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		if callErr != syscall.Errno(0) {
			return callErr
		}
		return fmt.Errorf("RegisterClassExW failed")
	}

	hwndRaw, _, callErr := procCreateWindowEx.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(titlePtr)),
		wsOverlapped,
		cwUseDefault, cwUseDefault, cwUseDefault, cwUseDefault,
		0, 0, uintptr(hInstance), 0,
	)
	if hwndRaw == 0 {
		if callErr != syscall.Errno(0) {
			return callErr
		}
		return fmt.Errorf("CreateWindowExW failed")
	}
	hwnd := syscall.Handle(hwndRaw)

	nid := notifyIconData{
		CbSize:           uint32(unsafe.Sizeof(notifyIconData{})),
		HWnd:             hwnd,
		UID:              1,
		UFlags:           nifMessage | nifIcon | nifTip,
		UCallbackMessage: trayCallbackMsg,
		HIcon:            icon,
	}
	copy(nid.SzTip[:], syscall.StringToUTF16(title))
	if r, _, callErr := procShellNotifyIcon.Call(nimAdd, uintptr(unsafe.Pointer(&nid))); r == 0 {
		if callErr != syscall.Errno(0) {
			return callErr
		}
		return fmt.Errorf("Shell_NotifyIconW add failed")
	}
	defer procShellNotifyIcon.Call(nimDelete, uintptr(unsafe.Pointer(&nid)))

	var m msg
	for {
		ret, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		switch int32(ret) {
		case -1:
			return fmt.Errorf("GetMessageW failed")
		case 0:
			return nil
		default:
			procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
			procDispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
		}
	}
}

func loadTrayIcon(iconPath string) syscall.Handle {
	if iconPath != "" {
		iconPtr, err := syscall.UTF16PtrFromString(iconPath)
		if err == nil {
			iconRaw, _, _ := procLoadImage.Call(
				0,
				uintptr(unsafe.Pointer(iconPtr)),
				imageIcon,
				0,
				0,
				lrLoadFromFile|lrDefaultSize,
			)
			if iconRaw != 0 {
				return syscall.Handle(iconRaw)
			}
		}
	}

	iconRaw, _, _ := procLoadIcon.Call(0, uintptr(idiApplication))
	return syscall.Handle(iconRaw)
}

func windowProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case trayCallbackMsg:
		if lParam == wmRButtonUp || lParam == wmLButtonUp {
			showContextMenu(syscall.Handle(hwnd))
			return 0
		}
	case wmCommand:
		if globalApp == nil {
			return 0
		}
		switch uint32(wParam & 0xffff) {
		case menuBrandOpen, menuOpen:
			if err := globalApp.OpenURL(); err != nil {
				showError(globalApp.AppTitle, err.Error())
			}
			return 0
		case menuClear:
			if err := globalApp.Controller.ClearAccounts(); err != nil {
				showError(globalApp.AppTitle, err.Error())
			}
			return 0
		case menuQuit:
			if err := globalApp.Controller.Exit(); err != nil {
				showError(globalApp.AppTitle, err.Error())
			}
			procPostQuitMessage.Call(0)
			return 0
		}
	case wmClose, wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}

	r, _, _ := procDefWindowProc.Call(hwnd, uintptr(msg), wParam, lParam)
	return r
}

func showContextMenu(hwnd syscall.Handle) {
	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	defer procDestroyMenu.Call(menu)

	brandText, _ := syscall.UTF16PtrFromString("chiccify小红书发布小助手")
	openText, _ := syscall.UTF16PtrFromString("打开网页")
	clearText, _ := syscall.UTF16PtrFromString("清空所有账号")
	quitText, _ := syscall.UTF16PtrFromString("退出小助手")
	procAppendMenu.Call(menu, mfString, menuBrandOpen, uintptr(unsafe.Pointer(brandText)))
	procAppendMenu.Call(menu, mfString, menuOpen, uintptr(unsafe.Pointer(openText)))
	procAppendMenu.Call(menu, mfString, menuClear, uintptr(unsafe.Pointer(clearText)))
	procAppendMenu.Call(menu, mfString, menuQuit, uintptr(unsafe.Pointer(quitText)))

	var pt point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	procSetForegroundWnd.Call(uintptr(hwnd))
	procTrackPopupMenu.Call(menu, tpmLeftAlign|tpmBottomAlign|tpmRightButton, uintptr(pt.X), uintptr(pt.Y), 0, uintptr(hwnd), 0)
}

func showError(title, message string) {
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	messagePtr, _ := syscall.UTF16PtrFromString(message)
	procMessageBox.Call(0, uintptr(unsafe.Pointer(messagePtr)), uintptr(unsafe.Pointer(titlePtr)), mbIconError)
}
