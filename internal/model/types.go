package model

const (
	CodeOK                 = 200
	CodeBadRequest         = 400
	CodeMethodNotAllowed   = 405
	CodeTitleTooLong       = 410
	CodeContentTooLong     = 411
	CodeImagesEmpty        = 412
	CodeImageSourceInvalid = 414
	CodeImageDownloadFail  = 415
	CodeImagePrepareFail   = 416
	CodeInstallFailed      = 430
	CodeLoginFailed        = 440
	CodeMcpRuntimeMissing  = 450
	CodeMcpUnavailable     = 451
	CodeMcpSessionFailed   = 452
	CodePublishUpstream    = 453
	CodeInternal           = 500
)

type StatusResponse struct {
	Success              bool   `json:"success"`
	Code                 int    `json:"code"`
	Installed            bool   `json:"installed"`
	McpRunning           bool   `json:"mcpRunning"`
	LoggedIn             bool   `json:"loggedIn"`
	AppDir               string `json:"appDir"`
	McpBaseURL           string `json:"mcpBaseUrl"`
	McpBinaryPath        string `json:"mcpBinaryPath"`
	LoginBinaryPath      string `json:"loginBinaryPath"`
	DefaultArchivePath   string `json:"defaultArchivePath"`
	McpPid               int    `json:"mcpPid,omitempty"`
	LoginPid             int    `json:"loginPid,omitempty"`
}

type InstallRequest struct {
	ArchivePath string `json:"archivePath"`
}

type ActionResponse struct {
	Success bool   `json:"success"`
	Code    int    `json:"code"`
	Message string `json:"message"`
	Pid     int    `json:"pid,omitempty"`
}

type PublishRequest struct {
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Tags    []string `json:"tags,omitempty"`
	Images  []string `json:"images"`
}

type PublishResponse struct {
	Success bool   `json:"success"`
	Code    int    `json:"code"`
	Message string `json:"message"`
	NoteID  string `json:"noteId,omitempty"`
	PostID  string `json:"postId,omitempty"`
}

type ErrorResponse struct {
	Success bool   `json:"success"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}
