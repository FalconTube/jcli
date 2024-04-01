/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"strings"

	util "jcli/jenkins"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	currentPkgNameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("211"))
	doneStyle           = lipgloss.NewStyle().Margin(1, 1, 0)
	helpStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Margin(0, 1)
	checkMark           = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).SetString("✓")
)

type BuildModel struct {
	BuildUrl      string
	File          string
	width         int
	height        int
	done          bool
	statusMessage string
	userScrolled  bool
	spinner       spinner.Model
	viewport      viewport.Model
}

type consoleOutput string
type consoleFinish string
type emptyUrl string

// var File string
var FullLog string

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update a Jenkins job with a new pipeline script",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		main()
	},
}

func (m *BuildModel) initBuild() tea.Cmd {
	return func() tea.Msg {

		// Check if the file exists
		if _, err := os.Stat(m.File); os.IsNotExist(err) {
			log.Println("Error: File does not exist.")
			log.Fatal("Error: Could not read pipeline script from file", m.File)
		}
		log.Println("Info: Reading pipeline script from file", m.File)
		config, _ := Jenkins.GetJobConfig("foo")
		newPipeline, err := util.LoadPipelineScriptFromFile(filepath.Clean(m.File))
		if err != nil {
			log.Println("Error:", err)
			log.Fatal("Error: Could not read pipeline script from file", m.File)
		}
		updatedScript, _ := util.ReplacePipelineScript(config, newPipeline)
		Jenkins.UpdateJobConfig("foo", updatedScript)
		log.Println("Info: Updated pipeline script for job foo")
		// Trigger a build
		m.statusMessage = "💤 Waiting for job to start..."
		buildUrl := Jenkins.TriggerBuild("foo")
		// log.Println("Info: Build URL:", buildUrl)
		time.Sleep(1 * time.Second)
		// Stream the output of the build console to the terminal
		log.Println("Info: Build URL inside initBuild:", buildUrl)
		m.BuildUrl = buildUrl
		m.statusMessage = " 👷 Executing build..."

		return consoleOutput("No console output yet...")
	}
}

func (m *BuildModel) GetBuildOutput(filterOutput bool) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(3 * time.Second)
		if m.BuildUrl == "" {
			return emptyUrl("No build URL found. Need to trigger build first.")
		}
		// Setup console output request
		req, err := http.NewRequest("GET", m.BuildUrl+"/logText/progressiveText", nil)
		if err != nil {
			log.Println("Error:", err)
		}
		req.SetBasicAuth(Jenkins.User, Jenkins.APIKey)
		client := &http.Client{}

		// Send the request
		resp, err := client.Do(req)
		if err != nil {
			log.Println("Error:", err)
			log.Println("Error: Could not connect to Jenkins server. Please check the address and try again.")
		}
		defer resp.Body.Close()
		// Read the response
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Println("Error:", err)
			m.done = true
		}
		// Check if the build is still running
		xMoreData := resp.Header.Get("X-More-Data")
		if xMoreData != "true" {
			m.done = true
			return consoleFinish("Build finished!")
			// return consoleOutput(string(body))
		}
		// Get the new log as raw
		rawLog := string(body)
		var cleanedLog string
		// Remove the [Pipeline] part from the console output
		if filterOutput {
			cleanedLog = removePipelinePart(rawLog)
		} else {
			cleanedLog = rawLog
		}

		// Check if the full log is empty
		var newLog string
		if FullLog == "" {
			log.Println("Setting full log to new log")
			FullLog = strings.Clone(cleanedLog)
			newLog = FullLog
		} else {
			// If the full log is not empty, get the new logText
			newLog = strings.ReplaceAll(cleanedLog, FullLog, "")
			FullLog = strings.Clone(cleanedLog)
		}
		_ = newLog
		return consoleOutput(FullLog)

	}
}

// removePipelinePart removes the [Pipeline] part from the console output
func removePipelinePart(consoleOutput string) string {
	regexp := regexp.MustCompile(`(?m)\[Pipeline\].*\n`)
	res := regexp.ReplaceAllString(consoleOutput, "")
	return res
}

func init() {
	rootCmd.AddCommand(updateCmd)
}

func NewBuildModel(filename string, width int, height int) *BuildModel {
	// Setup viewport initial dimensions. Will be set to full screen size in the first update.
	vp := viewport.New(width-3, height-7)
	vp.Style = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		Margin(1, 1, 0)

	vp.SetContent("No console output yet...")

	// Setup spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))
	return &BuildModel{
		File:          filename,
		spinner:       s,
		viewport:      vp,
		userScrolled:  false,
		statusMessage: "⌚ Triggering job ...",
	}
}

func (m *BuildModel) Init() tea.Cmd {
	log.Println("Init in BuildModel")
	return tea.Batch(m.GetBuildOutput(true), m.spinner.Tick)
}

func (m *BuildModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		log.Println("Window size message")
		m.width, m.height = msg.Width, msg.Height
		m.viewport.Height = m.height - 7
		m.viewport.Width = m.width - 3
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			return m, tea.Quit
		case "k", "up", "j", "down", "home", "end":
			m.userScrolled = true
		case "ctrl+u", "pageup":
			m.userScrolled = true
			m.viewport.ViewUp()
		case "ctrl+d", "pagedown":
			m.userScrolled = true
			m.viewport.ViewDown()
		case "a", "G":
			// Append to auto scroll again
			m.userScrolled = false
			m.viewport.GotoBottom()
		}
	case emptyUrl:
		log.Println("Empty URL")
		cmds = append(cmds, m.initBuild())
	case consoleFinish:
		// Build finished
		m.statusMessage = checkMark.Render() + " Build finished!"
		m.done = true
		// return m, tea.Quit
	case consoleOutput:
		m.viewport.SetContent(string(msg))
		// If user scrolled manually, don't auto-scroll
		if !m.userScrolled {
			m.viewport.GotoBottom()
		}
		// m.viewport, cmd = m.viewport.Update(msg)

		cmds = append(cmds, m.GetBuildOutput(true))
	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m BuildModel) View() string {
	help := helpStyle.Render(fmt.Sprintf("\n a/G: auto-scroll • j/↓: down • k/↑: up\n c+u/p-up: page up • c+d/p-down: page down •q: exit\n"))
	if m.done {
		return doneStyle.Render(m.statusMessage+"\n") + m.viewport.View() + help
	}
	spin := m.spinner.View() + " " + m.statusMessage + "\n"
	return doneStyle.Render(spin) + m.viewport.View() + help
}

func main() {
	f, err := tea.LogToFile("debug.log", "console")
	if err != nil {
		fmt.Println("fatal:", err)
		os.Exit(1)
	}
	defer f.Close()
	testFile := "Jenkinsfile"
	width, height := 80, 24
	if _, err := tea.NewProgram(NewBuildModel(testFile, width, height), tea.WithMouseCellMotion(), tea.WithAltScreen()).Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}
