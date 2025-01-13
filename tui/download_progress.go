package tui

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
)

// DownloadStatus is the top-level structure of the JSON on each line of the docker output
//
// This can be used to define onProgress callbacks when creating a new DownloadProgress TUI
type DownloadStatus struct {
	Status         string         `json:"status"`
	ProgressDetail progressDetail `json:"progressDetail"`
	ID             string         `json:"id"`
}
type progressDetail struct {
	Current *int `json:"current"`
	Total   *int `json:"total"`
}

type dockerProgressWriter struct {
	reader      io.ReadCloser
	layers      int
	downloaded  int
	downloads   map[string]float64
	onProgress  func(int, int, float64, DownloadStatus)
	onError     func(error)
	onJsonParse func(DownloadStatus)
	onFinish    func()
}

type quitMsg struct{}
type progressMsg float64
type progressErrMsg struct{ err error }
type debugMsg string
type Model struct {
	header   string
	progress progress.Model
	writer   *dockerProgressWriter
	Err      error
	debug    string
}

var teaProgram *tea.Program

// Start will copy the reader output to the DockerProgressWriter.
//
// This function is called in a goroutine so we don't block the TUI.
func (pw *dockerProgressWriter) Start() {
	_, err := io.Copy(pw, pw.reader)
	if err != nil {
		pw.onError(err)
	}
}

// NewDownloadProgress returns a bubbletea TUI that tracks the progress of the given reader.
//
// It assumes the reader outputs the docker progress JSON messages.
func NewDownloadProgress(initialHeader string, reader io.ReadCloser) *tea.Program {
	writer := newDockerProgressWriter(
		reader,
		func(err error) { teaProgram.Send(progressErrMsg{err}) },
		func(slice int, totalSlices int, downloadsInFlight float64, details DownloadStatus) {
			teaProgram.Send(progressMsg((float64(slice) + downloadsInFlight) / float64(totalSlices)))
		},
		func(details DownloadStatus) {
			// Uncomment to see each output line below the progres bar
			// encodedDetails, _ := json.Marshal(details)
			// teaProgram.Send(debugMsg(string(encodedDetails)))
		},
		func() {
			teaProgram.Send(quitMsg{})
		},
	)
	teaProgram = tea.NewProgram(
		Model{
			header:   initialHeader,
			progress: progress.New(progress.WithDefaultGradient()),
			writer:   writer,
		},
	)
	go writer.Start()
	return teaProgram
}

func newDockerProgressWriter(
	reader io.ReadCloser,
	onError func(error),
	onProgress func(int, int, float64, DownloadStatus),
	onJsonParse func(DownloadStatus),
	onFinish func(),
) *dockerProgressWriter {
	return &dockerProgressWriter{
		reader:      reader,
		layers:      0,
		downloaded:  0,
		downloads:   map[string]float64{},
		onError:     onError,
		onProgress:  onProgress,
		onJsonParse: onJsonParse,
		onFinish:    onFinish,
	}
}

func (cw *dockerProgressWriter) Write(payload []byte) (n int, jsonError error) {
	lines := strings.Split(string(payload), "\n")
	for _, line := range lines {
		var status DownloadStatus
		progressParseError := json.Unmarshal([]byte(line), &status)
		if progressParseError == nil {
			// useful for debugging
			cw.onJsonParse(status)

			switch status.Status {
			case "Downloading":
				var downloadProgress float64
				if status.ProgressDetail.Current != nil && status.ProgressDetail.Total != nil && *status.ProgressDetail.Total > 0 {
					downloadProgress = float64(*status.ProgressDetail.Current) / float64(*status.ProgressDetail.Total)
				} else {
					downloadProgress = 0
				}
				cw.downloads[status.ID] = downloadProgress

				// Sums all download progress in flight for finer grained progress
				totalDownload := 0.0
				for _, value := range cw.downloads {
					totalDownload += value
				}
				cw.onProgress(cw.downloaded, cw.layers, totalDownload, status)
				break
			case "Pulling fs layer":
				cw.layers++
				break
			case "Download complete":
				delete(cw.downloads, status.ID)
				cw.downloaded++
				break
			case "Already exists":
				cw.layers++
				cw.downloaded++
				break
			default:
				if strings.Contains(status.Status, "Status: Downloaded newer image") {
					cw.onProgress(cw.layers, cw.layers, 0.0, status)
					// we need to sleep to ensure the TUI has time to animate the progress bar
					// specially in cases where we have only one small image to download
					time.Sleep(time.Second)
					cw.onFinish()
				}
			}
		}
	}
	return len(payload), nil
}

func (m Model) Init() tea.Cmd {
	return func() tea.Msg {
		return nil
	}
}

const (
	padding  = 2
	maxWidth = 80
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyEsc {
			return m, func() tea.Msg { return progressErrMsg{errors.New("ESC was pressed")} }
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.progress.Width = msg.Width - padding*2 - 4
		if m.progress.Width > maxWidth {
			m.progress.Width = maxWidth
		}
		return m, nil

	case progressErrMsg:
		m.Err = msg.err
		return m, tea.Quit

	case debugMsg:
		m.debug = string(msg)
		return m, nil

	case quitMsg:
		return m, tea.Quit

	case progressMsg:
		cmd := m.progress.SetPercent(float64(msg))
		return m, cmd

	// FrameMsg is sent when the progress bar wants to animate itself
	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		return m, cmd

	default:
		return m, nil
	}
}

func (m Model) View() string {
	if m.Err != nil {
		return "Error downloading: " + m.Err.Error() + "\n"
	}
	pad := strings.Repeat(" ", padding)

	almostThere := ""
	if m.progress.Percent() == 1.0 {
		almostThere = pad + "Finishing download verification. "
	}
	return "\n" +
		pad + m.header + "\n\n" +
		pad + m.progress.View() + "\n\n" +
		almostThere +
		pad + "Press ESC to abort download.\n\n" +
		pad + m.debug
}
