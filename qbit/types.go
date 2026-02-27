package qbit

// TorrentInfo represents a torrent from qBittorrent's API.
type TorrentInfo struct {
	Hash           string  `json:"hash"`
	Name           string  `json:"name"`
	Size           int64   `json:"size"`
	Progress       float64 `json:"progress"`
	DLSpeed        int64   `json:"dlspeed"`
	UPSpeed        int64   `json:"upspeed"`
	NumSeeds       int     `json:"num_seeds"`
	NumLeeches     int     `json:"num_leechs"`
	State          string  `json:"state"`
	ETA            int64   `json:"eta"`
	Category       string  `json:"category"`
	Tags           string  `json:"tags"`
	SavePath       string  `json:"save_path"`
	AddedOn        int64   `json:"added_on"`
	CompletionOn   int64   `json:"completion_on"`
	Ratio          float64 `json:"ratio"`
	Downloaded     int64   `json:"downloaded"`
	Uploaded       int64   `json:"uploaded"`
	AmountLeft     int64   `json:"amount_left"`
	ContentPath    string  `json:"content_path"`
	Tracker        string  `json:"tracker"`
	MagnetURI      string  `json:"magnet_uri"`
}

// TransferInfo represents global transfer statistics.
type TransferInfo struct {
	DLInfoSpeed     int64  `json:"dl_info_speed"`
	DLInfoData      int64  `json:"dl_info_data"`
	UPInfoSpeed     int64  `json:"up_info_speed"`
	UPInfoData      int64  `json:"up_info_data"`
	DLRateLimit     int64  `json:"dl_rate_limit"`
	UPRateLimit     int64  `json:"up_rate_limit"`
	DHtNodes        int    `json:"dht_nodes"`
	ConnectionStatus string `json:"connection_status"`
}
