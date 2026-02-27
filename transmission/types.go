package transmission

// RPC request/response types per the Transmission RPC spec.

type rpcRequest struct {
	Method    string `json:"method"`
	Arguments any    `json:"arguments,omitempty"`
	Tag       int    `json:"tag,omitempty"`
}

type rpcResponse struct {
	Result    string         `json:"result"`
	Arguments map[string]any `json:"arguments,omitempty"`
	Tag       int            `json:"tag,omitempty"`
}

// TorrentInfo is a simplified torrent representation.
type TorrentInfo struct {
	ID            int     `json:"id"`
	Name          string  `json:"name"`
	Status        int     `json:"status"`
	StatusText    string  `json:"status_text"`
	PercentDone   float64 `json:"percent_done"`
	TotalSize     int64   `json:"total_size"`
	DownloadedEver int64  `json:"downloaded_ever"`
	UploadedEver  int64   `json:"uploaded_ever"`
	RateDownload  int64   `json:"rate_download"`
	RateUpload    int64   `json:"rate_upload"`
	ETA           int     `json:"eta"`
	Error         int     `json:"error"`
	ErrorString   string  `json:"error_string"`
	DownloadDir   string  `json:"download_dir"`
}

// Torrent status codes.
const (
	StatusStopped      = 0
	StatusCheckWait    = 1
	StatusChecking     = 2
	StatusDownloadWait = 3
	StatusDownloading  = 4
	StatusSeedWait     = 5
	StatusSeeding      = 6
)

var statusNames = map[int]string{
	StatusStopped:      "stopped",
	StatusCheckWait:    "check_wait",
	StatusChecking:     "checking",
	StatusDownloadWait: "download_wait",
	StatusDownloading:  "downloading",
	StatusSeedWait:     "seed_wait",
	StatusSeeding:      "seeding",
}

func StatusName(code int) string {
	if name, ok := statusNames[code]; ok {
		return name
	}
	return "unknown"
}
