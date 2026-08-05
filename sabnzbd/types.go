package sabnzbd

// SABnzbd returns most numeric values as preformatted strings ("1277.65",
// "1.2 GB", "0:16:44"), so these fields are strings rather than numbers.

// QueueSlot is one job in the download queue.
type QueueSlot struct {
	NzoID      string `json:"nzo_id"`
	Filename   string `json:"filename"`
	Status     string `json:"status"`
	Index      int    `json:"index"`
	Percentage string `json:"percentage"`
	Size       string `json:"size"`
	SizeLeft   string `json:"sizeleft"`
	TimeLeft   string `json:"timeleft"`
	Priority   string `json:"priority"`
	Category   string `json:"cat"`
	Script     string `json:"script"`
	AvgAge     string `json:"avg_age"`
}

// Queue is the mode=queue response.
type Queue struct {
	Version         string      `json:"version"`
	Paused          bool        `json:"paused"`
	Status          string      `json:"status"`
	Speed           string      `json:"speed"`
	KBPerSec        string      `json:"kbpersec"`
	SizeLeft        string      `json:"sizeleft"`
	Size            string      `json:"size"`
	TimeLeft        string      `json:"timeleft"`
	DiskSpace1Norm  string      `json:"diskspace1_norm"`
	DiskSpaceTotal1 string      `json:"diskspacetotal1"`
	SpeedLimit      string      `json:"speedlimit"`
	HaveWarnings    string      `json:"have_warnings"`
	NoOfSlots       int         `json:"noofslots"`
	NoOfSlotsTotal  int         `json:"noofslots_total"`
	Slots           []QueueSlot `json:"slots"`
}

// HistorySlot is one finished or failed job.
type HistorySlot struct {
	NzoID        string `json:"nzo_id"`
	Name         string `json:"name"`
	NzbName      string `json:"nzb_name"`
	Status       string `json:"status"`
	Category     string `json:"category"`
	Size         string `json:"size"`
	Bytes        int64  `json:"bytes"`
	Storage      string `json:"storage"`
	FailMessage  string `json:"fail_message"`
	ActionLine   string `json:"action_line"`
	Completed    int64  `json:"completed"`
	TimeAdded    int64  `json:"time_added"`
	DownloadTime int64  `json:"download_time"`
	PostprocTime int64  `json:"postproc_time"`
}

// History is the mode=history response.
type History struct {
	NoOfSlots int           `json:"noofslots"`
	Slots     []HistorySlot `json:"slots"`
}

// Version is the mode=version response.
type Version struct {
	Version string `json:"version"`
}
